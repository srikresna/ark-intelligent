# TASK-275: Fix BUG-UX1 — nav:cot Tidak Di-handle di cbNav

**Priority:** high
**Type:** bugfix
**Estimated:** XS
**Area:** internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 25:00 WIB

## Deskripsi

`cbNav` hanya menangani action `"home"`. Action `"cot"` jatuh ke `default: return nil` — silent no-op.

Keyboard buttons dengan `CallbackData: "nav:cot"` ada di 3 tempat:
- `MainMenu()` — "📊 COT Analysis" (keyboard.go:766)
- `StarterKitMenu("beginner")` — "📊 COT (Posisi Big Player)" (keyboard.go:1293)
- `StarterKitMenu("intermediate")` — "📊 COT Analysis" (keyboard.go:1309)

User menekan tombol COT dari menu utama → tidak terjadi apa-apa. Tidak ada toast, tidak ada navigasi. Tombol sepenuhnya mati.

## File yang Harus Diubah

### internal/adapter/telegram/handler.go — cbNav (sekitar line 1643)

**Sebelum:**
```go
func (h *Handler) cbNav(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
    action := strings.TrimPrefix(data, "nav:")
    switch action {
    case "home":
        _ = h.bot.DeleteMessage(ctx, chatID, msgID)
        return h.cmdStart(ctx, chatID, userID, "")
    default:
        return nil
    }
}
```

**Sesudah:**
```go
func (h *Handler) cbNav(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
    action := strings.TrimPrefix(data, "nav:")
    switch action {
    case "home":
        _ = h.bot.DeleteMessage(ctx, chatID, msgID)
        return h.cmdStart(ctx, chatID, userID, "")
    case "cot":
        _ = h.bot.DeleteMessage(ctx, chatID, msgID)
        return h.cmdCOT(ctx, chatID, userID, "")
    default:
        log.Warn().Str("action", action).Str("chat_id", chatID).Msg("unhandled nav action")
        return nil
    }
}
```

## Verifikasi

```bash
go build ./...
go test ./internal/adapter/telegram/...
```

Lalu test manual: `/start` → tekan "📊 COT Analysis" → harus buka COT overview.

## Acceptance Criteria

- [ ] Tombol "📊 COT Analysis" di MainMenu menjalankan `/cot`
- [ ] Tombol "📊 COT (Posisi Big Player)" di StarterKitMenu beginner menjalankan `/cot`
- [ ] Tombol "📊 COT Analysis" di StarterKitMenu intermediate menjalankan `/cot`
- [ ] `go build ./...` clean
- [ ] `default` case log warning (tidak silent lagi)

## Referensi

- `.agents/research/2026-04-02-25-ux-audit-dead-callbacks-loading-patterns-putaran17.md` — BUG-UX1
- `internal/adapter/telegram/handler.go` — cbNav function (~line 1641)
- `internal/adapter/telegram/keyboard.go` — nav:cot usages (lines 766, 1293, 1309)
