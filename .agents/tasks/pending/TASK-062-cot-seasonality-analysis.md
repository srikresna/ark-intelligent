# TASK-062: COT Seasonality Analysis

**Priority:** MEDIUM
**Cycle:** Siklus 3 — Fitur Baru
**Estimated Complexity:** MEDIUM
**Research Ref:** `.agents/research/2026-04-01-02-fitur-baru-siklus3-lanjutan.md`

---

## Deskripsi

Implementasi analisis seasonalitas untuk COT positioning. Mirip dengan price seasonal (`internal/service/price/seasonal.go`), tapi untuk net positions COT per currency per week-of-year. Menggunakan data history CFTC yang sudah ada di BadgerDB plus opsi fetch CFTC multi-year CSV.

## Konteks Teknis

### Existing Foundation
- `internal/service/price/seasonal.go` — pola referensi untuk seasonal analysis
- `internal/service/price/seasonal_context.go` — context builder
- `internal/service/cot/analyzer.go` — existing COT analysis
- `ports.COTRepository.GetHistory(ctx, code, weeks)` — sudah ada, max 52 weeks

### Data Source Tambahan (GRATIS)
CFTC menyediakan CSV historis multi-tahun untuk data disaggregated:
```
# Current year combined (all contracts, weekly)
https://www.cftc.gov/dea/newcot/c_disagg.txt

# Historical ZIP files per year range (TFF dan Disaggregated)
https://www.cftc.gov/files/dea/history/dea_fut_xls_2020_2024.zip
# CSV format: market_code, report_date, oi, commercial_long, commercial_short, ...
```
Gunakan existing `ports.COTRepository.GetHistory()` untuk 52-week max dulu. CFTC multi-year bisa sebagai enhancement.

### Files Yang Perlu Dibuat

**`internal/service/cot/seasonal.go`:**
```go
package cot

import (
    "context"
    "time"
    
    "github.com/arkcode369/ark-intelligent/internal/domain"
    "github.com/arkcode369/ark-intelligent/internal/ports"
)

// COTSeasonalPoint holds the net position average for one week-of-year.
type COTSeasonalPoint struct {
    WeekOfYear int     // 1-52
    AvgNet     float64 // Average net position (long - short) for this week over history
    StdDev     float64 // Standard deviation across years
    SampleSize int     // Number of data points (years) used
}

// COTSeasonalResult holds the full seasonal analysis for one contract.
type COTSeasonalResult struct {
    ContractCode  string
    Currency      string
    CurrentWeek   int     // Current week of year
    CurrentNet    float64 // Current actual net position
    SeasonalAvg   float64 // Historical average for current week
    Deviation     float64 // CurrentNet - SeasonalAvg (in contracts)
    DeviationZ    float64 // Deviation / StdDev of seasonal avg
    Trend         string  // "ABOVE_SEASONAL", "BELOW_SEASONAL", "IN_LINE"
    SeasonalBias  string  // "SEASONALLY_BULLISH", "SEASONALLY_BEARISH", "NEUTRAL"
    Description   string  // Human-readable seasonal context
    
    // Seasonal curve (week-by-week averages for the year)
    Curve []COTSeasonalPoint
}

// SeasonalEngine computes COT seasonality analysis.
type SeasonalEngine struct {
    repo ports.COTRepository
}

func NewSeasonalEngine(repo ports.COTRepository) *SeasonalEngine {
    return &SeasonalEngine{repo: repo}
}

// Analyze computes COT seasonality for a single contract.
func (e *SeasonalEngine) Analyze(ctx context.Context, contractCode, currency string) (*COTSeasonalResult, error) {
    // 1. Fetch history (up to 52 weeks = 1 year)
    history, err := e.repo.GetHistory(ctx, contractCode, 52)
    if err != nil || len(history) < 8 {
        return nil, fmt.Errorf("insufficient history for seasonal: %w", err)
    }
    
    // 2. Group records by week-of-year
    // 3. For each week-of-year in history, compute mean net position
    // 4. Get current week, compare current net to historical average
    // 5. Compute deviation Z-score
    // 6. Generate trend + bias + description
}

// weekOfYear returns ISO week number for a given time.
func weekOfYear(t time.Time) int {
    _, week := t.ISOWeek()
    return week
}
```

### Seasonal Bias Classification

```
// Historical seasonal patterns (based on well-known forex COT tendencies)
// Will be dynamically computed from data, but these are validation anchors:

EUR: Q4 typically seasonally bullish (institutional year-end hedging)
JPY: Q1 typically seasonally bearish (carry trade unwinding in April)
AUD: Q2 typically bearish (commodities seasonal weakness)
Gold: Q3-Q4 typically bullish (physical demand season)
Oil: Q1 typically bearish (refinery maintenance season)

DeviationZ > 1.5  → "ABOVE_SEASONAL (SEASONALLY_BULLISH + current above avg)"
DeviationZ < -1.5 → "BELOW_SEASONAL (SEASONALLY_BEARISH + current below avg)"
|DeviationZ| < 1.5 → "IN_LINE"
```

### Integration ke /cot Output

Tambah di `/cot` handler atau buat `/cotseasonal [CURRENCY]` command:
```
📅 COT Seasonal Pattern [EURUSD — Week 14]
  • Seasonal Avg (Week 14): +24,500 contracts long
  • Current Actual: +31,200 contracts long
  • Deviation: +6,700 (+0.9σ above seasonal)
  → IN_LINE — positioning slightly above typical for this time of year
  
📊 Q2 Seasonal Outlook: EUR historically weakens in April-May
   (sample: 3 years of data — limited confidence)
```

## Acceptance Criteria
- [ ] `SeasonalEngine.Analyze()` compute week-of-year averages dari BadgerDB history
- [ ] `COTSeasonalResult` struct lengkap dengan Deviation, DeviationZ, Trend
- [ ] Seasonal bias description human-readable
- [ ] Terintegrasi ke /cot output (section tambahan, collapsed by default)
- [ ] Minimum data: 8 weeks (2 months); lebih banyak = lebih baik
- [ ] Test: mock history data 52 weeks, verify seasonal computation

## Notes
- Dengan hanya 52 weeks history = hanya 1 year → seasonal accuracy terbatas tapi berguna
- Enhancement future: fetch CFTC multi-year CSV untuk 5Y seasonal baseline
- Seasonal pattern untuk FX well-documented; bisa tambah hardcoded "known seasonals" sebagai context
