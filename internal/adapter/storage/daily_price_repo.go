package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// DailyPriceRepo implements persistence for daily OHLCV price records using BadgerDB.
type DailyPriceRepo struct {
	db *badger.DB
}

// NewDailyPriceRepo creates a new DailyPriceRepo backed by the given DB.
func NewDailyPriceRepo(db *DB) *DailyPriceRepo {
	return &DailyPriceRepo{db: db.Badger()}
}

// --- Key builders ---

func dailyPriceKey(contractCode string, date time.Time) []byte {
	return []byte(fmt.Sprintf("dprice:%s:%s", contractCode, date.Format("20060102")))
}

func dailyPricePrefix(contractCode string) []byte {
	return []byte(fmt.Sprintf("dprice:%s:", contractCode))
}

// --- DailyPriceRepo methods ---

// SaveDailyPrices stores a batch of daily price records.
func (r *DailyPriceRepo) SaveDailyPrices(_ context.Context, records []domain.DailyPrice) error {
	if len(records) == 0 {
		return nil
	}

	wb := r.db.NewWriteBatch()
	defer wb.Cancel()

	for i := range records {
		data, err := json.Marshal(&records[i])
		if err != nil {
			return fmt.Errorf("marshal daily price %s: %w", records[i].ContractCode, err)
		}
		key := dailyPriceKey(records[i].ContractCode, records[i].Date)
		if err := wb.Set(key, data); err != nil {
			return fmt.Errorf("batch set daily price: %w", err)
		}
	}

	if err := wb.Flush(); err != nil {
		return fmt.Errorf("flush daily prices batch: %w", err)
	}
	return nil
}

// GetLatestDaily returns the most recent daily price record for a contract.
func (r *DailyPriceRepo) GetLatestDaily(_ context.Context, contractCode string) (*domain.DailyPrice, error) {
	var record *domain.DailyPrice

	prefix := dailyPricePrefix(contractCode)

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.Reverse = true
		opts.PrefetchValues = true
		opts.PrefetchSize = 1

		it := txn.NewIterator(opts)
		defer it.Close()

		seekKey := make([]byte, len(prefix)+1)
		copy(seekKey, prefix)
		seekKey[len(prefix)] = 0xFF
		it.Seek(seekKey)

		if it.ValidForPrefix(prefix) {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				record = &domain.DailyPrice{}
				return json.Unmarshal(val, record)
			})
			if err != nil {
				return fmt.Errorf("read latest daily price: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get latest daily price %s: %w", contractCode, err)
	}
	return record, nil
}

// GetDailyHistory returns daily price records for a contract over N days.
// Returns records in reverse chronological order (newest first).
func (r *DailyPriceRepo) GetDailyHistory(_ context.Context, contractCode string, days int) ([]domain.DailyPrice, error) {
	var records []domain.DailyPrice

	cutoff := time.Now().AddDate(0, 0, -days).Format("20060102")
	prefix := dailyPricePrefix(contractCode)
	seekKey := []byte(fmt.Sprintf("dprice:%s:%s", contractCode, cutoff))

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true
		opts.PrefetchSize = 100

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(seekKey); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var rec domain.DailyPrice
				if err := json.Unmarshal(val, &rec); err != nil {
					return err
				}
				records = append(records, rec)
				return nil
			})
			if err != nil {
				return fmt.Errorf("read daily price history: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get daily price history %s: %w", contractCode, err)
	}

	// Reverse to newest-first order.
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetDailyPriceAt retrieves the daily price record closest to the given date,
// searching within ±3 days (to handle weekends/holidays).
func (r *DailyPriceRepo) GetDailyPriceAt(_ context.Context, contractCode string, date time.Time) (*domain.DailyPrice, error) {
	var record *domain.DailyPrice

	prefix := dailyPricePrefix(contractCode)
	targetKey := []byte(fmt.Sprintf("dprice:%s:%s", contractCode, date.Format("20060102")))
	maxDistance := 3 * 24 * time.Hour

	err := r.db.View(func(txn *badger.Txn) error {
		// Forward scan
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
				record = &domain.DailyPrice{}
				return json.Unmarshal(val, record)
			})
			if err != nil {
				return fmt.Errorf("read daily price at date: %w", err)
			}
			if record.Date.Sub(date) <= maxDistance {
				return nil
			}
			record = nil
		}

		// Reverse scan
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
			var prior domain.DailyPrice
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &prior)
			})
			if err != nil {
				return fmt.Errorf("read daily price before date: %w", err)
			}
			if date.Sub(prior.Date) <= maxDistance {
				if record == nil || date.Sub(prior.Date) < record.Date.Sub(date) {
					record = &prior
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get daily price at %s %s: %w", contractCode, date.Format("20060102"), err)
	}
	return record, nil
}

// GetDailyHistoryBefore returns daily price records for a contract before a specific date.
// Returns up to `days` records in reverse chronological order (newest first),
// ending at or before `before`. Used by the bootstrap to get historical daily context.
func (r *DailyPriceRepo) GetDailyHistoryBefore(_ context.Context, contractCode string, before time.Time, days int) ([]domain.DailyPrice, error) {
	var records []domain.DailyPrice

	prefix := dailyPricePrefix(contractCode)
	// Seek to the target date and scan backward
	seekKey := []byte(fmt.Sprintf("dprice:%s:%s\xff", contractCode, before.Format("20060102")))

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.Reverse = true
		opts.PrefetchValues = true
		opts.PrefetchSize = 100

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(seekKey); it.ValidForPrefix(prefix) && len(records) < days; it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var rec domain.DailyPrice
				if err := json.Unmarshal(val, &rec); err != nil {
					return err
				}
				records = append(records, rec)
				return nil
			})
			if err != nil {
				return fmt.Errorf("read daily price history before: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get daily price history before %s %s: %w", contractCode, before.Format("20060102"), err)
	}

	// Already in newest-first order (reverse scan)
	return records, nil
}

// CountDailyRecords returns the total number of daily price records for a contract.
func (r *DailyPriceRepo) CountDailyRecords(_ context.Context, contractCode string) (int, error) {
	count := 0
	prefix := dailyPricePrefix(contractCode)

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = false // keys only

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			count++
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("count daily records %s: %w", contractCode, err)
	}
	return count, nil
}
