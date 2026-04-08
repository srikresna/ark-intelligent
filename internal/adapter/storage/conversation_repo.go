package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var convLog = logger.Component("conversation")

// conversationEntry wraps a ChatMessage with metadata for storage.
type conversationEntry struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// conversationMeta tracks per-user sequence numbers.
type conversationMeta struct {
	SeqNo        int       `json:"seq_no"`
	LastActivity time.Time `json:"last_activity"`
}

// ConversationRepo implements ports.ConversationRepository using BadgerDB.
type ConversationRepo struct {
	db      *badger.DB
	maxMsgs int
	ttl     time.Duration
}

// NewConversationRepo creates a new ConversationRepo.
// maxMsgs: maximum messages per user (oldest pruned on overflow).
// ttl: how long messages survive (BadgerDB native TTL).
func NewConversationRepo(db *DB, maxMsgs int, ttl time.Duration) *ConversationRepo {
	return &ConversationRepo{
		db:      db.Badger(),
		maxMsgs: maxMsgs,
		ttl:     ttl,
	}
}

// convKey returns a storage key for a conversation message.
func convKey(userID int64, seqNo int) []byte {
	return []byte(fmt.Sprintf("conv:%d:%08d", userID, seqNo))
}

// convPrefix returns the key prefix for all messages of a user.
func convPrefix(userID int64) []byte {
	return []byte(fmt.Sprintf("conv:%d:", userID))
}

// metaKey returns the storage key for a user's conversation metadata.
func metaKey(userID int64) []byte {
	return []byte(fmt.Sprintf("convmeta:%d", userID))
}

// getMeta retrieves the conversation metadata for a user.
// Returns a zero-value meta (not error) if not found.
func (r *ConversationRepo) getMeta(txn *badger.Txn, userID int64) (*conversationMeta, error) {
	item, err := txn.Get(metaKey(userID))
	if err == badger.ErrKeyNotFound {
		return &conversationMeta{SeqNo: 0}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get conv meta: %w", err)
	}

	var meta conversationMeta
	err = item.Value(func(val []byte) error {
		return json.Unmarshal(val, &meta)
	})
	if err != nil {
		return nil, fmt.Errorf("unmarshal conv meta: %w", err)
	}
	return &meta, nil
}

// setMeta persists the conversation metadata for a user.
func (r *ConversationRepo) setMeta(txn *badger.Txn, userID int64, meta *conversationMeta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal conv meta: %w", err)
	}
	e := badger.NewEntry(metaKey(userID), data).WithTTL(r.ttl)
	return txn.SetEntry(e)
}

// GetHistory returns the most recent N messages for a user.
func (r *ConversationRepo) GetHistory(_ context.Context, userID int64, limit int) ([]ports.ChatMessage, error) {
	var entries []conversationEntry

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = convPrefix(userID)
		opts.PrefetchValues = true
		opts.PrefetchSize = limit

		it := txn.NewIterator(opts)
		defer it.Close()

		// Collect all entries (they're in order by seqNo)
		for it.Seek(convPrefix(userID)); it.ValidForPrefix(convPrefix(userID)); it.Next() {
			var entry conversationEntry
			err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &entry)
			})
			if err != nil {
				convLog.Warn().Err(err).Str("key", string(it.Item().Key())).Msg("skipping corrupt conversation entry")
				continue
			}
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get conversation history: %w", err)
	}

	// Return only the last `limit` entries
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	messages := make([]ports.ChatMessage, len(entries))
	for i, e := range entries {
		messages[i] = ports.ChatMessage{
			Role:    e.Role,
			Content: e.Content,
		}
	}
	return messages, nil
}

// AppendMessage adds a message and prunes oldest if over the cap.
func (r *ConversationRepo) AppendMessage(_ context.Context, userID int64, msg ports.ChatMessage) error {
	return r.db.Update(func(txn *badger.Txn) error {
		meta, err := r.getMeta(txn, userID)
		if err != nil {
			return err
		}

		// Increment sequence
		meta.SeqNo++
		meta.LastActivity = time.Now()

		// Store the message
		entry := conversationEntry{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: time.Now(),
		}
		data, err := json.Marshal(&entry)
		if err != nil {
			return fmt.Errorf("marshal conv entry: %w", err)
		}

		e := badger.NewEntry(convKey(userID, meta.SeqNo), data).WithTTL(r.ttl)
		if err := txn.SetEntry(e); err != nil {
			return fmt.Errorf("set conv entry: %w", err)
		}

		// Update metadata
		if err := r.setMeta(txn, userID, meta); err != nil {
			return err
		}

		// Prune if over cap: delete the oldest message
		if meta.SeqNo > r.maxMsgs {
			oldSeq := meta.SeqNo - r.maxMsgs
			oldKey := convKey(userID, oldSeq)
			_ = txn.Delete(oldKey) // best-effort — TTL will clean up eventually
		}

		return nil
	})
}

// ClearHistory deletes all conversation history for a user.
func (r *ConversationRepo) ClearHistory(_ context.Context, userID int64) error {
	deleteKeys := make([][]byte, 0)

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = convPrefix(userID)
		opts.PrefetchValues = false

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(convPrefix(userID)); it.ValidForPrefix(convPrefix(userID)); it.Next() {
			deleteKeys = append(deleteKeys, it.Item().KeyCopy(nil))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan conversation keys: %w", err)
	}

	// Also delete metadata key
	deleteKeys = append(deleteKeys, metaKey(userID))

	if len(deleteKeys) == 0 {
		return nil
	}

	wb := r.db.NewWriteBatch()
	defer wb.Cancel()
	for _, k := range deleteKeys {
		if err := wb.Delete(k); err != nil {
			return fmt.Errorf("delete conv key: %w", err)
		}
	}
	if err := wb.Flush(); err != nil {
		return fmt.Errorf("flush conv delete: %w", err)
	}

	convLog.Info().Int64("user_id", userID).Int("deleted", len(deleteKeys)).Msg("conversation history cleared")
	return nil
}
