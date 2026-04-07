# Task: Fix FRED Fetcher Goroutine Recovery — PHI-REL-008

## Summary
Add panic recovery to the goroutine in `internal/service/fred/fetcher.go` to prevent cascading failures if a panic occurs during parallel FRED series fetching.

## Type
reliability

## Priority
MEDIUM

## Estimated Effort
XS (extra small — 30 min)

## Location
- `internal/service/fred/fetcher.go:343`

## Issue Details

The goroutine at line 343 lacks panic recovery:

```go
for i, job := range jobs {
    wg.Add(1)
    go func(idx int, j fetchJob) {
        defer wg.Done()  // ← No panic recovery!
        sem <- struct{}{}
        defer func() { <-sem }()
        obs := fetchSeries(ctx, client, j.id, apiKey, j.limit)
        results[idx] = fetchResult{id: j.id, obs: obs}
    }(i, job)
}
```

If `fetchSeries()` panics (e.g., due to unexpected API response), the goroutine will crash without:
1. Releasing the semaphore (`<-sem` may not execute)
2. Calling `wg.Done()` — causing `wg.Wait()` to hang indefinitely
3. Logging the panic — making debugging difficult

## Acceptance Criteria

- [ ] Add `defer func() { if r := recover(); r != nil { ... } }()` pattern to the goroutine
- [ ] Log the panic with appropriate context (series ID)
- [ ] Ensure both `wg.Done()` and semaphore release happen even on panic
- [ ] Consider using named return or careful defer ordering

## Suggested Implementation

```go
go func(idx int, j fetchJob) {
    defer func() {
        if r := recover(); r != nil {
            log.Error().Interface("panic", r).Str("series", j.id).Msg("PANIC in FRED fetch")
        }
        wg.Done()
    }()
    sem <- struct{}{}
    defer func() { <-sem }()
    obs := fetchSeries(ctx, client, j.id, apiKey, j.limit)
    results[idx] = fetchResult{id: j.id, obs: obs}
}(i, job)
```

## References
- Similar pattern already exists in `internal/service/news/scheduler.go:678-685`
- Part of reliability sprint PHI-REL-001 through PHI-REL-007

## Created
2026-04-04 12:24 UTC by Research Agent
