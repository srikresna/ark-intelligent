# TASK-195: Callback Handler — Guard Empty chatID When Message Nil

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/telegram/

## Deskripsi

Add validation untuk empty chatID di callback dispatch. Saat cb.Message nil, chatID jadi "" → handlers fail silently. Need early return with AnswerCallback error toast.

## Bug Detail

```go
// bot.go:345-351
chatID := ""
if cb.Message != nil {
    chatID = strconv.FormatInt(cb.Message.Chat.ID, 10)
}
// chatID="" → handlers fail at Telegram API
```

## Fix

```go
if chatID == "" {
    log.Warn().Str("data", cb.Data).Msg("callback with nil message")
    _ = b.AnswerCallback(ctx, cb.ID, "Session expired. Please resend command.")
    return
}
```

## File Changes

- `internal/adapter/telegram/bot.go` — Add chatID="" check after extraction, return with toast

## Acceptance Criteria

- [ ] Empty chatID detected and short-circuited
- [ ] User sees "Session expired" toast
- [ ] Log warning for debugging
- [ ] No panic or silent failure
- [ ] Existing callbacks with valid message unaffected
