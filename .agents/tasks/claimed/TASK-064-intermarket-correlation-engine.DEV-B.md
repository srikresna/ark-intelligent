# TASK-064: Intermarket Correlation Signal Engine

**Priority:** MEDIUM
**Cycle:** Siklus 3 — Fitur Baru
**Estimated Complexity:** MEDIUM-HIGH
**Research Ref:** `.agents/research/2026-04-01-02-fitur-baru-siklus3-lanjutan.md`

---

## Deskripsi

Implementasi rules-based intermarket correlation engine untuk forex. Engine mendeteksi ketika harga bergerak MELAWAN hubungan intermarket yang diketahui (divergence = signal), menggunakan data price yang sudah ada dari existing services.

## Konteks Teknis

### Data Yang Sudah Ada
- `internal/service/price/correlation.go` — correlation matrix sudah ada
- `internal/service/price/aggregator.go` — multi-pair price data
- `internal/service/price/eia.go` — Oil data (WTI)
- `internal/service/fred/` — yield data, DXY dari FRED
- `internal/service/price/fetcher.go` — TwelveData/Polygon price data

### Known Intermarket Relationships (FX-specific)

```go
// IntermarketRule defines a known relationship between two assets.
type IntermarketRule struct {
    Base       string  // e.g. "AUDUSD"
    Correlated string  // e.g. "XAUUSD" (Gold)
    Direction  int     // +1 = positive correlation, -1 = negative
    Window     int     // Rolling correlation window in days
    Label      string  // Human-readable relationship name
    Reliability string // "HIGH", "MEDIUM" — confidence in this relationship
}

var StandardRules = []IntermarketRule{
    // AUD = commodity + risk currency
    {Base: "AUDUSD", Correlated: "XAUUSD", Direction: +1, Window: 20, Label: "AUD-Gold", Reliability: "HIGH"},
    {Base: "AUDUSD", Correlated: "SPX500", Direction: +1, Window: 20, Label: "AUD-Equities (risk-on)", Reliability: "HIGH"},
    
    // CAD = oil currency
    {Base: "CADUSD", Correlated: "CL_OIL", Direction: +1, Window: 20, Label: "CAD-Oil", Reliability: "HIGH"},
    // Note: USD/CAD is inverse, so CADUSD positive with oil = USDCAD negative with oil
    
    // JPY = safe haven
    {Base: "JPYUSD", Correlated: "US10Y", Direction: -1, Window: 20, Label: "JPY-Yields (carry)", Reliability: "HIGH"},
    {Base: "JPYUSD", Correlated: "SPX500", Direction: -1, Window: 20, Label: "JPY-Equities (risk-off)", Reliability: "HIGH"},
    
    // CHF = safe haven
    {Base: "CHFUSD", Correlated: "XAUUSD", Direction: +1, Window: 20, Label: "CHF-Gold (safe haven)", Reliability: "MEDIUM"},
    
    // DXY relationships
    {Base: "DXY", Correlated: "XAUUSD", Direction: -1, Window: 20, Label: "DXY-Gold (inverse)", Reliability: "HIGH"},
    {Base: "DXY", Correlated: "EURUSD", Direction: -1, Window: 20, Label: "DXY-EUR (definitional)", Reliability: "HIGH"},
    
    // Cross-asset risk regime
    {Base: "XAUUSD", Correlated: "SPX500", Direction: -1, Window: 20, Label: "Gold-Equities (crisis hedge)", Reliability: "MEDIUM"},
}
```

### Algorithm

```
1. For each IntermarketRule:
   a. Fetch 30-day price history for Base and Correlated
   b. Compute rolling 20-day Pearson correlation
   c. Check: does actual correlation sign match expected Direction?
   d. Compute divergence score = actual_corr - expected_direction

2. Divergence detection:
   - STRONG DIVERGENCE: actual_corr * expected_direction < -0.3 (opposite of expected)
   - WEAK DIVERGENCE: actual_corr * expected_direction < 0 (slightly off)
   - IN_LINE: actual_corr * expected_direction > 0.2
   
3. For divergences → generate trading signal:
   - "AUD rising but Gold falling → AUD overextended, fade rally"
   - "JPY weakening despite risk-off → JPY signal broken, watch for snap-back"
```

