# TASK-276: Fix BUG-UX2 — cmd:cta Tidak Di-handle di cbQuickCommand

**Priority:** high
**Type:** bugfix
**Estimated:** XS
**Area:** internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 25:00 WIB

## Deskripsi

`cbQuickCommand` menangani banyak `cmd:` callbacks (bias, macro, rank, calendar, sentiment, dll) tetapi tidak ada `case "cta"`.

Keyboard buttons dengan `CallbackData: "cmd:cta"` ada di 2 tempat:
- `StarterKitMenu("intermediate")` — "📉 CTA Dashboard" (keyboard.go:1313)
- `StarterKitMenu("advanced")` — "📉 CTA + Backtest" (keyboard.go:1337)

User intermediate/advanced yang onboarding via StarterKit menekan tombol CTA Dashboard → tidak terjadi apa-apa. Silent no-op karena `default: return nil`.

## File yang Harus Diubah

### internal/adapter/telegram/handler.go — cbQuickCommand (sekitar line 1593)

Tambah case sebelum `default`:

**Sebelum:**
```go
    case "vp":
        return h.cmdVP(ctx, chatID, userID, args)
    default:
        return nil
    }
```

**Sesudah:**
```go
    case "vp":
        return h.cmdVP(ctx, chatID, userID, args)
    case "cta":
        return h.cmdCTA(ctx, chatID, userID, args)
    default:
        log.Warn().Str("cmd", cmd).Str("chat_id", chatID).Msg("unhandled cmd: callback")
        return nil
    }
```

**Catatan:** Verifikasi nama method yang benar di `handler_cta.go`. Kemungkinan `cmdCTA` atau nama lain — cek dengan `grep "func.*cmdCTA" internal/adapter/telegram/handler_cta.go`.

## Verifikasi

```bash
go build ./...
go test ./internal/adapter/telegram/...
```

Test manual: `/start` → onboarding → pilih intermediate/advanced → tekan "📉 CTA Dashboard" → harus membuka CTA selector.

## Acceptance Criteria

- [ ] Tombol "📉 CTA Dashboard" di StarterKitMenu intermediate menjalankan `/cta`
- [ ] Tombol "📉 CTA + Backtest" di StarterKitMenu advanced menjalankan `/cta`
- [ ] `go build ./...` clean
- [ ] `default` case log warning

## Referensi

- `.agents/research/2026-04-02-25-ux-audit-dead-callbacks-loading-patterns-putaran17.md` — BUG-UX2
- `internal/adapter/telegram/handler.go` — cbQuickCommand (~line 1593)
- `internal/adapter/telegram/keyboard.go` — cmd:cta usages (lines 1313, 1337)
- `internal/adapter/telegram/handler_cta.go` — CTA command handler
