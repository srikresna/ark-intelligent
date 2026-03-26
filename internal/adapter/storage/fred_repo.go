package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// FREDRepo implements persistence for FRED data snapshots using BadgerDB.
type FREDRepo struct {
	db *badger.DB
}

// NewFREDRepo creates a new FREDRepo backed by the given DB.
func NewFREDRepo(db *DB) *FREDRepo {
	return &FREDRepo{db: db.Badger()}
}

// fredKey: fred:{series_id}:{YYYYMMDD}
func fredKey(seriesID string, date time.Time) []byte {
	return []byte(fmt.Sprintf("fred:%s:%s", seriesID, date.Format("20060102")))
}

// fredPrefix: fred:{series_id}:
func fredPrefix(seriesID string) []byte {
	return []byte(fmt.Sprintf("fred:%s:", seriesID))
}

type fredEntry struct {
	Value float64   `json:"v"`
	Date  time.Time `json:"d"`
}

// SaveSnapshot persists a single FRED observation.
func (r *FREDRepo) SaveSnapshot(_ context.Context, seriesID string, date time.Time, value float64) error {
	data, err := json.Marshal(&fredEntry{Value: value, Date: date})
	if err != nil {
		return fmt.Errorf("marshal fred entry: %w", err)
	}

	key := fredKey(seriesID, date)
	return r.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}

// SaveSnapshots persists multiple observations in a batch.
func (r *FREDRepo) SaveSnapshots(_ context.Context, observations []ports.FREDObservation) error {
	wb := r.db.NewWriteBatch()
	defer wb.Cancel()

	for _, obs := range observations {
		data, err := json.Marshal(&fredEntry{Value: obs.Value, Date: obs.Date})
		if err != nil {
			continue
		}
		key := fredKey(obs.SeriesID, obs.Date)
		if err := wb.Set(key, data); err != nil {
			return fmt.Errorf("batch set fred: %w", err)
		}
	}

	return wb.Flush()
}

// GetHistory returns the last N days of observations for a series, newest first.
func (r *FREDRepo) GetHistory(_ context.Context, seriesID string, days int) ([]ports.FREDObservation, error) {
	var results []ports.FREDObservation
	prefix := fredPrefix(seriesID)

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.Reverse = true
		opts.PrefetchValues = true
		opts.PrefetchSize = days

		it := txn.NewIterator(opts)
		defer it.Close()

		// Seek to the end of this prefix range
		seekKey := append(prefix, 0xFF)
		count := 0
		for it.Seek(seekKey); it.ValidForPrefix(prefix) && count < days; it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var entry fredEntry
				if err := json.Unmarshal(val, &entry); err != nil {
					return err
				}
				results = append(results, ports.FREDObservation{
					SeriesID: seriesID,
					Date:     entry.Date,
					Value:    entry.Value,
				})
				return nil
			})
			if err != nil {
				return fmt.Errorf("read fred entry: %w", err)
			}
			count++
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get fred history %s: %w", seriesID, err)
	}
	return results, nil
}

// GetLatest returns the most recent observation for a series.
func (r *FREDRepo) GetLatest(ctx context.Context, seriesID string) (*ports.FREDObservation, error) {
	history, err := r.GetHistory(ctx, seriesID, 1)
	if err != nil {
		return nil, err
	}
	if len(history) == 0 {
		return nil, nil
	}
	return &history[0], nil
}