### Files Yang Perlu Dibuat

**`internal/service/intermarket/types.go`:**
```go
package intermarket

// IntermarketRule, IntermarketSignal, CorrelationStatus types
type CorrelationStatus string
const (
    StatusAligned   CorrelationStatus = "ALIGNED"
    StatusDiverging CorrelationStatus = "DIVERGING"
    StatusBroken    CorrelationStatus = "BROKEN"
)

type IntermarketSignal struct {
    Rule        IntermarketRule
    ActualCorr  float64         // rolling 20D Pearson correlation
    Status      CorrelationStatus
    Implication string          // trading implication
    Strength    float64         // 0-1 confidence
}

type IntermarketResult struct {
    Signals     []IntermarketSignal
    Divergences []IntermarketSignal // only DIVERGING/BROKEN signals
    RiskRegime  string              // "RISK_ON", "RISK_OFF", "MIXED"
    AsOf        time.Time
}
```

**`internal/service/intermarket/engine.go`:**
```go
package intermarket

// Engine computes intermarket signals using existing price data.
type Engine struct {
    priceRepo price.DailyPriceStore // existing interface
    fredSvc   *fred.Service         // existing FRED service
}

func (e *Engine) Analyze(ctx context.Context) (*IntermarketResult, error) {
    // 1. Fetch 30-day price histories for needed symbols
    // 2. Compute rolling correlations
    // 3. Check against rules
    // 4. Generate signals + risk regime
}
```

**Data Symbol Mapping:**
- AUD → AUDUSD (or USDAUD inverse)
- Gold → XAUUSD via existing price fetcher
- Oil → use EIA WTI data (already in `price/eia.go`)
- SPX500 → already tracked in price service (`SPX500` in `domain.PriceSymbolMapping`)
- US10Y → from FRED service (`DGS10` series)
- DXY → FRED `DTWEXBGS` or from price service

### New Telegram Command: /intermarket

```
🔗 Intermarket Correlation [2026-04-01]

🟢 ALIGNED (5/8 relationships on track):
  • AUD-Gold: +0.72 corr ✅ (expected positive)
  • CAD-Oil: +0.81 corr ✅
  • JPY-Yields: -0.68 corr ✅
  • DXY-Gold: -0.79 corr ✅
  • DXY-EUR: -0.95 corr ✅

🔴 DIVERGING (3/8 relationships breaking):
  • JPY-Equities: +0.31 corr ⚠️ (expected negative)
    → JPY weakening WITH equities rising = risk-on override, carry trade back
  • AUD-Equities: -0.42 corr ⚠️ (expected positive)
    → AUD not following risk-on = watch for AUD catch-up or equity fade
  • Gold-Equities: +0.55 corr ⚠️ (expected negative)
    → Both rising = dollar weakness driving both, not true risk-off

📊 Risk Regime: MIXED (conflicting signals)
   → Trade individual divergences rather than broad regime bias
```

## Acceptance Criteria
- [ ] `Engine.Analyze()` computes rolling 20D correlations for all standard rules
- [ ] Divergence detection: ALIGNED / DIVERGING / BROKEN classification
- [ ] Trading implication text generated per divergence
- [ ] Risk regime (RISK_ON / RISK_OFF / MIXED) synthesized from signals
- [ ] /intermarket command wired in bot handler
- [ ] Graceful degradation: missing data = skip that rule, don't error
- [ ] Cache 4 hours (correlations don't change minute-to-minute)

## Notes
- CADUSD vs CAD: price fetcher might have USDCAD, need to invert prices for rule
- SPX500 daily close already available (check `domain.PriceSymbolMapping`)
- For minimal dependency: use existing `price.DailyPriceStore` via `GetDailyHistory()`
- pearsonCorrelation() function: simple O(n) implementation, doesn't need a library
- Handle thin/missing data: if < 15 data points, mark correlation as "insufficient data"
