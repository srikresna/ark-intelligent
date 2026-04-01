# TASK-174: Seasonal Analysis Nil Pointer Guard for New Contracts

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/service/price/

## Deskripsi

SeasonalPattern struct has pointer fields (RegimeStats, COTAlignment, EventDensity) yang nil untuk contracts dengan <1 year history. Calling code accesses these tanpa nil check → panic.

## Fix Pattern

Add nil checks di semua SeasonalPattern field accesses:
```go
if pattern.RegimeStats != nil {
    avgReturn = pattern.RegimeStats.AvgReturn
} else {
    avgReturn = 0 // no historical data
}
```

Dan di formatter:
```go
if pattern.COTAlignment != nil {
    // render COT alignment section
} else {
    sb.WriteString("COT alignment: insufficient history\n")
}
```

## File Changes

- `internal/service/price/seasonal.go` — Add nil checks before pointer field access
- `internal/adapter/telegram/formatter.go` — Add nil checks in seasonal formatting
- `internal/service/ta/confluence.go` — Add nil check jika seasonal data digunakan di confluence scoring

## Acceptance Criteria

- [ ] All SeasonalPattern pointer fields checked before access
- [ ] Formatter shows "insufficient history" untuk nil fields
- [ ] No panic on /seasonal for new instruments (e.g., SOL/USD, AVAX/USD)
- [ ] Unit test: SeasonalPattern with all-nil fields
- [ ] Graceful degradation — partial data still displayed
