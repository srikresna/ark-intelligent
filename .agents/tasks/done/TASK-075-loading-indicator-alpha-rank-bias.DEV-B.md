# TASK-075: Loading Indicator untuk /alpha, /rank, /bias + Telegram ChatAction

**Priority:** high
**Type:** ux-improvement
**Estimated:** S (2 jam)
**Area:** internal/adapter/telegram/handler.go, handler_alpha.go, bot.go
**Created by:** Research Agent
**Created at:** 2026-04-01 19:00 WIB
**Siklus:** UX-1 (Siklus 1 Putaran 3)
**Ref:** UX_AUDIT.md TASK-UX-004

## Deskripsi

`cmdAlpha`, `cmdRank`, `cmdBias` tidak memiliki loading indicator sebelum komputasi data.
User tidak mendapat feedback saat perintah ini diproses (bisa 3-10 detik).
Handler lain seperti quant dan vp sudah menggunakan pattern yang benar.

## Gap

```go
// handler_alpha.go — SEKARANG (tidak ada loading):
func (h *Handler) cmdAlpha(...) error {
    state, err := h.computeAlphaState(ctx) // bisa 5-10 detik, tanpa feedback
    ...
}

// handler.go cmdRank — SEKARANG:
func (h *Handler) cmdRank(...) error {
    analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx) // langsung fetch
    ...
}
```

## Implementasi

### 1. Tambah helper SendChatAction di bot.go
```go
// SendChatAction sends a typing/upload indicator to the chat.
func (b *Bot) SendChatAction(ctx context.Context, chatID string, action string) error {
    payload := map[string]interface{}{
        "chat_id": chatID,
        "action":  action, // "typing", "upload_photo", "upload_document"
    }
    return b.postJSON(ctx, "sendChatAction", payload, nil)
}
```

### 2. Fix cmdAlpha (handler_alpha.go)
```go
func (h *Handler) cmdAlpha(...) error {
    if h.alpha == nil {
        _, err := h.bot.SendHTML(ctx, chatID, "⚙️ Alpha Engine not configured.")
        return err
    }
    loadingID, _ := h.bot.SendHTML(ctx, chatID, "⚡ Computing Alpha Engine analysis... ⏳")
    state, err := h.computeAlphaState(ctx)
    if loadingID > 0 {
        _ = h.bot.DeleteMessage(ctx, chatID, loadingID)
    }
    if err != nil { ... }
    ...
}
```

### 3. Fix cmdRank (handler.go)
```go
func (h *Handler) cmdRank(...) error {
    loadingID, _ := h.bot.SendHTML(ctx, chatID, "📈 Computing currency strength ranking... ⏳")
    analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
    // ... (rest of logic)
    if loadingID > 0 {
        _ = h.bot.DeleteMessage(ctx, chatID, loadingID)
    }
    ...
}
```

### 4. Fix cmdBias (handler.go)
```go
func (h *Handler) cmdBias(...) error {
    loadingID, _ := h.bot.SendHTML(ctx, chatID, "🎯 Detecting directional bias... ⏳")
    // ... existing logic
    if loadingID > 0 {
        _ = h.bot.DeleteMessage(ctx, chatID, loadingID)
    }
    ...
}
```

### 5. Gunakan SendChatAction di cmdOutlook
Tambah `_ = h.bot.SendChatAction(ctx, chatID, "typing")` sebelum AI generation.

## Acceptance Criteria
- [ ] `cmdAlpha` mengirim ⏳ loading message, lalu hapus setelah selesai
- [ ] `cmdRank` mengirim ⏳ loading message
- [ ] `cmdBias` mengirim ⏳ loading message
- [ ] `bot.go` punya method `SendChatAction`
- [ ] `go build ./...` clean
