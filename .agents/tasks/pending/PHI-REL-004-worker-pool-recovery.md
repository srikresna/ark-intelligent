# PHI-REL-004: Add Panic Recovery to Worker Pool Goroutines

## Problem Statement

Three worker pool implementations spawn goroutines without panic recovery. While these are bounded by semaphores/WaitGroup, a panic in any worker will crash the entire process.

### Location 1: FRED Fetcher
`internal/service/fred/fetcher.go:343-349`

```go
for i, job := range jobs {
    wg.Add(1)
    go func(idx int, j fetchJob) {
        defer wg.Done()
        sem <- struct{}{}
        defer func() { <-sem }()
        obs := fetchSeries(ctx, client, j.id, apiKey, j.limit)  // Could panic
        results[idx] = fetchResult{id: j.id, obs: obs}
    }(i, job)
}
```

### Location 2: BIS REER Fetcher
`internal/service/bis/reer.go:138-142`

```go
for i, cc := range currencyConfig {
    wg.Add(1)
    go func(idx int, code, currency string) {
        defer wg.Done()
        data := fetchCurrency(ctx, code, currency)  // Could panic
        ch <- result{data: data, idx: idx}
    }(i, cc.Code, cc.Currency)
}
```

### Location 3: World Bank Client
`internal/service/worldbank/client.go:117-121`

```go
for i, cc := range countryConfig {
    wg.Add(1)
    go func(idx int, code, currency string) {
        defer wg.Done()
        macro := fetchCountry(ctx, code, currency)  // Could panic
        ch <- result{macro: macro, idx: idx}
    }(i, cc.Code, cc.Currency)
}
```

## Expected Behavior

- All worker goroutines should have defer/recover protection
- Panic in one worker should not crash the process
- Log which job/index caused the panic
- Return partial results for non-panicked workers
- Worker should still signal completion (wg.Done()) even on panic

## Acceptance Criteria

- [ ] Add panic recovery to FRED fetcher worker goroutines
- [ ] Add panic recovery to BIS REER fetcher worker goroutines
- [ ] Add panic recovery to World Bank client worker goroutines
- [ ] Log recovered panic with job identifier (index, ID, or currency code)
- [ ] Ensure `defer wg.Done()` still executes via separate defer or wrapper
- [ ] Verify partial results are returned for successful workers
- [ ] Optional: Add metric counters for panics per service

## Implementation Pattern

```go
// For FRED fetcher
go func(idx int, j fetchJob) {
    defer func() {
        if r := recover(); r != nil {
            log.Error().Interface("panic", r).Str("series_id", j.id).Int("idx", idx).Msg("fetchSeries panic recovered")
        }
        wg.Done()
    }()
    sem <- struct{}{}
    defer func() { <-sem }()
    obs := fetchSeries(ctx, client, j.id, apiKey, j.limit)
    results[idx] = fetchResult{id: j.id, obs: obs}
}(i, job)

// For BIS and World Bank (similar pattern)
go func(idx int, code, currency string) {
    defer func() {
        if r := recover(); r != nil {
            log.Error().Interface("panic", r).Str("code", code).Str("currency", currency).Msg("fetch panic recovered")
        }
        wg.Done()
    }()
    data := fetchCurrency(ctx, code, currency)
    ch <- result{data: data, idx: idx}
}(i, cc.Code, cc.Currency)
```

## Files to Modify

- `internal/service/fred/fetcher.go` — worker pool loop
- `internal/service/bis/reer.go` — worker pool loop
- `internal/service/worldbank/client.go` — worker pool loop

## Risk Assessment

**Impact**: MEDIUM — Worker panic crashes process  
**Effort**: LOW — 30 minutes for all three  
**Priority**: P2 (Medium)

## Related

- Similar pattern to PHI-REL-001 and PHI-REL-002
- New finding from Research Agent scheduled audit (Run 9)
