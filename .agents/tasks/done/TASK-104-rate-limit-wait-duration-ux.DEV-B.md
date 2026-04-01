# TASK-104: Rate Limit UX — Show Wait Duration & Context

**Priority:** medium
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 20:00 WIB
**Siklus:** UX

## Deskripsi
Rate limit feedback saat ini: `"⏳ Rate limited — please wait a moment before sending more commands."` tanpa informasi:
- Berapa lama user harus menunggu
- Apa yang trigger rate limit (command vs callback)
- Sisa quota / cooldown

User cenderung re-send immediately (memperburuk masalah). Juga ada inkonsistensi bahasa (English) vs UI lain (Indonesian).

## Konteks
- `bot.go:360` — command rate limit message
- `bot.go:395` — callback rate limit message
- `middleware.go:228-230` — middleware rate limit response
- Rate limiter implementation kemungkinan punya info tentang remaining cooldown
- Ref: `.agents/research/2026-04-01-20-ux-callback-error-session-polish.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Rate limit message menampilkan estimasi waktu tunggu (jika available dari limiter): `"⏳ Batas request tercapai. Coba lagi dalam ~30 detik."`
- [ ] Bahasa konsisten dengan UI lain (Indonesia sebagai default)
- [ ] Command dan callback rate limit messages unified (sama formatnya)
- [ ] Jika exact duration tidak available, gunakan generic tapi tetap informatif: `"⏳ Batas request tercapai. Tunggu beberapa saat sebelum mencoba lagi."`

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/bot.go` (rate limit message di handleMessage dan handleCallback)
- `internal/adapter/telegram/middleware.go` (rate limit response message)

## Referensi
- `.agents/research/2026-04-01-20-ux-callback-error-session-polish.md`
