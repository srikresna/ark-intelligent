# TASK-116: chartPath Defer Cleanup di Quant & VP Handlers

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 23:00 WIB
**Siklus:** Refactor

## Deskripsi
Di handler_quant.go dan handler_vp.go, `defer os.Remove()` ada untuk inputPath dan outputPath, tapi TIDAK untuk chartPath (PNG file). Setiap kali subprocess gagal, PNG file tertinggal di /tmp permanent.

## Konteks
- `handler_quant.go:442-443` — defer ada untuk input/output, TIDAK untuk chartPath
- `handler_vp.go:410-411` — sama, chartPath tidak di-cleanup
- Mirip TASK-071 (chartpath leak di handler_cta.go) tapi di file berbeda
- Ref: `.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Tambah `defer os.Remove(chartPath)` di handler_quant.go setelah chartPath dibuat
- [ ] Tambah `defer os.Remove(chartPath)` di handler_vp.go setelah chartPath dibuat
- [ ] Audit semua handler_*.go yang punya subprocess calls — pastikan SEMUA temp files punya defer cleanup
- [ ] Defer harus dipanggil sebelum subprocess execution, bukan setelah

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler_quant.go`
- `internal/adapter/telegram/handler_vp.go`
- Kemungkinan handler_cta.go, handler_ctabt.go (audit)

## Referensi
- `.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`
- TASK-071 (similar fix di handler_cta.go)
