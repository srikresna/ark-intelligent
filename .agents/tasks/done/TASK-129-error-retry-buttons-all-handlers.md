# TASK-129: Error Retry Buttons di Semua Command Handlers

**Priority:** medium
**Type:** ux
**Estimated:** M
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 01:00 WIB
**Siklus:** UX

## Deskripsi
Saat error terjadi (API failure, timeout, no data), user hanya lihat teks error tanpa aksi. User harus type ulang command manual. Tambahkan inline keyboard dengan retry button di semua error responses.

## Konteks
- `handler_wyckoff.go:96-104` — Error tanpa retry button
- `handler_gex.go:88-92` — "Please try again" tapi harus type manual
- Pattern ini tersebar di semua handler — cek handler_cta.go, handler_quant.go, handler_alpha.go, dll
- Mobile UX sangat buruk tanpa retry button
- Ref: `.agents/research/2026-04-02-01-ux-new-features-wyckoff-gex-discovery.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat helper function `errorKeyboard(command, args string) InlineKeyboard` di keyboard.go:
  - Button: [🔄 Coba Lagi] — callback re-executes the same command with same args
  - Button: [🏠 Menu] — navigate to /help
- [ ] Apply errorKeyboard di semua error paths di handlers:
  - handler_wyckoff.go
  - handler_gex.go
  - handler_cta.go
  - handler_quant.go
  - handler_alpha.go
  - handler_vp.go
  - (audit semua handler_*.go)
- [ ] Callback handler untuk retry: extract command + args dari callback data, re-execute
- [ ] Rate limit: jangan allow retry spam — max 3 retries per error within 1 minute

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/keyboard.go` (errorKeyboard helper)
- `internal/adapter/telegram/handler_wyckoff.go`
- `internal/adapter/telegram/handler_gex.go`
- `internal/adapter/telegram/handler_cta.go`
- `internal/adapter/telegram/handler_quant.go`
- `internal/adapter/telegram/handler_alpha.go`
- `internal/adapter/telegram/handler_vp.go`
- `internal/adapter/telegram/bot.go` (retry callback routing)

## Referensi
- `.agents/research/2026-04-02-01-ux-new-features-wyckoff-gex-discovery.md`
- `.agents/UX_AUDIT.md#5`
