# TASK-009: Add OECD Consumer Confidence & Business Climate Indicators

**Priority:** MEDIUM
**Type:** New Data Source (free, extend existing client)
**Estimated effort:** S (half day)
**Ref:** research/2026-04-06-09-data-sources-siklus2.md

---

## Context

The /leading command shows OECD Composite Leading Indicators (CLI) from
`internal/service/macro/oecd_client.go`. This is the 6-9 month forward-looking
indicator. Missing: Consumer Confidence Index (CCI) and Business Confidence Index (BCI)
— direct leading indicators for consumer spending (70% of GDP in G10 economies).

OECD provides CCI and BCI via the same SDMX API already used for CLI — zero new
infrastructure needed. Same base URL, different dataset code.

OECD CCI > 100 = optimistic (risk-on), < 100 = pessimistic (risk-off)

---

## Implementation

### Extend `internal/service/macro/oecd_client.go`

The existing `fetchOECDSeries()` function is already general-purpose. Add two new
dataset fetchers using the same pattern as `FetchOECDCLI()`:

```go
// OECD Consumer Confidence Index (monthly)
// Flow: OECD.SDD.STES,DSD_STES@DF_MEI_BTS_COS,1.0
// Measure: CSESFT (consumer confidence, amplitude-adjusted, normalised)
func FetchOECDConsumerConfidence(ctx context.Context) (*OECDConfidenceData, error)

// OECD Business Confidence Index (monthly)
// Same flow, different measure: BSCICP03
func FetchOECDBusinessConfidence(ctx context.Context) (*OECDConfidenceData, error)
```

```go
type OECDConfidencePoint struct {
    Country   string  // ISO3 code
    Name      string  // display name
    Period    string  // YYYY-MM
    Value     float64 // index value (100 = long-term avg)
    Change    float64 // MoM change
    Trend     string  // "RISING", "FALLING", "STABLE"
}

type OECDConfidenceData struct {
    Type      string               // "CCI" or "BCI"
    Points    []OECDConfidencePoint
    FetchedAt time.Time
}
```

Countries: same as CLI (USA, G7, GBR, JPN, CAN, AUS, DEU, FRA, CHN, KOR)

Cache TTL: 24 hours (same as CLI — monthly data)

### Integrate into /leading output

In `internal/adapter/telegram/handler_macro_cmd.go` (or handler_leading.go),
add CCI/BCI section after existing CLI section:

```
📊 Consumer Confidence (OECD, monthly)
🇺🇸 US:   102.3 (+0.8) RISING  🟢
🇩🇪 DE:    98.7 (-0.3) FALLING 🔴
🇬🇧 UK:   100.1 (+0.1) STABLE  🟡
🇯🇵 JP:    99.4 (-0.6) FALLING 🔴
```

### Divergence signal

Add a simple divergence check:
- CLI rising + CCI rising = "Macro + Consumer aligned bullish" (strong)
- CLI rising + CCI falling = "Business optimistic, consumers cautious" (divergence)

---

## SDMX API Details

OECD Statistics SDMX base: https://sdmx.oecd.org/public/rest/data
CCI dataset: OECD.SDD.STES,DSD_STES@DF_MEI_BTS_COS,1.0
BCI dataset: same dataset, different measure code
Measure codes: CSESFT (consumer), BSCICP03 (business)
Format: CSV (same as existing CLI client)
Auth: None required

---

## Acceptance Criteria

- [ ] `FetchOECDConsumerConfidence()` in oecd_client.go
- [ ] `FetchOECDBusinessConfidence()` in oecd_client.go
- [ ] CCI section visible in `/leading` output
- [ ] BCI section visible in `/leading` output
- [ ] CLI/CCI divergence signal shown when applicable
- [ ] Cache TTL = 24h, graceful degradation on API error
- [ ] `go build ./...` passes, existing CLI tests still pass
