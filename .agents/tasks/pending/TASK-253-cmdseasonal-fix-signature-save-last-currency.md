# TASK-253: cmdSeasonal — Fix Signature untuk Gunakan userID + saveLastCurrency

**Priority:** low
**Type:** ux
**Estimated:** XS
**Area:** internal/adapter/telegram/handler_seasonal.go
**Created by:** Research Agent
**Created at:** 2026-04-02 04:00 WIB

## Deskripsi

`cmdSeasonal` (handler_seasonal.go:18) menggunakan `_ int64` — membuang parameter `userID`. Interface `CommandHandler` (bot.go:121) sudah support userID, tapi handler seasonal tidak menggunakannya.

Akibatnya:
1. `saveLastCurrency` tidak bisa dipanggil setelah user lihat seasonal EUR
2. User yang pakai `/seasonal` tidak mendapat manfaat context carry-over

Bandingkan dengan cmdPrice dan cmdLevels yang sudah memiliki `userID int64` dalam signature (meskipun belum memanggil saveLastCurrency — itu dicakup TASK-252).

## File yang Harus Diubah

- `internal/adapter/telegram/handler_seasonal.go`

## Implementasi

### Sebelum (handler_seasonal.go:18):

```go
func (h *Handler) cmdSeasonal(ctx context.Context, chatID string, _ int64, args string) error {
```

### Sesudah:

```go
func (h *Handler) cmdSeasonal(ctx context.Context, chatID string, userID int64, args string) error {
```

Lalu, setelah currency berhasil diidentifikasi dan analisis berhasil dijalankan (sebelum return nil pada success path), tambahkan:

```go
h.saveLastCurrency(ctx, userID, currency)
```

Identifikasi exact line dimana currency sudah di-parse dan analysis berhasil — tambahkan saveLastCurrency call di sana.

## Acceptance Criteria

- [ ] `cmdSeasonal` signature menggunakan `userID int64` (bukan `_ int64`)
- [ ] `/seasonal EUR` → `prefs.LastCurrency` tersimpan sebagai "EUR"
- [ ] Setelah `/seasonal GBP`, ketik `/cot` → LastCurrency "GBP" tersedia
- [ ] Tidak ada perubahan pada currency selector behavior (tanpa args)
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-04-ux-audit-navigation-context-settings-putaran12.md` — Temuan 3
- `handler_seasonal.go:18` — signature yang perlu difix
- `bot.go:121` — CommandHandler type signature
- `handler.go:1765` — saveLastCurrency() helper
- `TASK-252` — task serupa untuk cmdPrice + cmdLevels
