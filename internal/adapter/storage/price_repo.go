package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// PriceRepo implements ports.PriceRepository using BadgerDB.
type PriceRepo struct {
	db *badger.DB
}

// NewPriceRepo creates a new PriceRepo backed by the given DB.
func NewPriceRepo(db *DB) *PriceRepo {
	return &PriceRepo{db: db.Badger()}
}

// --- Key builders ---

func priceKey(contractCode string, date time.Time) []byte {
	return []byte(fmt.Sprintf("price:%s:%s", contractCode, date.Format("20060102")))
}

func pricePrefix(contractCode string) []byte {
	return []byte(fmt.Sprintf("price:%s:", contractCode))
}

// --- PriceRepository interface implementation ---

// SavePrices stores a batch of weekly price records.
func (r *PriceRepo) SavePrices(ctx context.Context, records []domain.PriceRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	wb := r.db.NewWriteBatch()
	defer wb.Cancel()

	for i := range records {
		data, err := json.Marshal(&records[i])
		if err != nil {
			return fmt.Errorf("marshal price record %s: %w", records[i].ContractCode, err)
		}
		key := priceKey(records[i].ContractCode, records[i].Date)
		if err := wb.Set(key, data); err != nil {
			return fmt.Errorf("batch set price record: %w", err)
		}
	}

	if err := wb.Flush(); err != nil {
		return fmt.Errorf("flush price records batch: %w", err)
	}
	return nil
}

// GetLatest returns the most recent price record for a contract.
func (r *PriceRepo) GetLatest(ctx context.Context, contractCode string) (*domain.PriceRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var record *domain.PriceRecord

	prefix := pricePrefix(contractCode)

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.Reverse = true
		opts.PrefetchValues = true
		opts.PrefetchSize = 1

		it := txn.NewIterator(opts)
		defer it.Close()

		// Build seek key beyond all valid dates (0xFF suffix) without mutating prefix.
		seekKey := make([]byte, len(prefix)+1)
		copy(seekKey, prefix)
		seekKey[len(prefix)] = 0xFF
		it.Seek(seekKey)

		if it.ValidForPrefix(prefix) {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				record = &domain.PriceRecord{}
				return json.Unmarshal(val, record)
			})
			if err != nil {
				return fmt.Errorf("read latest price record: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get latest price %s: %w", contractCode, err)
	}
	return record, nil
}

// GetHistory returns price records for a contract over N weeks.
// Returns records in reverse chronological order (newest first).
func (r *PriceRepo) GetHistory(ctx context.Context, contractCode string, weeks int) ([]domain.PriceRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var records []domain.PriceRecord

	cutoff := time.Now().AddDate(0, 0, -weeks*7).Format("20060102")
	prefix := pricePrefix(contractCode)
	seekKey := []byte(fmt.Sprintf("price:%s:%s", contractCode, cutoff))

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true
		opts.PrefetchSize = 60

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(seekKey); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var rec domain.PriceRecord
				if err := json.Unmarshal(val, &rec); err != nil {
					return err
				}
				records = append(records, rec)
				return nil
			})
			if err != nil {
				return fmt.Errorf("read price history: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get price history %s: %w", contractCode, err)
	}

	// Reverse to newest-first order.
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetPriceAt retrieves the price record closest to the given date,
// searching both forward (up to 7 days) and backward (up to 7 days).
// Returns nil if no record is found within 7 days in either direction.
func (r *PriceRepo) GetPriceAt(ctx context.Context, contractCode string, date time.Time) (*domain.PriceRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var record *domain.PriceRecord

	prefix := pricePrefix(contractCode)
	targetKey := []byte(fmt.Sprintf("price:%s:%s", contractCode, date.Format("20060102")))
	maxDistance := 7 * 24 * time.Hour

	err := r.db.View(func(txn *badger.Txn) error {
		// First try: exact match or forward scan to find nearest entry
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true
		opts.PrefetchSize = 1

		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(targetKey)
		if it.ValidForPrefix(prefix) {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				record = &domain.PriceRecord{}
				return json.Unmarshal(val, record)
			})
			if err != nil {
				return fmt.Errorf("read price at date: %w", err)
			}
			// If exact match or within maxDistance forward, accept it
			if record.Date.Sub(date) <= maxDistance {
				return nil
			}
			// Too far forward — discard and try backward
			record = nil
		}

		// Second try: reverse scan from the target date to find the closest prior record
		opts2 := badger.DefaultIteratorOptions
		opts2.Prefix = prefix
		opts2.Reverse = true
		opts2.PrefetchValues = true
		opts2.PrefetchSize = 1

		it2 := txn.NewIterator(opts2)
		defer it2.Close()

		it2.Seek(targetKey)
		if it2.ValidForPrefix(prefix) {
			item := it2.Item()
			var prior domain.PriceRecord
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &prior)
			})
			if err != nil {
				return fmt.Errorf("read price before date: %w", err)
			}
			// Only accept if within maxDistance backward
			if date.Sub(prior.Date) <= maxDistance {
				if record == nil || date.Sub(prior.Date) < record.Date.Sub(date) {
					record = &prior
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get price at %s %s: %w", contractCode, date.Format("20060102"), err)
	}
	return record, nil
}
