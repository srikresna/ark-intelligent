package telegram

import (
	"testing"
	"time"
)

func TestChunkTracker_RecordAndPop(t *testing.T) {
	ct := newChunkTracker()

	// Pop on empty tracker returns nil.
	if ids := ct.Pop("chat1", 100); ids != nil {
		t.Fatalf("expected nil, got %v", ids)
	}

	// Record overflow for message 100 in chat1.
	ct.Record("chat1", 100, []int{101, 102, 103})

	// Pop returns the overflow and removes the entry.
	ids := ct.Pop("chat1", 100)
	if len(ids) != 3 || ids[0] != 101 || ids[1] != 102 || ids[2] != 103 {
		t.Fatalf("expected [101 102 103], got %v", ids)
	}

	// Second pop returns nil (entry was removed).
	if ids := ct.Pop("chat1", 100); ids != nil {
		t.Fatalf("expected nil after pop, got %v", ids)
	}
}

func TestChunkTracker_RecordReplacesOld(t *testing.T) {
	ct := newChunkTracker()

	ct.Record("chat1", 100, []int{101})
	ct.Record("chat1", 100, []int{201, 202})

	ids := ct.Pop("chat1", 100)
	if len(ids) != 2 || ids[0] != 201 || ids[1] != 202 {
		t.Fatalf("expected replaced IDs [201 202], got %v", ids)
	}
}

func TestChunkTracker_SkipsEmptyOverflow(t *testing.T) {
	ct := newChunkTracker()

	// Recording empty overflow should be a no-op.
	ct.Record("chat1", 100, nil)
	ct.Record("chat1", 100, []int{})

	if ids := ct.Pop("chat1", 100); ids != nil {
		t.Fatalf("expected nil for empty overflow, got %v", ids)
	}
}

func TestChunkTracker_MultipleChatsAndMessages(t *testing.T) {
	ct := newChunkTracker()

	ct.Record("chat1", 100, []int{101})
	ct.Record("chat1", 200, []int{201})
	ct.Record("chat2", 100, []int{301})

	ids1_100 := ct.Pop("chat1", 100)
	ids1_200 := ct.Pop("chat1", 200)
	ids2_100 := ct.Pop("chat2", 100)

	if len(ids1_100) != 1 || ids1_100[0] != 101 {
		t.Fatalf("chat1/100: expected [101], got %v", ids1_100)
	}
	if len(ids1_200) != 1 || ids1_200[0] != 201 {
		t.Fatalf("chat1/200: expected [201], got %v", ids1_200)
	}
	if len(ids2_100) != 1 || ids2_100[0] != 301 {
		t.Fatalf("chat2/100: expected [301], got %v", ids2_100)
	}
}

func TestChunkTracker_TTLEviction(t *testing.T) {
	ct := newChunkTracker()

	// Manually insert an expired entry.
	ct.mu.Lock()
	ct.entries["chat1"] = map[int]*chunkEntry{
		100: {
			overflowIDs: []int{101},
			createdAt:   time.Now().Add(-chunkTTL - time.Minute),
		},
	}
	ct.mu.Unlock()

	// Record a new entry — this triggers eviction of expired entries.
	ct.Record("chat1", 200, []int{201})

	// The expired entry should have been evicted.
	if ids := ct.Pop("chat1", 100); ids != nil {
		t.Fatalf("expected expired entry to be evicted, got %v", ids)
	}

	// The new entry should still be there.
	ids := ct.Pop("chat1", 200)
	if len(ids) != 1 || ids[0] != 201 {
		t.Fatalf("expected [201], got %v", ids)
	}
}
