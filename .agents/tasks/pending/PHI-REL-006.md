# Task: Fix WorldBank Client Goroutine Recovery — PHI-REL-006

## Summary
Add panic recovery to the goroutine in `internal/service/worldbank/client.go` to prevent cascading failures if a panic occurs during parallel country data fetching.

## Type
reliability

## Priority
MEDIUM

## Estimated Effort
XS (extra small — 30 min)

## Location
- `internal/service/worldbank/client.go:117`

## Issue Details

The goroutine at line 117 lacks panic recovery:

```go
for i, cc := range countryConfig {
    wg.Add(1)
    go func(idx int, code, currency string) {
        defer wg.Done()  // ← No panic recovery!
        macro := fetchCountry(ctx, code, currency)
        ch <- result{macro: macro, idx: idx}
    }(i, cc.Code, cc.Currency)
}
```

If `fetchCountry()` panics (e.g., due to unexpected API response), the goroutine will crash without:
1. Calling `wg.Done()` — causing `wg.Wait()` to hang indefinitely
2. Logging the panic — making debugging difficult

## Acceptance Criteria

- [ ] Add `defer func() { if r := recover(); r != nil { ... } }()` pattern to the goroutine
- [ ] Log the panic with appropriate context (country code, currency)
- [ ] Ensure `wg.Done()` is still called even on panic
- [ ] Consider populating the channel with an error marker or handling the partial failure gracefully

## Suggested Implementation

```go
go func(idx int, code, currency string) {
    defer func() {
        if r := recover(); r != nil {
            log.Error().Interface("panic", r).Str("country", code).Str("currency", currency).Msg("PANIC in WorldBank fetch")
        }
        wg.Done()
    }()
    macro := fetchCountry(ctx, code, currency)
    ch <- result{macro: macro, idx: idx}
}(i, cc.Code, cc.Currency)
```

## References
- Similar pattern already exists in `internal/service/news/scheduler.go:678-685`
- Part of reliability sprint PHI-REL-001 through PHI-REL-005

## Created
2026-04-04 12:24 UTC by Research Agent


## PR Link
- https://github.com/arkcode369/ark-intelligent/pull/389
