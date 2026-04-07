# TASK-008: COT Open Interest Trend Analysis

**Priority:** MEDIUM
**Type:** Feature Enhancement (no new data source)
**Estimated effort:** M (1 day)
**Ref:** research/2026-04-06-09-data-sources-siklus2.md

---

## Context

Open Interest (OI) data is already fetched from CFTC and stored in:
- `domain.COTRecord.OpenInterest` (total OI)
- `domain.COTRecord.OpenInterestChg` (WoW change)
- `domain.COTRecord.OIPctChange` (% change)

But this data is NOT used in any meaningful analysis. The /cot output doesn't
show OI trends, and no signals are generated from OI expansion/contraction patterns.

OI is one of the most reliable institutional positioning confirmation tools:
- Rising OI + rising net long = fresh institutional longs (accumulation)
- Rising OI + rising net short = fresh institutional shorts (distribution)
- Falling OI from extreme = positioning unwind (reversal risk)

---

## Implementation

### 1. Add OI trend signals to `internal/service/cot/signals.go`

New signal types:
```go
const (
    SignalOIExpansionBull = "OI_EXPANSION_BULL" // OI expanding + net longs rising
    SignalOIExpansionBear = "OI_EXPANSION_BEAR" // OI expanding + net shorts rising
    SignalOIUnwindRisk    = "OI_UNWIND_RISK"    // OI contracting from extreme percentile
    SignalOIDivergence    = "OI_DIVERGENCE"     // OI expanding but net position reversing
)
```

New function: `DetectOITrendSignal(records []domain.COTRecord) *OITrendSignal`
- Requires minimum 4 weeks of history
- OI expansion = OI growing for 3+ consecutive weeks
- Extreme = OI in 90th percentile vs trailing 52-week range
- Unwind = OI dropped >10% from 52-week high + net position at extreme

### 2. Add OI trend to `internal/service/cot/confluence_score.go`

Add OI expansion confirmation as a 6th component to ConvictionScoreV3:
- +1 if OI expanding with directional conviction
- 0 if OI neutral
- -1 if OI contracting from extreme (reduces conviction)

### 3. Add OI trend display to `/cot` output

In `internal/adapter/telegram/format_cot.go`, add an OI section:

```
📊 Open Interest Trend
OI: 458,234 (+3.2% WoW) — 3 wks expanding
Signal: 🟢 OI_EXPANSION_BULL — Institutional accumulation confirmed
OI Percentile: 78th (52-week range)
```

### 4. Add OI history to `GetHistory` queries

Ensure `cotRepo.GetHistory(ctx, code, 8)` is used (8 weeks) in the OI analysis
to have enough data for trend detection. Current history calls use 4 weeks.

---

## Data Flow

```
CFTC API → fetcher.go → domain.COTRecord (OpenInterest, OpenInterestChg)
         → analyzer.go → COTAnalysis (OIChangeAPI, PctOfOI)
         → signals.go  → NEW: OITrendSignal
         → format_cot.go → display in /cot output
```

---

## Acceptance Criteria

- [ ] `DetectOITrendSignal()` function in signals.go
- [ ] OI signals: OI_EXPANSION_BULL, OI_EXPANSION_BEAR, OI_UNWIND_RISK, OI_DIVERGENCE
- [ ] OI section visible in `/cot EUR` output (when 4+ weeks data available)
- [ ] OI expansion correctly adjusts ConvictionScoreV3 (+1/-1)
- [ ] Minimum 4 weeks history required (graceful: "Insufficient data" if less)
- [ ] All existing COT tests pass
- [ ] `go build ./...` passes
