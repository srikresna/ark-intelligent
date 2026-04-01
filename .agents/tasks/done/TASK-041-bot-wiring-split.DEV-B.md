# TASK-041: bot.go Split — Polling / Wiring / API Layer

**Type:** refactor  
**Priority:** HIGH  
**Effort:** M (3-4h)  
**Phase:** Tech Refactor Phase 3 (TECH-004)  
**Assignee:** unassigned

---

## Problem

`internal/adapter/telegram/bot.go` adalah 1,289 LOC yang mencampur:
1. **Telegram polling loop** — long-polling getUpdates, dispatch ke handler
2. **Dependency injection / wiring** — konstruksi semua dependencies
3. **Telegram API wrapper** — `apiCall()`, `sendMessage()`, `editMessage()`, dll
4. **Type-unsafe API params** — 15+ lokasi menggunakan `map[string]interface{}` untuk Telegram params

## Solution

### Split menjadi 3 file:
```
internal/adapter/telegram/
├── bot.go          ← ONLY: polling loop + update dispatch + struct Bot definition
├── wiring.go       ← ONLY: NewBot() konstruktor + dependency injection
└── api.go          ← ONLY: apiCall(), sendMessage(), editMessage(), deleteMessage(), dll
```

### Typed params (optional, dapat jadi subtask):
```go
// Ganti map[string]interface{} dengan struct:
type sendMessageParams struct {
    ChatID    string `json:"chat_id"`
    Text      string `json:"text"`
    ParseMode string `json:"parse_mode,omitempty"`
    ReplyMarkup interface{} `json:"reply_markup,omitempty"`
}
```

## Acceptance Criteria
- [ ] `bot.go` < 400 LOC setelah split
- [ ] `wiring.go` berisi semua konstruksi dependency
- [ ] `api.go` berisi semua Telegram API wrapper functions
- [ ] `go build ./...` sukses
- [ ] `go test ./...` tidak ada test yang break
- [ ] Polling behavior identik (no behavior change)

## Notes
- File `bot.go` yang ada saat ini ada `time.Sleep(5s)` di reconnect loop dan `time.Sleep(35ms)` di rate limiter — pertahankan behavior ini
- Typed params bisa dikerjakan sebagai sub-task terpisah (jangan gabungkan dalam satu PR)
- Priority split adalah LEBIH TINGGI dari typed params
