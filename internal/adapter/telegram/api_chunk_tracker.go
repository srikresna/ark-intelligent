package telegram

import (
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Chunk ID Tracker — tracks overflow message IDs for multi-part messages
// ---------------------------------------------------------------------------
//
// When SendHTML or EditMessage splits a long message into multiple Telegram
// messages, only the first (or last) message ID is returned to the caller.
// Subsequent edits of the same logical message leave orphaned chunks in the
// chat because the overflow message IDs are lost.
//
// chunkTracker records the overflow (extra) message IDs associated with a
// primary message so that future edits can delete them before sending new
// overflow chunks.
//
// Memory is bounded via TTL-based expiration: entries older than chunkTTL
// are lazily evicted on each write to prevent unbounded growth.

const chunkTTL = 30 * time.Minute

// chunkEntry holds overflow message IDs and a creation timestamp for TTL.
type chunkEntry struct {
	overflowIDs []int     // message IDs of chunks beyond the primary
	createdAt   time.Time // for TTL-based eviction
}

// chunkTracker is a per-chat map of primary-message-ID → overflow chunk IDs.
// The outer key is chatID, inner key is the primary (first) message ID.
type chunkTracker struct {
	mu      sync.Mutex
	entries map[string]map[int]*chunkEntry
}

// newChunkTracker creates an empty tracker.
func newChunkTracker() *chunkTracker {
	return &chunkTracker{
		entries: make(map[string]map[int]*chunkEntry),
	}
}

// Record stores overflow message IDs for a primary message in a chat.
// Calling Record replaces any previously recorded overflow for that primary ID.
func (ct *chunkTracker) Record(chatID string, primaryID int, overflowIDs []int) {
	if len(overflowIDs) == 0 {
		return
	}
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.evictExpiredLocked()

	if ct.entries[chatID] == nil {
		ct.entries[chatID] = make(map[int]*chunkEntry)
	}
	ct.entries[chatID][primaryID] = &chunkEntry{
		overflowIDs: overflowIDs,
		createdAt:   time.Now(),
	}
}

// Pop retrieves and removes the overflow IDs for a primary message.
// Returns nil if no overflow was tracked (single-chunk message).
func (ct *chunkTracker) Pop(chatID string, primaryID int) []int {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	chatMap, ok := ct.entries[chatID]
	if !ok {
		return nil
	}
	entry, ok := chatMap[primaryID]
	if !ok {
		return nil
	}
	delete(chatMap, primaryID)
	if len(chatMap) == 0 {
		delete(ct.entries, chatID)
	}
	return entry.overflowIDs
}

// evictExpiredLocked removes entries older than chunkTTL.
// Must be called with ct.mu held.
func (ct *chunkTracker) evictExpiredLocked() {
	cutoff := time.Now().Add(-chunkTTL)
	for chatID, chatMap := range ct.entries {
		for msgID, entry := range chatMap {
			if entry.createdAt.Before(cutoff) {
				delete(chatMap, msgID)
			}
		}
		if len(chatMap) == 0 {
			delete(ct.entries, chatID)
		}
	}
}
