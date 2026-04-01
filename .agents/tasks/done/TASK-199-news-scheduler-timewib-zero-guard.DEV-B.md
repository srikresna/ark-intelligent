# TASK-199: News Scheduler — TimeWIB Zero-Value Guard

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/news/

## Deskripsi

Add IsZero() check sebelum computing time differences dengan e.TimeWIB. Zero-valued time.Time produces wrong durations → missed or premature alerts.

## Bug Detail

```go
// scheduler.go:435
minsUntil := int(e.TimeWIB.Sub(now).Minutes())
// If e.TimeWIB is time.Time{}, Sub() returns ~-60 years duration!

// scheduler.go:541
minsSinceRelease := int(now.Sub(e.TimeWIB).Minutes())
// Same issue
```

## Fix

```go
if e.TimeWIB.IsZero() {
    continue  // skip events with invalid times
}
minsUntil := int(e.TimeWIB.Sub(now).Minutes())
```

## File Changes

- `internal/service/news/scheduler.go` — Add IsZero() check at lines 435 and 541

## Acceptance Criteria

- [ ] Events with zero-value TimeWIB skipped
- [ ] Log warning for zero-value events (data quality issue)
- [ ] Normal events with valid TimeWIB unaffected
- [ ] No premature alert triggers from invalid times
