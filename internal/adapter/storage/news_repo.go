package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
)

// NewsRepo implements ports.NewsRepository using BadgerDB.
type NewsRepo struct {
	db *badger.DB
}

// NewNewsRepo creates a new NewsRepo backed by the given DB.
func NewNewsRepo(db *DB) *NewsRepo {
	return &NewsRepo{db: db.Badger()}
}

// --- Key builders ---

// newsKey formatting: news:{date}:{id} -> "news:20260317:some-hash"
func newsKey(date string, eventID string) []byte {
	return []byte(fmt.Sprintf("news:%s:%s", date, eventID))
}

// newsPrefix formatting: news:{date}:
func newsPrefix(date string) []byte {
	return []byte(fmt.Sprintf("news:%s:", date))
}

// SaveEvents stores a batch of NewsEvent records.
func (r *NewsRepo) SaveEvents(_ context.Context, events []domain.NewsEvent) error {
	wb := r.db.NewWriteBatch()
	defer wb.Cancel()

	for i := range events {
		data, err := json.Marshal(&events[i])
		if err != nil {
			return fmt.Errorf("marshal news event %s: %w", events[i].ID, err)
		}
		// Assuming domain.NewsEvent.Date is formatted properly
		// For simplicity, we can use TimeWIB to format a solid "20060102"
		dateKeyStr := events[i].TimeWIB.Format("20060102")
		key := newsKey(dateKeyStr, events[i].ID)

		if err := wb.Set(key, data); err != nil {
			return fmt.Errorf("batch set news %s: %w", events[i].ID, err)
		}
	}

	if err := wb.Flush(); err != nil {
		return fmt.Errorf("flush news batch: %w", err)
	}
	return nil
}

// GetByDate returns all events for a specific date "YYYYMMDD".
func (r *NewsRepo) GetByDate(_ context.Context, date string) ([]domain.NewsEvent, error) {
	var events []domain.NewsEvent
	prefix := newsPrefix(date)

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
				var evt domain.NewsEvent
				if err := json.Unmarshal(val, &evt); err != nil {
					return err
				}
				events = append(events, evt)
				return nil
			})
			if err != nil {
				return fmt.Errorf("read news at %s: %w", item.Key(), err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get news by date: %w", err)
	}
	return events, nil
}

// GetByWeek returns events starting from a specific date spanning 7 days.
func (r *NewsRepo) GetByWeek(ctx context.Context, weekStart string) ([]domain.NewsEvent, error) {
	var allEvents []domain.NewsEvent
	startT, err := time.Parse("20060102", weekStart)
	if err != nil {
		return nil, fmt.Errorf("invalid weekStart format: %w", err)
	}

	for startT.Weekday() != time.Monday {
		startT = startT.AddDate(0, 0, -1)
	}

	// Iterate for 7 days
	for i := 0; i < 7; i++ {
		dateStr := startT.AddDate(0, 0, i).Format("20060102")
		dailyEvents, err := r.GetByDate(ctx, dateStr)
		if err != nil {
			return nil, err
		}
		allEvents = append(allEvents, dailyEvents...)
	}

	return allEvents, nil
}

// GetByMonth returns all events for a given month. yearMonth format: "202603"
func (r *NewsRepo) GetByMonth(ctx context.Context, yearMonth string) ([]domain.NewsEvent, error) {
	// yearMonth is "YYYYMM" — scan all dates with prefix "news:YYYYMM"
	var allEvents []domain.NewsEvent
	prefix := []byte(fmt.Sprintf("news:%s", yearMonth))

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true
		opts.PrefetchSize = 100

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var evt domain.NewsEvent
				if err := json.Unmarshal(val, &evt); err != nil {
					return err
				}
				allEvents = append(allEvents, evt)
				return nil
			})
			if err != nil {
				return fmt.Errorf("read news at %s: %w", item.Key(), err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get news by month: %w", err)
	}
	return allEvents, nil
}

// GetPending returns all events for a specific date where status is "pending_retry".
func (r *NewsRepo) GetPending(ctx context.Context, date string) ([]domain.NewsEvent, error) {
	daily, err := r.GetByDate(ctx, date)
	if err != nil {
		return nil, err
	}

	var pending []domain.NewsEvent
	for _, e := range daily {
		if e.Status == "pending_retry" || e.Status == "upcoming" && e.Actual == "" {
			// Include events that still need fetching actually
			pending = append(pending, e)
		}
	}
	return pending, nil
}

// UpdateActual updates a specific event's Actual field without touching other records.
func (r *NewsRepo) UpdateActual(ctx context.Context, id string, actual string) error {
	// Because keys contain date, we need a prefix scan over all dates to find this ID?
	// Usually ID is unique, but Badger limits us to prefix.
	// To optimize: we should pass date as well, but Interface only gives ID.
	// We'll iterate the whole news keys to find it if we don't know the date.

	// A better way: let's scan all news keys that contain the ID.
	var targetKey []byte
	var evt domain.NewsEvent

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("news:")

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte("news:")); it.ValidForPrefix([]byte("news:")); it.Next() {
			item := it.Item()
			if strings.HasSuffix(string(item.Key()), ":"+id) {
				targetKey = item.KeyCopy(nil)
				return item.Value(func(val []byte) error {
					return json.Unmarshal(val, &evt)
				})
			}
		}
		return badger.ErrKeyNotFound
	})

	if err != nil {
		return fmt.Errorf("find event to update: %w", err)
	}

	// Update field
	evt.Actual = actual
	if actual != "" {
		evt.Status = "released"
	}

	// Write back
	data, _ := json.Marshal(&evt)
	return r.db.Update(func(txn *badger.Txn) error {
		return txn.Set(targetKey, data)
	})
}

// UpdateStatus updates the event status.
func (r *NewsRepo) UpdateStatus(ctx context.Context, id string, status string, retryCount int) error {
	var targetKey []byte
	var evt domain.NewsEvent

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("news:")

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte("news:")); it.ValidForPrefix([]byte("news:")); it.Next() {
			item := it.Item()
			if strings.HasSuffix(string(item.Key()), ":"+id) {
				targetKey = item.KeyCopy(nil)
				return item.Value(func(val []byte) error {
					return json.Unmarshal(val, &evt)
				})
			}
		}
		return badger.ErrKeyNotFound
	})

	if err != nil {
		return fmt.Errorf("find event to update status: %w", err)
	}

	evt.Status = status
	evt.RetryCount = retryCount
	data, _ := json.Marshal(&evt)

	return r.db.Update(func(txn *badger.Txn) error {
		return txn.Set(targetKey, data)
	})
}

// SaveRevision stores an event revision record for historical tracking.
func (r *NewsRepo) SaveRevision(_ context.Context, rev domain.EventRevision) error {
	data, err := json.Marshal(&rev)
	if err != nil {
		return fmt.Errorf("marshal revision: %w", err)
	}

	key := []byte(fmt.Sprintf("evtrev:%s:%s:%s",
		rev.Currency,
		rev.RevisionDate.Format("20060102"),
		rev.EventID,
	))
	return r.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}
