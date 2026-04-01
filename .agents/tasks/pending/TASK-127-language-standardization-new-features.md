# TASK-127: Language Standardization di Fitur Baru

**Priority:** high
**Type:** ux
**Estimated:** M
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 01:00 WIB
**Siklus:** UX

## Deskripsi
Fitur baru (Wyckoff, GEX, Sentiment formatters) punya bahasa yang inkonsisten. Wyckoff pakai Indonesian errors + English content, GEX full English, Sentiment campur. Standardize ke satu bahasa (Indonesian default, sesuai bot standard).

## Konteks
- Wyckoff: `handler_wyckoff.go:49` — "⚠️ Wyckoff engine tidak tersedia" (ID)
- GEX: `handler_gex.go:52` — "⚠️ GEX engine is not configured" (EN)
- GEX: `handler_gex.go:69` — "Symbol not supported" (EN)
- Sentiment: `formatter.go:3728-3734` — "Penurunan tajam" (ID) + "Contrarian BUY" (EN)
- Bot standard = Indonesian default dengan toggle ke English via /prefs
- Ref: `.agents/research/2026-04-02-01-ux-new-features-wyckoff-gex-discovery.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Audit semua string literals di:
  - `handler_wyckoff.go` — pastikan semua user-facing text Indonesian
  - `handler_gex.go` — convert English errors ke Indonesian
  - `formatter_wyckoff.go` — user-facing labels Indonesian
  - `formatter_gex.go` — user-facing labels Indonesian
  - `formatter.go` (sentiment section 3591-3800) — standardize
- [ ] Technical terms boleh tetap English (e.g., "GEX", "Wyckoff", "Max Pain") tapi penjelasan dalam Indonesian
- [ ] Error messages unified format: "⚠️ <b>[Feature]</b> tidak tersedia/gagal. [Suggestion]."
- [ ] Cek apakah language preference dari /prefs sudah di-respect di fitur baru (likely belum)

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler_wyckoff.go`
- `internal/adapter/telegram/handler_gex.go`
- `internal/adapter/telegram/formatter_wyckoff.go`
- `internal/adapter/telegram/formatter_gex.go`
- `internal/adapter/telegram/formatter.go` (sentiment section)

## Referensi
- `.agents/research/2026-04-02-01-ux-new-features-wyckoff-gex-discovery.md`
- `.agents/UX_AUDIT.md#5` (Standardize Language)
