# Research Report: Tech Refactor Siklus 4 Putaran 5
# Test Coverage, Error Sentinels, Config Validation, Formatter Consolidation
**Date:** 2026-04-02 14:00 WIB
**Siklus:** 4/5 (Tech Refactor) — Putaran 5
**Author:** Research Agent

## Ringkasan

Critical finding: 18+ production files dengan ZERO tests di 4 core services. 13 locations silently ignoring errors. Config validation minimal. Formatter code heavily duplicated.

## Temuan 1: Test Coverage — 18+ Untested Production Files

### Critical untested services:
- **COT service:** analyzer.go, index.go, regime.go, thresholds.go — 0 tests
- **FRED service:** fetcher.go (807 LOC!), cache.go, alerts.go, rate_differential.go — 0 tests
- **Price service:** aggregator.go, hmm_regime.go, garch.go, hurst.go, levels.go, correlation.go — 0 tests
- **Backtest service:** 24+ files including walkforward.go, montecarlo.go, bootstrap.go — 0 tests

**Impact:** Bugs di services ini bisa silent corrupt output tanpa regression protection.

## Temuan 2: Silenced Errors — 13+ Locations

Pattern `_, _ =` yang suppress errors tanpa logging:

**fred/fetcher.go:539-573:** 10 FRED series fetches silently fail:
```go
data.RetailSalesExFood, _ = yoy("RSXFS")    // silent
data.UK_CPI, _ = yoy("GBRCPIALLMINMEI")     // silent
```

**cot/analyzer.go:46:** Options positions silently fail:
```go
records, _ = a.fetcher.FetchOptionsPositions(ctx, contracts, records)
```

**keyring/keyring.go:40:** `MustNext()` panics instead of graceful degradation:
```go
func (k *Keyring) MustNext() string {
    key, err := k.Next()
    if err != nil { panic(err) }  // CRASH!
}
```

## Temuan 3: Config Validation Minimal

config.go only validates 3 things (COTHistoryWeeks, COTFetchInterval, ConfluenceCalcInterval). Missing:
- Empty API keys when features enabled
- PriceFetchInterval bounds (could be 0 → busy loop)
- Storage directory permissions
- IntradayRetentionDays bounds (could be negative)
- No startup log of disabled features due to missing keys

## Temuan 4: Formatter Code Duplication

46+ header building patterns duplicated across 5 formatter files:
```go
// Each formatter reinvents the wheel:
sb.WriteString(fmt.Sprintf("🔷 <b>ICT — %s %s</b>\n", symbol, tf))
b.WriteString(fmt.Sprintf("📊 <b>WYCKOFF — %s %s</b>\n", symbol, tf))
sb.WriteString(fmt.Sprintf("📊 <b>GEX — %s</b>\n", symbol))
```

GEX formatter has its own `gexCommaSep()` when `fmtutil.FmtNum()` does the same thing. 15+ `strings.Repeat()` bar chart patterns scattered.

## Temuan 5: No Sentinel Error Package

Only 4 sentinel errors exist across entire codebase. Missing standard errors for:
- API rate limits, network timeouts, data parsing failures
- Cache miss vs cache error, insufficient data
- Feature disabled, auth failed

No pkg/errs/ package exists.
