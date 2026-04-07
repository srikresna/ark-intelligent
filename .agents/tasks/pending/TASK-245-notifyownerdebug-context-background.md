# TASK-245: BUG-001 — handler.go: `notifyOwnerDebug` goroutine harus gunakan `context.Background()`

**Priority:** medium
**Type:** bugfix
**Estimated:** XS
**Area:** internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 23:00 WIB

## Deskripsi

Fungsi `notifyOwnerDebug` (baris 2585) menjalankan `SendHTML` di dalam goroutine tetapi menangkap `ctx` dari caller (Telegram request context). Jika koneksi Telegram sudah closed atau request-timeout sebelum goroutine berjalan, `SendHTML` akan gagal dengan `context canceled` secara silent — pesan debug ke owner tidak pernah terkirim.

`chat_service.go:notifyOwner` (baris 298) sudah benar menggunakan `context.Background()` dengan komentar eksplisit:
```go
// Non-blocking — fires in a goroutine with a detached context so the
// notification survives even if the request context is cancelled.
func (cs *ChatService) notifyOwner(_ context.Context, html string) {
    go cs.ownerNotify(context.Background(), html)
}
```

`notifyOwnerDebug` harus mengikuti pattern yang sama.

## File yang Harus Diubah

- `internal/adapter/telegram/handler.go`
  - Fungsi `notifyOwnerDebug` (sekitar baris 2585)
  - Ganti `ctx` yang ditangkap goroutine dengan `context.Background()`

## Implementasi

### Sebelum (handler.go:~2590):
```go
func (h *Handler) notifyOwnerDebug(ctx context.Context, html string) {
	ownerID := h.bot.OwnerID()
	if ownerID <= 0 {
		return
	}
	go func() {
		_, _ = h.bot.SendHTML(ctx, fmt.Sprintf("%d", ownerID), html)
	}()
}
```

### Sesudah:
```go
// notifyOwnerDebug sends a debug message to the bot owner (non-blocking, best-effort).
// Uses context.Background() so the notification survives even if the request context
// is cancelled before the goroutine fires (e.g. Telegram timeout, user disconnect).
// Does nothing if OwnerID is not set.
func (h *Handler) notifyOwnerDebug(_ context.Context, html string) {
	ownerID := h.bot.OwnerID()
	if ownerID <= 0 {
		return
	}
	go func() {
		_, _ = h.bot.SendHTML(context.Background(), fmt.Sprintf("%d", ownerID), html)
	}()
}
```

## Acceptance Criteria

- [x] Goroutine di `notifyOwnerDebug` menggunakan `context.Background()` bukan `ctx` dari parameter
- [x] Parameter `ctx` diganti `_ context.Context` (underscore) atau dipertahankan tapi tidak dipakai ke goroutine
- [x] Tidak ada perubahan behavior lain
- [x] `go build ./...` sukses

## Implementation

- **PR**: [#370](https://github.com/arkcode369/ark-intelligent/pull/370)
- **Branch**: `feat/TASK-245-notifyownerdebug-context`
- **Commit**: `d1fa038`
- **Status**: In Review

## Referensi

- `.agents/research/2026-04-02-23-bug-hunt-putaran11.md` — BUG-001
- `internal/service/ai/chat_service.go:298` — contoh pattern benar
- `internal/adapter/telegram/handler.go:683` — call site utama
