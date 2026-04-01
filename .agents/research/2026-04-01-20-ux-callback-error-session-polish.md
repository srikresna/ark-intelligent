# Research Report: UX Siklus 1 Putaran 2 — Callback Error, Session State & UX Polish

**Tanggal:** 2026-04-01 20:00 WIB
**Fokus:** UX/UI Improvement (Siklus 1, Putaran 2)
**Siklus:** 1/5

---

## Ringkasan

Audit mendalam terhadap codebase Telegram adapter (`internal/adapter/telegram/`) mengungkap beberapa UX gap kritis yang belum di-cover oleh task sebelumnya. Fokus utama: callback error handling, session expiry UX, message chunking, dan konsistensi UI.

---

## Temuan Utama

### 1. CRITICAL: Callback Errors Tidak User-Friendly (bot.go:401-413)

Saat callback handler gagal, user hanya melihat `"Error processing request"` via AnswerCallback popup. Tidak ada detail, tidak ada actionable suggestion. Padahal `userFriendlyError()` di `errors.go` sudah ada dan cukup baik untuk command errors — tapi TIDAK digunakan untuk callback errors.

**Code path:**
```go
if err := handler(ctx, chatID, msgID, userID, cb.Data); err != nil {
    log.Error().Err(err).Str("data", cb.Data).Msg("callback handler error")
    _ = b.AnswerCallback(ctx, cb.ID, "Error processing request")  // Generic!
    return
}
```

**Solusi:** Route semua callback errors melalui `userFriendlyError()`, kirim sebagai `AnswerCallback` toast atau edit message jika diperlukan.

### 2. CRITICAL: Session Expired Messages Inkonsisten (handler_cta.go:226, handler_ict.go:247, handler_quant.go:258)

Setiap handler punya format berbeda untuk session expired:
- CTA: `"⏳ Data expired. Gunakan /cta untuk refresh."`
- ICT: `"⏰ Session expired. Gunakan /ict lagi."`
- Quant: `"⏰ Session expired. Gunakan <code>/quant</code> lagi."`

Emoji berbeda (⏳ vs ⏰), bahasa campuran, format HTML campuran. Perlu unified session expired template.

### 3. HIGH: Settings Toggle Tanpa Konfirmasi (handler.go:1144-1151)

Saat user toggle setting (alerts, language, dll), callback handler edit message tapi kirim empty AnswerCallback:
```go
_ = b.AnswerCallback(ctx, cb.ID, "")
```
User tidak dapat feedback visual/audio bahwa aksi berhasil. Harus kirim toast "✅ Updated" atau "✅ Disimpan".

### 4. HIGH: Message Chunk Overflow IDs Lost (bot.go:460-530)

Saat pesan >4096 chars di-split ke beberapa chunks:
- SendHTML: hanya return ID message terakhir → chunk sebelumnya orphaned
- EditMessage: hanya edit chunk pertama → chunk overflow jadi message baru yang tidak ter-track

Ini menyebabkan chat clutter saat user berulang kali klik button yang memicu edit.

### 5. MEDIUM: Rate Limit UX Tanpa Duration Info (bot.go:360, 395)

Rate limit feedback: `"⏳ Rate limited — please wait a moment..."` tanpa indikasi:
- Berapa lama harus menunggu
- Apa yang trigger rate limit
- Sisa quota

User cenderung re-send immediately (memperburuk masalah).

### 6. MEDIUM: Callback Data 64-byte Limit Tanpa Validasi (keyboard.go)

Telegram callback_data max 64 bytes. Tidak ada validasi di codebase. Untuk symbol/parameter panjang bisa silently truncated. Perlu validation layer.

---

## Gap vs Existing Tasks

| Area | Existing Task | Gap Found |
|---|---|---|
| Callback error UX | TASK-005 (command errors only) | Callback errors NOT routed through userFriendlyError |
| Session expired | TASK-076 (back button language) | Session expired messages NOT standardized |
| Settings confirmation | — | No toast confirmation on toggle |
| Message chunk tracking | — | Overflow IDs lost, causes clutter |
| Rate limit duration | — | No wait time shown to user |

---

## Task Recommendations

1. **TASK-100**: Callback error routing melalui userFriendlyError [HIGH]
2. **TASK-101**: Unified session expired template semua handler [HIGH]
3. **TASK-102**: Settings toggle AnswerCallback confirmation toast [MEDIUM]
4. **TASK-103**: Message chunk ID tracking untuk multi-part responses [MEDIUM]
5. **TASK-104**: Rate limit UX — tampilkan wait duration & context [MEDIUM]

---

## File yang Dianalisis

- `internal/adapter/telegram/bot.go` (1,289 LOC)
- `internal/adapter/telegram/handler.go` (2,381 LOC)
- `internal/adapter/telegram/handler_cta.go` (1,618 LOC)
- `internal/adapter/telegram/handler_ict.go`
- `internal/adapter/telegram/handler_quant.go`
- `internal/adapter/telegram/handler_alpha.go`
- `internal/adapter/telegram/keyboard.go` (1,170 LOC)
- `internal/adapter/telegram/formatter.go` (4,489 LOC)
- `internal/adapter/telegram/errors.go`
- `internal/adapter/telegram/middleware.go`
