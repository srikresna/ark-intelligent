# TASK-050: Share/Forward Feature — Tombol 📤 + Clean Text Generator

**Priority:** medium
**Type:** feature
**Estimated:** M (3-4 jam)
**Area:** internal/adapter/telegram/keyboard.go, internal/adapter/telegram/handler.go, internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-01 18:00 WIB
**Siklus:** UX-1 (Siklus 1 Sesi 2)

## Deskripsi

User sering screenshot atau forward analysis secara manual, tapi format HTML Telegram (`<b>`, `<code>`) tidak render bagus di luar Telegram.
UX_AUDIT TASK-UX-012 merekomendasikan tombol "📤 Share" yang generate clean text version.

## Solution

### 1. Tambah callback `share:<type>:<key>` di keyboard.go

Untuk COT detail keyboard:
```go
{Text: "📤 Share", CallbackData: fmt.Sprintf("share:cot:%s", contractCode)},
```

Untuk Outlook keyboard:
```go
{Text: "📤 Share", CallbackData: "share:outlook:latest"},
```

### 2. Tambah `FormatCOTShareText()` di formatter.go

Plain text tanpa HTML tags, copypaste-friendly:
```
📊 COT Report — EUR/USD
Date: 2 Apr 2026

Net Position: +123,456 contracts [BULLISH]
Percentile: 78% (Extreme Long)
Conviction: 8.2/10

⚡ ARK Intelligence Terminal
```

### 3. Handle callback `share:*` di bot.go/handler.go

- Kirim `SendHTML` dengan content dalam `<code>` block (auto-copyable di Telegram)
- Flag pesan sebagai `ephemeral` (hanya untuk user sendiri via answerCallbackQuery)
- Atau: kirim sebagai reply ke message asli

### 4. Apply ke: COT Detail, Outlook, Alpha Summary, CTA Summary

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] Tombol 📤 Share muncul di COT detail keyboard
- [ ] Tap 📤 Share → kirim clean text versi tanpa HTML
- [ ] Clean text bisa di-copy dan di-forward ke grup lain tanpa artefak formatting
- [ ] Tombol Share muncul di Outlook dan Alpha summary

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/keyboard.go`
- `internal/adapter/telegram/formatter.go`
- `internal/adapter/telegram/handler.go` (callback routing `share:*`)
