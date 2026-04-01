# TASK-061: VIX Term Structure Engine (CBOE CSV — Gratis)

**Priority:** HIGH
**Cycle:** Siklus 3 — Fitur Baru
**Estimated Complexity:** LOW-MEDIUM
**Research Ref:** `.agents/research/2026-04-01-02-fitur-baru-siklus3-lanjutan.md`

---

## Deskripsi

Implementasi VIX term structure engine menggunakan CBOE free CSV data. Engine menghitung apakah VIX futures dalam contango (risk-on) atau backwardation (risk-off) dan mengintegrasikan ke output /sentiment dan makro regime.

## Konteks Teknis

### Data Source GRATIS (no API key)
```
# VIX Futures End-of-Day Settlement
https://cdn.cboe.com/api/global/us_indices/daily_prices/VX_EOD.csv
Format: Trade Date, Futures, Open, High, Low, Close, Settle, Change, %Change, Volume, EFP, Open Interest
Example: 2026-03-31, /VXK26 (May 2026), ..., 21.30, ...

# VIX Spot Index
https://cdn.cboe.com/api/global/us_indices/daily_prices/VIX_EOD.csv

# VVIX (VIX of VIX)
https://cdn.cboe.com/api/global/us_indices/daily_prices/VVIX_EOD.csv
```

### Existing Integration Points
- `internal/service/sentiment/sentiment.go` — `SentimentData` struct (add VIX fields here)
- `internal/service/sentiment/cache.go` — existing cache pattern to follow
- /sentiment command handler — add VIX section to output

### Files Yang Perlu Dibuat

**`internal/service/vix/types.go`:**
```go
package vix

import "time"

// VIXTermStructure holds the VIX spot and futures term structure.
type VIXTermStructure struct {
    Spot       float64 // VIX spot index
    M1         float64 // Front-month VIX futures settle price
    M2         float64 // Second-month VIX futures settle price
    M3         float64 // Third-month VIX futures settle price
    VVIX       float64 // VIX of VIX (volatility of volatility)
    
    M1Symbol   string  // e.g. "/VXK26"
    M2Symbol   string
    M3Symbol   string
    
    // Derived signals
    Contango     bool    // true if M1 > Spot and M2 > M1 (normal/risk-on)
    Backwardation bool   // true if M1 < Spot (fear/risk-off)
    SlopePct     float64 // (M2-M1)/M1 * 100 — % slope of term structure
    RollYield    float64 // approx monthly roll cost/benefit for VIX long (%/month)
    
    // Regime interpretation
    Regime  string // "RISK_ON_COMPLACENT", "RISK_ON_NORMAL", "ELEVATED", "FEAR", "EXTREME_FEAR"
    Signal  string // trading signal based on term structure
    
    AsOf     time.Time
    Available bool
}
```

**`internal/service/vix/engine.go`:**
```go
package vix

import (
    "context"
    "encoding/csv"
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "time"
)

const (
    vxEODURL   = "https://cdn.cboe.com/api/global/us_indices/daily_prices/VX_EOD.csv"
    vixEODURL  = "https://cdn.cboe.com/api/global/us_indices/daily_prices/VIX_EOD.csv"
    vvixEODURL = "https://cdn.cboe.com/api/global/us_indices/daily_prices/VVIX_EOD.csv"
)

// FetchTermStructure fetches VIX spot and futures from CBOE and
// computes the term structure metrics.
func FetchTermStructure(ctx context.Context) (*VIXTermStructure, error) {
    ts := &VIXTermStructure{AsOf: time.Now()}
    client := &http.Client{Timeout: 15 * time.Second}
    
    // 1. Fetch VIX spot from VIX_EOD.csv (last row)
    // 2. Fetch VX_EOD.csv — filter to last trading day, group by contract month
    //    → sort by expiry, take M1/M2/M3
    // 3. Fetch VVIX_EOD.csv (last row)
    // 4. Compute slope, contango/backwardation, regime
    
    // ... implementation
    ts.Available = true
    return ts, nil
}
```

**VX_EOD.csv parsing logic:**
- CSV contains multiple rows per date (one per active contract)
- Filter to latest trade date
- Parse contract symbol (e.g. `/VXK26`) to get expiry month/year
- Sort by expiry → M1=nearest, M2=next, M3=third
- Use "Settle" column for settlement price

**Regime classification:**
```
SlopePct := (M2 - M1) / M1 * 100

if Spot > M1:
    Backwardation = true
    if Spot > M1*1.10: Regime = "EXTREME_FEAR"
    else: Regime = "FEAR"
elif SlopePct < 3:
    Regime = "ELEVATED" (flat term structure, elevated vol)
elif SlopePct < 7:
    Regime = "RISK_ON_NORMAL"
else:
    Regime = "RISK_ON_COMPLACENT" (steep contango = max complacency)
```

**`internal/service/vix/cache.go`:**
- TTL: 6 hours (CBOE updates end-of-day)
- Pattern: identical to `internal/service/sentiment/cache.go`

### Integration ke SentimentData

Tambahkan ke `internal/service/sentiment/sentiment.go`:
```go
type SentimentData struct {
    // ... existing fields ...
    
    // VIX Term Structure
    VIXSpot         float64
    VIXM1           float64
    VIXM2           float64
    VIXContango     bool
    VIXSlopePct     float64
    VIXRegime       string
    VIXAvailable    bool
}
```

### /sentiment output tambahan:
```
📈 VIX Term Structure:
  • Spot: 18.2 | M1: 19.4 | M2: 21.1
  • Structure: CONTANGO (+8.8% slope) → RISK_ON_COMPLACENT
  • VVIX: 87.3 (vol-of-vol elevated → uncertainty about direction)
  ⚡ Signal: Steep contango = market complacent, VIX ETPs bleed → equity-bullish
```

## Acceptance Criteria
- [ ] `FetchTermStructure()` berhasil parse CBOE CSV dan return M1/M2/M3 settle prices
- [ ] Contango/backwardation detection berfungsi
- [ ] Regime classification: 5 kategori ter-define dengan benar
- [ ] Terintegrasi ke SentimentData
- [ ] Cache 6 jam berjalan (tidak hit CBOE setiap request)
- [ ] Graceful fallback jika CBOE URL tidak tersedia

## Notes
- CBOE CSV tidak memerlukan API key — plain HTTP GET
- VX_EOD.csv bisa berukuran besar (multi-tahun), ambil hanya baris terakhir per filter
- Test: mock HTTP response dengan sample CSV dari CBOE
