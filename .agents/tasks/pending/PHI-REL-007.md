# Task: Fix BIS REER Client Goroutine Recovery — PHI-REL-007

## Summary
Add panic recovery to the goroutine in `internal/service/bis/reer.go` to prevent cascading failures if a panic occurs during parallel currency data fetching.

## Type
reliability

## Priority
MEDIUM

## Estimated Effort
XS (extra small — 30 min)

## Location
- `internal/service/bis/reer.go:138`

## Issue Details

The goroutine at line 138 lacks panic recovery:

```go
for i, cc := range currencyConfig {
    wg.Add(1)
    go func(idx int, code, currency string) {
        defer wg.Done()  // ← No panic recovery!
        data := fetchCurrency(ctx, code, currency)
        ch <- result{data: data, idx: idx}
    }(i, cc.Code, cc.Currency)
}
```

If `fetchCurrency()` panics (e.g., due to unexpected API response), the goroutine will crash without:
1. Calling `wg.Done()` — causing `wg.Wait()` to hang indefinitely
2. Logging the panic — making debugging difficult

## Acceptance Criteria

- [ ] Add `defer func() { if r := recover(); r != nil { ... } }()` pattern to the goroutine
- [ ] Log the panic with appropriate context (currency code)
- [ ] Ensure `wg.Done()` is still called even on panic
- [ ] Consider populating the channel with an error marker or handling the partial failure gracefully

## Suggested Implementation

```go
go func(idx int, code, currency string) {
    defer func() {
        if r := recover(); r != nil {
            log.Error().Interface("panic", r).Str("currency", currency).Msg("PANIC in BIS REER fetch")
        }
        wg.Done()
    }()
    data := fetchCurrency(ctx, code, currency)
    ch <- result{data: data, idx: idx}
}(i, cc.Code, cc.Currency)
```

## References
- Similar pattern already exists in `internal/service/news/scheduler.go:678-685`
- Part of reliability sprint PHI-REL-001 through PHI-REL-006

## Created
2026-04-04 12:24 UTC by Research Agent
