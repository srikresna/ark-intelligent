# TASK-051: Reaction Feedback Buttons — 👍/👎/🔔 di Analysis Messages

**Priority:** low
**Type:** feature
**Estimated:** M (2-3 jam)
**Area:** internal/adapter/telegram/keyboard.go, internal/adapter/telegram/handler.go, internal/domain/
**Created by:** Research Agent
**Created at:** 2026-04-01 18:00 WIB
**Siklus:** UX-1 (Siklus 1 Sesi 2)

## Deskripsi

Bot tidak punya mekanisme feedback dari user.
UX_AUDIT TASK-UX-011 merekomendasikan reaction buttons di analysis messages.
Data ini berguna untuk:
- Mengetahui command mana paling berguna
- Tuning conviction score weights (jika analysis dinilai tidak helpful, bisa disimpan)
- Membangun personalisasi di masa depan

## Solution

### 1. Tambah `FeedbackRow()` di keyboard.go

```go
// FeedbackRow returns a feedback row for analysis messages.
// callbackBase: e.g. "fb:cot:EUR", "fb:outlook:latest"
func (kb *KeyboardBuilder) FeedbackRow(callbackBase string) []ports.InlineButton {
    return []ports.InlineButton{
        {Text: "👍", CallbackData: callbackBase + ":up"},
        {Text: "👎", CallbackData: callbackBase + ":down"},
        {Text: "🔔 Alert on change", CallbackData: callbackBase + ":alert"},
    }
}
```

### 2. Simpan feedback ke SQLite

Buat tabel `analysis_feedback`:
```sql
CREATE TABLE IF NOT EXISTS analysis_feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    analysis_type TEXT NOT NULL,  -- "cot", "outlook", "alpha"
    analysis_key TEXT NOT NULL,   -- "EUR", "latest", etc.
    rating TEXT NOT NULL,         -- "up", "down"
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 3. Handle callback `fb:*` di handler.go

- Parse `fb:<type>:<key>:<rating>`
- Insert ke tabel feedback
- Answer callback dengan "✅ Feedback diterima!"
- Jika `:alert` → set per-pair alert (link ke TASK-052)

### 4. Apply FeedbackRow ke

- COT Detail (setelah keyboard utama)
- Outlook response
- Alpha summary

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] 👍/👎/🔔 muncul di COT detail, Outlook, Alpha
- [ ] Tap 👍 atau 👎 → answerCallbackQuery "✅ Feedback diterima!" (toast)
- [ ] Feedback tersimpan di SQLite
- [ ] 🔔 Alert on change → redirect ke pair alert setup (atau toast "Fitur segera hadir")

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/keyboard.go`
- `internal/adapter/telegram/handler.go`
- `internal/db.go` atau new `internal/adapter/sqlite/feedback_repo.go`
