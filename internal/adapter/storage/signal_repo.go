package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// SignalRepo implements ports.SignalRepository using BadgerDB.
type SignalRepo struct {
	db *badger.DB
}

// NewSignalRepo creates a new SignalRepo backed by the given DB.
func NewSignalRepo(db *DB) *SignalRepo {
	return &SignalRepo{db: db.Badger()}
}

// --- Key builders ---
// Key format: sig:{contractCode}:{YYYYMMDD}:{signalType}
// This ensures at most one signal per type per contract per week (deduplication).

func signalKey(s domain.PersistedSignal) []byte {
	return []byte(fmt.Sprintf("sig:%s:%s:%s",
		s.ContractCode,
		s.ReportDate.Format("20060102"),
		s.SignalType,
	))
}

func signalContractPrefix(contractCode string) []byte {
	return []byte(fmt.Sprintf("sig:%s:", contractCode))
}

func signalAllPrefix() []byte {
	return []byte("sig:")
}

// --- SignalRepository interface implementation ---

// SaveSignals persists a batch of signal snapshots.
func (r *SignalRepo) SaveSignals(ctx context.Context, signals []domain.PersistedSignal) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(signals) == 0 {
		return nil
	}

	wb := r.db.NewWriteBatch()
	defer wb.Cancel()

	for i := range signals {
		data, err := json.Marshal(&signals[i])
		if err != nil {
			return fmt.Errorf("marshal signal %s/%s: %w", signals[i].ContractCode, signals[i].SignalType, err)
		}
		key := signalKey(signals[i])
		if err := wb.Set(key, data); err != nil {
			return fmt.Errorf("batch set signal: %w", err)
		}
	}

	if err := wb.Flush(); err != nil {
		return fmt.Errorf("flush signals batch: %w", err)
	}
	return nil
}

// GetSignalsByContract retrieves all persisted signals for a contract.
// Ordered newest-first by report date.
func (r *SignalRepo) GetSignalsByContract(ctx context.Context, contractCode string) ([]domain.PersistedSignal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prefix := signalContractPrefix(contractCode)
	signals, err := r.scanSignals(prefix)
	if err != nil {
		return nil, fmt.Errorf("get signals for %s: %w", contractCode, err)
	}
	reverseSignals(signals)
	return signals, nil
}

// GetSignalsByType retrieves all persisted signals of a given type across all contracts.
func (r *SignalRepo) GetSignalsByType(ctx context.Context, signalType string) ([]domain.PersistedSignal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	all, err := r.scanSignals(signalAllPrefix())
	if err != nil {
		return nil, fmt.Errorf("get signals by type %s: %w", signalType, err)
	}

	var filtered []domain.PersistedSignal
	for i := range all {
		if all[i].SignalType == signalType {
			filtered = append(filtered, all[i])
		}
	}
	reverseSignals(filtered)
	return filtered, nil
}

// GetAllSignals retrieves all persisted signals.
func (r *SignalRepo) GetAllSignals(ctx context.Context) ([]domain.PersistedSignal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	signals, err := r.scanSignals(signalAllPrefix())
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}
	reverseSignals(signals)
	return signals, nil
}

// GetPendingSignals retrieves signals that need outcome evaluation.
// A signal is pending if any horizon (1W/2W/4W) still needs evaluation
// and enough time has passed since the report date for that horizon.
func (r *SignalRepo) GetPendingSignals(ctx context.Context) ([]domain.PersistedSignal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	all, err := r.scanSignals(signalAllPrefix())
	if err != nil {
		return nil, fmt.Errorf("get pending signals: %w", err)
	}

	now := time.Now()
	var pending []domain.PersistedSignal
	for i := range all {
		if all[i].NeedsEvaluation(now) {
			pending = append(pending, all[i])
		}
	}
	log.Info().
		Int("total_signals", len(all)).
		Int("pending", len(pending)).
		Msg("GetPendingSignals filter result")
	return pending, nil
}

// UpdateSignal overwrites a single persisted signal.
func (r *SignalRepo) UpdateSignal(ctx context.Context, signal domain.PersistedSignal) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(&signal)
	if err != nil {
		return fmt.Errorf("marshal signal update: %w", err)
	}

	key := signalKey(signal)
	return r.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}

// --- Internal helpers ---

// scanSignals iterates all keys with the given prefix and deserializes signals.
func (r *SignalRepo) scanSignals(prefix []byte) ([]domain.PersistedSignal, error) {
	var signals []domain.PersistedSignal

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true
		opts.PrefetchSize = 50

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var sig domain.PersistedSignal
				if err := json.Unmarshal(val, &sig); err != nil {
					return err
				}
				signals = append(signals, sig)
				return nil
			})
			if err != nil {
				// Log and skip corrupted entries rather than failing the whole scan
				log.Warn().Err(err).Bytes("key", item.KeyCopy(nil)).Msg("skipping corrupted signal entry")
				continue
			}
		}
		return nil
	})
	return signals, err
}

// reverseSignals reverses the slice in-place (newest-first).
func reverseSignals(s []domain.PersistedSignal) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// PurgeInvalidSignals deletes signals that have EntryPrice == 0.
// These are leftovers from an older bootstrap that didn't check entry prices.
// Returns the number of signals deleted.
func (r *SignalRepo) PurgeInvalidSignals(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	all, err := r.scanSignals(signalAllPrefix())
	if err != nil {
		return 0, fmt.Errorf("scan signals for purge: %w", err)
	}

	var deleteKeys [][]byte
	for i := range all {
		if all[i].EntryPrice == 0 {
			deleteKeys = append(deleteKeys, signalKey(all[i]))
		}
	}

	if len(deleteKeys) == 0 {
		return 0, nil
	}

	wb := r.db.NewWriteBatch()
	defer wb.Cancel()
	for _, key := range deleteKeys {
		if err := wb.Delete(key); err != nil {
			return 0, fmt.Errorf("batch delete signal: %w", err)
		}
	}
	if err := wb.Flush(); err != nil {
		return 0, fmt.Errorf("flush signal purge: %w", err)
	}

	log.Info().Int("purged", len(deleteKeys)).Msg("purged signals with zero entry price")
	return len(deleteKeys), nil
}

// GetRecentSignals retrieves all signals detected within the last N days, newest-first.
func (r *SignalRepo) GetRecentSignals(ctx context.Context, days int) ([]domain.PersistedSignal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	all, err := r.scanSignals(signalAllPrefix())
	if err != nil {
		return nil, fmt.Errorf("get recent signals: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var recent []domain.PersistedSignal
	for i := range all {
		if all[i].DetectedAt.After(cutoff) || all[i].DetectedAt.Equal(cutoff) {
			recent = append(recent, all[i])
		}
	}
	reverseSignals(recent)
	return recent, nil
}

// SignalExists checks if a signal with the given key already exists.
// Used by the bootstrapper to avoid duplicate inserts.
func (r *SignalRepo) SignalExists(ctx context.Context, contractCode string, reportDate time.Time, signalType string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	key := signalKey(domain.PersistedSignal{
		ContractCode: contractCode,
		ReportDate:   reportDate,
		SignalType:   signalType,
	})

	var exists bool
	err := r.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	})
	return exists, err
}
