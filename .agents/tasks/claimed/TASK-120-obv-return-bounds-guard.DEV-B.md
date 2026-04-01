# TASK-120: OBV Return Value Bounds Guard

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/service/ta
**Created by:** Research Agent
**Created at:** 2026-04-02 00:00 WIB
**Siklus:** BugHunt

## Deskripsi
`CalcOBV()` di `indicators.go:818` mengakses `series[0]` sebagai return value tanpa final bounds check. Jika `CalcOBVSeries()` return empty slice (edge case saat data sangat pendek), ini panic.

Berbeda dari TASK-024 yang focus pada OBV series computation — ini focus pada bounds guard sebelum akses return value.

## Konteks
- `indicators.go:818` — `return &OBVResult{Value: series[0], ...}`
- Juga akses: `series[1]`, `series[len(series)-1]`, `series[len(series)-2]` di trend detection (lines 803-809)
- Edge case: pair dengan sangat sedikit historical data, atau API return partial data
- Ref: `.agents/research/2026-04-01-24-bug-hunting-bounds-divzero-goroutine.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Tambah `if len(series) == 0 { return nil }` guard sebelum line 818
- [ ] Tambah bounds check sebelum semua slice indexing di CalcOBV trend detection section
- [ ] Audit CalcOBVSeries juga — pastikan tidak bisa return empty slice tanpa error
- [ ] Tidak ada behavior change untuk normal data (series panjang enough)

## File yang Kemungkinan Diubah
- `internal/service/ta/indicators.go`

## Referensi
- `.agents/research/2026-04-01-24-bug-hunting-bounds-divzero-goroutine.md`
- TASK-024 (related OBV fix, different scope)
