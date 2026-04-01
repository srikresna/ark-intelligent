# TASK-190: Unit Test Coverage — COT + FRED Services (Critical)

**Priority:** high
**Type:** test
**Estimated:** L
**Area:** internal/service/cot/, internal/service/fred/

## Deskripsi

Add unit tests untuk critical untested COT and FRED service files. These services drive all macro analysis — zero test coverage is unacceptable.

## Files to Test

### COT (target 60% coverage):
- `analyzer.go` — Test metric calculations, edge cases (empty data, single week)
- `index.go` — Test index helpers
- `regime.go` — Test regime classification boundaries
- `confluence.go` — Test score aggregation with partial data

### FRED (target 60% coverage):
- `alerts.go` — Test alert triggering with threshold crossings
- `rate_differential.go` — Test carry ranking, div-by-zero cases
- `regime_asset.go` — Test per-asset regime detection
- `cache.go` — Test cache hit/miss, TTL expiry

## Test Patterns

```go
func TestAnalyzer_EmptyData(t *testing.T) {
    a := NewAnalyzer(nil)
    result, err := a.Analyze(ctx, []domain.COTRecord{})
    require.Error(t, err)
}

func TestAlerts_SpreadInversion(t *testing.T) {
    prev := &MacroData{Spread3M10Y: 0.5}
    curr := &MacroData{Spread3M10Y: -0.1}
    alerts := CheckAlerts(curr, prev)
    require.Len(t, alerts, 1)
    assert.Contains(t, alerts[0].Message, "inversion")
}
```

## Acceptance Criteria

- [ ] COT analyzer: 10+ test cases covering empty, single, normal, extreme data
- [ ] COT regime: test each regime classification boundary
- [ ] FRED alerts: test each of 18 alert types
- [ ] FRED rate_differential: test carry ranking with NaN, zero, negative
- [ ] All tests pass: `go test ./internal/service/cot/... ./internal/service/fred/...`
- [ ] Coverage report generated
