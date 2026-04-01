# TASK-172: COT Broadcast Dedup Guard — Prevent Duplicate Alerts

**Priority:** high
**Type:** fix
**Estimated:** M
**Area:** internal/scheduler/

## Deskripsi

Add dedup mechanism untuk COT broadcast. Current code checks oldLatest vs newLatest tanpa locking, enabling duplicate broadcasts dari concurrent triggers.

## Bug Detail

```go
// scheduler.go:305-318
oldLatest, _ := s.deps.COTRepo.GetLatestReportDate(ctx)
// ... fetch and analyze (slow!) ...
newLatest, _ := s.deps.COTRepo.GetLatestReportDate(ctx)
if newLatest.After(oldLatest) {
    s.broadcastCOTRelease(ctx, newLatest, analyses) // NO DEDUP
}
```

## Solution

Add `lastBroadcastDate` field to scheduler with sync.Mutex:
```go
type Scheduler struct {
    mu               sync.Mutex
    lastCOTBroadcast time.Time
}

func (s *Scheduler) broadcastCOTRelease(ctx, date, analyses) {
    s.mu.Lock()
    if !date.After(s.lastCOTBroadcast) {
        s.mu.Unlock()
        return // already broadcast
    }
    s.lastCOTBroadcast = date
    s.mu.Unlock()
    // ... broadcast ...
}
```

## File Changes

- `internal/scheduler/scheduler.go` — Add lastCOTBroadcast + mutex, check before broadcast
- `internal/scheduler/scheduler.go` — Same pattern for FRED broadcast

## Acceptance Criteria

- [ ] Dedup field `lastCOTBroadcast time.Time` protected by mutex
- [ ] Duplicate broadcast prevented even under concurrent trigger
- [ ] Same pattern applied to FRED broadcast
- [ ] Log message when dedup catches duplicate
- [ ] No behavior change for normal single-trigger flow
