# TASK-179: Page X/Y Indicator + Error Retry Inline Buttons

**Priority:** medium
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram/

## Deskripsi

Dua quick wins dalam satu task:
1. Tambah "Page X/Y" footer di chunked messages
2. Tambah "Retry" inline button di error messages

## Part 1: Page Indicator

```go
// api.go SendWithKeyboardChunked
for i, chunk := range chunks {
    footer := fmt.Sprintf("\n\n--- Page %d/%d ---", i+1, len(chunks))
    h.bot.SendHTML(ctx, chatID, chunk + footer)
}
```

## Part 2: Error Retry Button

```go
// errors.go sendUserError
keyboard := InlineKeyboardMarkup{
    InlineKeyboard: [][]InlineKeyboardButton{{
        {Text: "🔄 Retry", CallbackData: "retry:" + originalCommand},
    }},
}
h.bot.SendHTMLWithKeyboard(ctx, chatID, errorMsg, keyboard)
```

## File Changes

- `internal/adapter/telegram/api.go` — Add page footer to chunked messages
- `internal/adapter/telegram/errors.go` — Add retry keyboard to error messages
- `internal/adapter/telegram/handler.go` — Handle "retry:" callback → re-execute command

## Acceptance Criteria

- [ ] Chunked messages show "Page X/Y" footer
- [ ] Error messages have "Retry" inline button
- [ ] Retry button re-executes original command with same args
- [ ] Retry button only shown for retriable errors (timeout, rate limit, server error)
- [ ] Non-retriable errors (invalid args, unauthorized) don't show retry
