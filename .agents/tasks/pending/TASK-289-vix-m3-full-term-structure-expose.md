# TASK-289: VIX M3 + Full Term Structure Curve Slope — Surface ke /sentiment

**Priority:** low
**Type:** enhancement
**Estimated:** S
**Area:** internal/service/vix/types.go, internal/service/sentiment/sentiment.go, internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-02 27:00 WIB

## Deskripsi

VIX M3 (3rd month futures) sudah di-fetch di `vix/fetcher.go:218-219` dan tersimpan di `VIXTermStructure.M3`, tapi **tidak pernah digunakan di output manapun**. Dead data.

Dengan M3, kita bisa menampilkan **full 4-point term structure**: Spot → M1 → M2 → M3, dan menghitung:
- `FullSlopePct = (M3 - M1) / M1 * 100` — lebih stabil dari M2-M1
- `CalendarSpreadM2M3 = M3 - M2` — forward vol expectation
- Steeper slope = lebih risk-on; flatter/inverted = fear building

Ini adalah improvement kecil (S effort) dengan nilai yang signifikan untuk `/sentiment` output.

## Perubahan yang Diperlukan

### 1. Tambah fields ke `internal/service/vix/types.go`

```go
type VIXTermStructure struct {
    // ... existing fields ...
    
    // Full term structure metrics — NEW
    FullSlopePct     float64 // (M3-M1)/M1 * 100 — 2-month slope
    CalendarM2M3     float64 // M3 - M2 (forward premium/discount)
    FullContango     bool    // true if M3 > M2 > M1 > Spot
    FullBackwardation bool   // true if M3 < M2 < M1 < Spot (rare, extreme fear)
}
```

### 2. Hitung di `internal/service/vix/fetcher.go` setelah fetch M3

Di `FetchTermStructure()`, setelah `ts.M3` ter-populate:

```go
// Full term structure metrics
if ts.M3 > 0 && ts.M1 > 0 {
    ts.FullSlopePct = (ts.M3 - ts.M1) / ts.M1 * 100
    ts.CalendarM2M3 = ts.M3 - ts.M2
    ts.FullContango = ts.M3 > ts.M2 && ts.M2 > ts.M1 && ts.M1 > ts.Spot
    ts.FullBackwardation = ts.M3 < ts.M2 && ts.M2 < ts.M1 && ts.M1 > ts.Spot
}
```

### 3. Pass ke `SentimentData` di `internal/service/sentiment/sentiment.go`

Tambah ke `SentimentData`:
```go
// VIX full term structure
VIXM3          float64 // 3rd month VIX futures
VIXFullSlope   float64 // (M3-M1)/M1 * 100
VIXCalM2M3     float64 // M3 - M2 calendar spread
VIXFullContango bool
```

Di `Fetch()` method, setelah VIX term structure di-fetch, copy fields baru ke SentimentData.

### 4. Update `/sentiment` formatter di `internal/adapter/telegram/formatter.go`

Di seksi VIX Term Structure, ubah dari 3-point ke 4-point display:

```go
// Before (3-point):
// VIX: 18.2 | M1: 19.4 | M2: 20.1 | Slope: +3.6%

// After (4-point):
b.WriteString(fmt.Sprintf("<code>  Spot: %-6.1f M1: %-6.1f M2: %-6.1f M3: %-6.1f</code>\n",
    d.VIXSpot, d.VIXM1, d.VIXM2, d.VIXM3))
if d.VIXFullSlope != 0 {
    slopeIcon := "📈"
    if d.VIXFullSlope < 0 { slopeIcon = "⚠️" }
    b.WriteString(fmt.Sprintf("<code>  Full Slope (M3/M1): %s%+.1f%%  Cal M2→M3: %+.2f</code>\n",
        slopeIcon, d.VIXFullSlope, d.VIXCalM2M3))
}
if d.VIXFullContango {
    b.WriteString("<code>  Structure: ✅ Full Contango (risk-on normal)</code>\n")
} else if d.VIXFullBackwardation {
    b.WriteString("<code>  Structure: 🔴 Full Backwardation (extreme fear)</code>\n")
}
```

## File yang Harus Diubah

1. `internal/service/vix/types.go` — tambah 4 fields baru
2. `internal/service/vix/fetcher.go` — hitung fields baru setelah M3 ter-set
3. `internal/service/sentiment/sentiment.go` — tambah 4 fields ke SentimentData + copy
4. `internal/adapter/telegram/formatter.go` — update VIX term structure display ke 4-point

## Verifikasi

```bash
go build ./...
# Manual: /sentiment → cek M3 muncul di VIX Term Structure section
# Cek: Full Slope (M3/M1-1) dan Cal M2→M3 tampil
# Edge case: jika M3 = 0 (data unavailable), bagian Full Slope tidak muncul
```

## Acceptance Criteria

- [ ] `VIXTermStructure.FullSlopePct` dihitung dengan benar: `(M3-M1)/M1*100`
- [ ] `SentimentData.VIXM3` populated jika M3 tersedia
- [ ] `/sentiment` menampilkan 4-point term structure: Spot, M1, M2, M3
- [ ] Full Slope dan Cal M2→M3 ditampilkan
- [ ] Gracefully skip M3 section jika M3 = 0
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-27-feature-index-gaps-carry-gjrgarch-oi4w-hmm4-vix-putaran19.md` — GAP 5
- `internal/service/vix/fetcher.go:208-219` — M3 di-fetch tapi tidak digunakan
- `internal/service/vix/types.go:11` — M3 field (dead field)
- `internal/service/sentiment/sentiment.go:155-163` — VIXTermStructure fields
