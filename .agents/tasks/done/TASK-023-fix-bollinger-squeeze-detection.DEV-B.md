# TASK-023: Fix Bollinger Squeeze detection yang hampir selalu false

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/service/ta
**Created by:** Research Agent
**Created at:** 2026-04-01 14:00 WIB
**Siklus:** BugHunt

## Deskripsi
Di `CalcBollinger` (`internal/service/ta/indicators.go:490-503`), squeeze detection bermasalah:

```go
for i := 0; i < len(upper) && i < period; i++ {
    // upper/middle/lower adalah newest-first
    // upper[0] = most recent (valid), upper[n-1] = oldest (bisa NaN)
    if !math.IsNaN(upper[i]) && ... {
        bwSeries = append(bwSeries, ...)
    }
}
if len(bwSeries) >= period { // hampir selalu false!
    squeeze = ...
}
```

Loop mengambil dari index 0 sampai `period` dari slice newest-first. Nilai valid (non-NaN) ada di dekat index 0, sehingga harusnya bisa mengumpulkan `period` entries. Tapi kondisi `len(bwSeries) >= period` mungkin perlu di-review: apakah seluruh `period` bar terbaru valid? Perlu verifikasi apakah ada off-by-one atau kondisi edge case yang membuat squeeze tidak pernah terdeteksi.

Dugaan utama: kalkulasi `bwSeries` mengumpulkan entries dari newest-first slice, dan jika slice lebih pendek dari `period` (jarang punya cukup valid bars), squeeze selalu false.

## Konteks
Squeeze detection adalah sinyal penting untuk anticipate volatility expansion. Jika selalu return false, fitur ini tidak berfungsi sama sekali.

Ref: `.agents/research/2026-04-01-14-bug-hunting-edge-cases.md` (BUG-A5)

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Unit test: buat test case dengan data squeeze yang jelas, pastikan `Squeeze: true` returned
- [ ] Unit test: buat test case normal, pastikan `Squeeze: false` returned
- [ ] Logic squeeze detection diperbaiki agar berfungsi dengan benar

## File yang Kemungkinan Diubah
- `internal/service/ta/indicators.go`
- `internal/service/ta/ta_test.go` (tambah test)

## Referensi
- `.agents/research/2026-04-01-14-bug-hunting-edge-cases.md` (BUG-A5)
