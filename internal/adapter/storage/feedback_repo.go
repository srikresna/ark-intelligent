package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// FeedbackRepo persists analysis feedback in BadgerDB.
// Key format: "fb:<userID>:<analysisType>:<analysisKey>:<timestamp>"
type FeedbackRepo struct {
	db *badger.DB
}

// NewFeedbackRepo creates a new FeedbackRepo backed by the given DB.
func NewFeedbackRepo(db *DB) *FeedbackRepo {
	return &FeedbackRepo{db: db.Badger()}
}

func feedbackKey(userID int64, analysisType, analysisKey string, ts time.Time) []byte {
	return []byte(fmt.Sprintf("fb:%d:%s:%s:%d", userID, analysisType, analysisKey, ts.UnixMilli()))
}

// Save persists a feedback entry.
func (r *FeedbackRepo) Save(_ context.Context, fb *domain.Feedback) error {
	data, err := json.Marshal(fb)
	if err != nil {
		return fmt.Errorf("marshal feedback: %w", err)
	}

	key := feedbackKey(fb.UserID, fb.AnalysisType, fb.AnalysisKey, fb.CreatedAt)

	return r.db.Update(func(txn *badger.Txn) error {
		// TTL of 90 days — old feedback is auto-pruned
		e := badger.NewEntry(key, data).WithTTL(90 * 24 * time.Hour)
		return txn.SetEntry(e)
	})
}

// CountByType returns (upCount, downCount) for a given analysis type+key across all users.
func (r *FeedbackRepo) CountByType(_ context.Context, analysisType, analysisKey string) (up, down int, err error) {
	prefix := []byte("fb:")

	err = r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			if valErr := item.Value(func(val []byte) error {
				var fb domain.Feedback
				if jsonErr := json.Unmarshal(val, &fb); jsonErr != nil {
					return nil // skip corrupt entries
				}
				if fb.AnalysisType == analysisType && fb.AnalysisKey == analysisKey {
					switch fb.Rating {
					case "up":
						up++
					case "down":
						down++
					}
				}
				return nil
			}); valErr != nil {
				return valErr
			}
		}
		return nil
	})

	return up, down, err
}
