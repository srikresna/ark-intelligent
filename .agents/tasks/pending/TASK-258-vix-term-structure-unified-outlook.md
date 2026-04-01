# TASK-258: VIX Term Structure (vix/ package) → UnifiedOutlookData Integration

**Priority:** medium
**Type:** data-source
**Estimated:** S
**Area:** internal/service/ai/unified_outlook.go, internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 09:00 WIB

## Deskripsi

Package `internal/service/vix/` sudah diimplementasi lengkap dengan:
- VIX spot dari CBOE CDN CSV (`VIX_EOD.csv`)
- VX M1/M2 futures settle dari `VX_EOD.csv`
- VVIX dari `VVIX_EOD.csv`
- Computed: contango/backwardation flag, slope %, regime classification
- Cache 6 jam (vix/cache.go)

**Masalah:** Service ini TIDAK masuk ke `UnifiedOutlookData`. Saat ini unified_outlook
hanya dapat VIX term regime dari FRED macro data (VIXCLS/VXVCLS — dua seri terpisah
yang sering lag). Padahal `vix/` package punya M1/M2 futures prices + VVIX yang
memberikan picture lebih lengkap untuk volatility regime assessment.

Perbedaan nilai vs FRED approach:
- FRED `VXVCLS` = 3-month constant maturity VIX, bukan actual futures settle
- `vix/` package = actual VX M1/M2 futures settle + VVIX
- Regime dari `vix/` lebih presisi untuk contango/backwardation call

## File yang Harus Diubah

1. `internal/service/ai/unified_outlook.go` — tambah `VIXTermData *vix.VIXTermStructure` field + section
2. `internal/adapter/telegram/handler.go` — fetch dari `vix.Cache`, inject ke unifiedData

## Implementasi

### 1. unified_outlook.go — tambah field

Di `UnifiedOutlookData` struct (tambah import vixsvc):
```go
import vixsvc "github.com/arkcode369/ark-intelligent/internal/service/vix"

// VIXTermData holds detailed VIX futures term structure from CBOE CDN.
// More precise than FRED VXVCLS for contango/backwardation signaling.
VIXTermData *vixsvc.VIXTermStructure
```

Di `BuildUnifiedOutlookPrompt()`, tambah section baru setelah Risk Sentiment:
```go
// Section: VIX Futures Term Structure (dedicated vix package)
if data.VIXTermData != nil && data.VIXTermData.Available {
    ts := data.VIXTermData
    b.WriteString(fmt.Sprintf("=== %d. VIX FUTURES TERM STRUCTURE ===\n", section))
    section++
    b.WriteString(fmt.Sprintf("VIX Spot: %.2f | M1: %.2f | M2: %.2f | VVIX: %.2f\n",
        ts.VIXSpot, ts.M1Settle, ts.M2Settle, ts.VVIX))
    b.WriteString(fmt.Sprintf("Regime: %s | Contango: %v | Slope M1→M2: %.1f%%\n",
        ts.Regime, ts.Contango, ts.SlopePct))
    b.WriteString("NOTE: M1>Spot=backwardation(fear), M1<Spot=contango(calm). " +
        "VVIX>120=vol-of-vol spike (reflexive selling risk).\n\n")
}
```

### 2. handler.go — fetch VIX term structure

Tambah `vixCache *vixsvc.Cache` ke Handler struct (atau konstruksi inline):
```go
// Di struct Handler atau via field baru:
vixCache *vixsvc.Cache
```

Di konstruktor `NewHandler()` atau `WithVIX()`:
```go
h.vixCache = vixsvc.NewCache()
```

Di `cmdOutlook` sebelum construct `unifiedData`:
```go
var vixTermData *vixsvc.VIXTermStructure
if h.vixCache != nil {
    vixTermData, _ = h.vixCache.Get(ctx)
}
```

Inject:
```go
unifiedData := aisvc.UnifiedOutlookData{
    // ... existing ...
    VIXTermData: vixTermData,
}
```

**Alternatif lebih simple** (jika tidak mau modifikasi Handler struct):
Buat `vixsvc.NewCache()` inline di cmdOutlook dan fetch langsung:
```go
ts, _ := vixsvc.FetchTermStructure(ctx) // direct fetch, no cache
```
Tapi ini boros — sebaiknya pakai cache di Handler.

## Acceptance Criteria

- [ ] `UnifiedOutlookData` punya field `VIXTermData *vixsvc.VIXTermStructure`
- [ ] `/outlook` prompt includes "VIX FUTURES TERM STRUCTURE" section saat data available
- [ ] Section shows: VIX Spot, M1, M2, VVIX, Regime, Contango flag, Slope %
- [ ] Jika `VIXTermData == nil` atau `Available == false` → section dilewati
- [ ] Handler fetch VIX term structure (via cache) sebelum construct unifiedData
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-09-data-sources-audit-gaps-putaran13.md` — Temuan #1
- `internal/service/vix/types.go:6` — VIXTermStructure struct (fields tersedia)
- `internal/service/vix/cache.go:26` — Cache.Get(ctx) method
- `internal/service/vix/fetcher.go:25` — FetchTermStructure(ctx) direct
- `internal/service/ai/unified_outlook.go:22` — UnifiedOutlookData struct
- `internal/adapter/telegram/handler.go:1004` — unifiedData construction point
