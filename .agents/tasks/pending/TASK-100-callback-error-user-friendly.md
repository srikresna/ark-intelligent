# TASK-100: Route Callback Errors Through userFriendlyError

**Priority:** high
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 20:00 WIB
**Siklus:** UX

## Deskripsi
Saat ini callback handler errors hanya menampilkan generic `"Error processing request"` via AnswerCallback popup. Padahal `userFriendlyError()` di `errors.go` sudah memetakan error types ke pesan user-friendly — tapi hanya dipakai untuk command errors, bukan callback errors.

Route semua callback error melalui `userFriendlyError()` agar user mendapat feedback yang sama baiknya saat error terjadi di callback (klik button) maupun command.

## Konteks
- `bot.go:401-413` — callback error handling saat ini
- `errors.go:20-67` — userFriendlyError() mapping yang sudah ada
- Callback errors sangat sering terjadi (timeout, rate limit, API failure) dan user tidak tahu penyebabnya
- Ref: `.agents/research/2026-04-01-20-ux-callback-error-session-polish.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Callback errors di `bot.go handleCallback` diproses melalui `userFriendlyError()` atau equivalent
- [ ] AnswerCallback menampilkan pesan singkat yang informatif (max 200 chars, Telegram limit)
- [ ] Jika pesan error terlalu panjang untuk AnswerCallback toast, fallback ke edit message atau send message
- [ ] Error logging tetap ada (jangan hilangkan log.Error)

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/bot.go` (handleCallback method)
- `internal/adapter/telegram/errors.go` (mungkin perlu variant ringkas untuk callback toast)

## Referensi
- `.agents/research/2026-04-01-20-ux-callback-error-session-polish.md`
- `.agents/UX_AUDIT.md`
