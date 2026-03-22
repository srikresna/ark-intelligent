package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var memRepoLog = logger.Component("memory-repo")

// MemoryRepo implements ai.MemoryPersister using BadgerDB.
// Keys: "mem:{userID}:{path}" → content string
// TTL: 30 days (memory files expire after inactivity).
type MemoryRepo struct {
	db  *badger.DB
	ttl time.Duration
}

// NewMemoryRepo creates a MemoryRepo with the given TTL.
func NewMemoryRepo(db *DB, ttl time.Duration) *MemoryRepo {
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour // 30 days default
	}
	return &MemoryRepo{
		db:  db.db,
		ttl: ttl,
	}
}

// memEntry wraps content with metadata for storage.
type memEntry struct {
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

func memKey(userID int64, path string) []byte {
	return []byte(fmt.Sprintf("mem:%d:%s", userID, path))
}

func memPrefix(userID int64) []byte {
	return []byte(fmt.Sprintf("mem:%d:", userID))
}

// LoadAll loads all memory files for a user.
func (r *MemoryRepo) LoadAll(_ context.Context, userID int64) (map[string]string, error) {
	files := make(map[string]string)
	prefix := memPrefix(userID)

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := string(item.Key())

			// Extract path from key: "mem:{userID}:{path}" → path
			parts := strings.SplitN(key, ":", 3)
			if len(parts) < 3 {
				continue
			}
			path := parts[2]

			var entry memEntry
			if err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &entry)
			}); err != nil {
				memRepoLog.Warn().Err(err).Str("key", key).Msg("failed to unmarshal memory entry")
				continue
			}

			files[path] = entry.Content
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("load memory files: %w", err)
	}

	return files, nil
}

// Save persists a single memory file.
func (r *MemoryRepo) Save(_ context.Context, userID int64, path string, content string) error {
	entry := memEntry{
		Content:   content,
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal memory entry: %w", err)
	}

	return r.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry(memKey(userID, path), data).WithTTL(r.ttl)
		return txn.SetEntry(e)
	})
}

// Delete removes a memory file.
func (r *MemoryRepo) Delete(_ context.Context, userID int64, path string) error {
	return r.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(memKey(userID, path))
	})
}
