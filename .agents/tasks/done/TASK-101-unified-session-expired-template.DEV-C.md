# TASK-101: Unified Session Expired Template Across All Handlers

**Priority:** high
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 20:00 WIB
**Siklus:** UX

## Deskripsi
Setiap handler punya format session expired yang berbeda:
- CTA: `"⏳ Data expired. Gunakan /cta untuk refresh."`
- ICT: `"⏰ Session expired. Gunakan /ict lagi."`
- Quant: `"⏰ Session expired. Gunakan <code>/quant</code> lagi."`

Emoji berbeda (⏳ vs ⏰), bahasa campuran, format HTML berbeda. Buat satu template function yang digunakan semua handler.

## Konteks
- `handler_cta.go:226-231` — CTA session expired
- `handler_ict.go:247-250` — ICT session expired
- `handler_quant.go:258-261` — Quant session expired
- Kemungkinan handler lain juga punya pattern serupa
- Ref: `.agents/research/2026-04-01-20-ux-callback-error-session-polish.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat helper function `sessionExpiredMessage(command string) string` di errors.go atau helper baru
- [ ] Semua handler (CTA, ICT, Quant, Alpha, dll) menggunakan template yang sama
- [ ] Format unified: emoji konsisten (pilih satu: ⏳), bahasa konsisten (Indonesia), command dalam `<code>` tag
- [ ] Template contoh: `"⏳ <b>Sesi berakhir</b>\n\nData sudah expired. Ketik <code>/cta</code> untuk memulai ulang."`
- [ ] Audit semua handler_*.go untuk pattern "expired" / "session" — pastikan semua diganti

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/errors.go` (tambah sessionExpiredMessage)
- `internal/adapter/telegram/handler_cta.go`
- `internal/adapter/telegram/handler_ict.go`
- `internal/adapter/telegram/handler_quant.go`
- `internal/adapter/telegram/handler_alpha.go` (cek apakah ada)
- Semua handler_*.go yang punya state TTL

## Referensi
- `.agents/research/2026-04-01-20-ux-callback-error-session-polish.md`
- `.agents/UX_AUDIT.md#5`
