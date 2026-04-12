package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// IntradayRepo implements persistence for intraday (4H/1H) OHLCV price records.
type IntradayRepo struct {
	db *badger.DB
}

// NewIntradayRepo creates a new IntradayRepo backed by the given DB.
func NewIntradayRepo(db *DB) *IntradayRepo {
	return &IntradayRepo{db: db.Badger()}
}

// --- Key builders ---

// Key format: iprice:<contract_code>:<interval>:<YYYYMMDDHHmm>
func intradayKey(contractCode, interval string, ts time.Time) []byte {
	return []byte(fmt.Sprintf("iprice:%s:%s:%s", contractCode, interval, ts.UTC().Format("200601021504")))
}

func intradayPrefix(contractCode, interval string) []byte {
	return []byte(fmt.Sprintf("iprice:%s:%s:", contractCode, interval))
}

// --- Methods ---

// SaveBars stores a batch of intraday bars.
func (r *IntradayRepo) SaveBars(ctx context.Context, bars []domain.IntradayBar) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(bars) == 0 {
		return nil
	}

	wb := r.db.NewWriteBatch()
	defer wb.Cancel()

	for i := range bars {
		data, err := json.Marshal(&bars[i])
		if err != nil {
			return fmt.Errorf("marshal intraday bar %s: %w", bars[i].ContractCode, err)
		}
		key := intradayKey(bars[i].ContractCode, bars[i].Interval, bars[i].Timestamp)
		if err := wb.Set(key, data); err != nil {
			return fmt.Errorf("batch set intraday bar: %w", err)
		}
	}

	if err := wb.Flush(); err != nil {
		return fmt.Errorf("flush intraday batch: %w", err)
	}
	return nil
}

// GetHistory returns intraday bars for a contract, newest first.
// barCount is the number of bars to retrieve.
func (r *IntradayRepo) GetHistory(ctx context.Context, contractCode, interval string, barCount int) ([]domain.IntradayBar, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var bars []domain.IntradayBar

	prefix := intradayPrefix(contractCode, interval)

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.Reverse = true
		opts.PrefetchValues = true
		opts.PrefetchSize = min(barCount, 200)

		it := txn.NewIterator(opts)
		defer it.Close()

		seekKey := make([]byte, len(prefix)+1)
		copy(seekKey, prefix)
		seekKey[len(prefix)] = 0xFF
		it.Seek(seekKey)

		for ; it.ValidForPrefix(prefix) && len(bars) < barCount; it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var bar domain.IntradayBar
				if err := json.Unmarshal(val, &bar); err != nil {
					return err
				}
				bars = append(bars, bar)
				return nil
			})
			if err != nil {
				return fmt.Errorf("read intraday bar: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get intraday history %s %s: %w", contractCode, interval, err)
	}
	return bars, nil // Already newest-first from reverse iteration
}

// GetLatest returns the most recent intraday bar for a contract.
func (r *IntradayRepo) GetLatest(ctx context.Context, contractCode, interval string) (*domain.IntradayBar, error) {
	bars, err := r.GetHistory(ctx, contractCode, interval, 1)
	if err != nil {
		return nil, err
	}
	if len(bars) == 0 {
		return nil, nil
	}
	return &bars[0], nil
}

// PurgeOlderThan removes intraday bars older than the given cutoff.
func (r *IntradayRepo) PurgeOlderThan(ctx context.Context, interval string, cutoff time.Time) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	cutoffStr := cutoff.UTC().Format("200601021504")

	// Phase 1: collect keys to delete (read-only).
	var keysToDelete [][]byte
	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		prefix := []byte("iprice:")

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			keyStr := string(key)
			// Key format: iprice:CODE:INTERVAL:YYYYMMDDHHmm
			if len(keyStr) >= 12 {
				tsPart := keyStr[len(keyStr)-12:]
				if tsPart < cutoffStr {
					keysToDelete = append(keysToDelete, key)
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("purge scan: %w", err)
	}

	if len(keysToDelete) == 0 {
		return 0, nil
	}

	// Phase 2: delete collected keys (write batch, outside View).
	wb := r.db.NewWriteBatch()
	defer wb.Cancel()
	for _, k := range keysToDelete {
		if err := wb.Delete(k); err != nil {
			return 0, fmt.Errorf("purge delete: %w", err)
		}
	}
	if err := wb.Flush(); err != nil {
		return 0, fmt.Errorf("purge flush: %w", err)
	}

	return len(keysToDelete), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
