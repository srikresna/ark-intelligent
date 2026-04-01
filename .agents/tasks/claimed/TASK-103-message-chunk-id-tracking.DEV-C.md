# TASK-103: Message Chunk ID Tracking — Claimed by Dev-C

**Claimed:** 2026-04-02 06:02 CST
**Agent:** Dev-C
**Branch:** feat/TASK-103-message-chunk-id-tracking

## Implementation Summary

Added internal chunk tracking to Bot so that multi-part message overflow IDs
are recorded and automatically cleaned up on subsequent edits.

### Changes:
- `api_chunk_tracker.go` — NEW: Thread-safe chunk tracker with TTL-based eviction
- `api_chunk_tracker_test.go` — NEW: 5 unit tests covering record/pop/eviction
- `api.go` — Updated SendHTML, EditMessage, SendWithKeyboardChunked, EditWithKeyboardChunked
  to track and clean up overflow chunk IDs
- `bot.go` — Added `chunks *chunkTracker` field to Bot struct
- `wiring.go` — Initialize chunk tracker in NewBot()

### Design Decisions:
- Backward-compatible: SendHTML still returns (int, error), no interface change
- TTL-based cleanup (30 min) prevents unbounded memory growth
- Best-effort deletion of old overflow (errors swallowed)
- Record() is a no-op for single-chunk messages (zero overhead)
