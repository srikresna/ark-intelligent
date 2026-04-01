# TASK-099: Guard Callback Handler Ketika ChatID Kosong

**Priority:** LOW
**Siklus:** 5 (Bug Hunting)
**Estimasi:** 30 menit

## Problem

Di `internal/adapter/telegram/bot.go` — `handleCallback()`:

```go
chatID := ""
if cb.Message != nil {
    chatID = strconv.FormatInt(cb.Message.Chat.ID, 10)
    msgID = cb.Message.MessageID
}
// chatID tetap "" jika cb.Message == nil
// Handler kemudian dipanggil, dan di dalam handler:
// if chatID == "" { chatID = b.defaultID }
// → Balasan dikirim ke OWNER CHAT, bukan user yang klik
```

Ini bisa terjadi ketika:
- User klik inline keyboard dari pesan yang di-forward ke channel
- Pesan sudah terlalu lama (Telegram tidak include message object di callback)

Risikonya: Response yang seharusnya untuk user malah dikirim ke owner chat, berpotensi leak data.

## Solution

```go
func (b *Bot) handleCallback(ctx context.Context, cb *CallbackQuery) {
    chatID := ""
    msgID := 0
    if cb.Message != nil {
        chatID = strconv.FormatInt(cb.Message.Chat.ID, 10)
        msgID = cb.Message.MessageID
    }
    
    // Guard: jika tidak ada target chat, acknowledge dan return
    if chatID == "" {
        log.Warn().Str("data", cb.Data).Int64("user_id", cb.From.ID).Msg("callback without message context, ignoring")
        _ = b.AnswerCallback(ctx, cb.ID, "Session expired. Please use the command again.")
        return
    }
    // ... rest of handler
```

## Acceptance Criteria
- [ ] Callback dengan nil Message dijawab dengan friendly message
- [ ] Tidak ada data yang di-forward ke defaultID tanpa context
- [ ] Log warning untuk debugging
- [ ] `go build ./...` clean

## Files to Modify
- `internal/adapter/telegram/bot.go`

## Resolution

Task sudah diimplementasikan via TASK-195 (feat/TASK-195-callback-nil-chatid-guard, PR #130). Guard chatID == "" sudah ada di bot.go lines 385-391. Task ini duplicate — marked as done.
