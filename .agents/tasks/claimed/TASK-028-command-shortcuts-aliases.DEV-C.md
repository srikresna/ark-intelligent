# TASK-028: Command Shortcuts & Aliases Pendek

**Priority:** MEDIUM  
**Siklus:** 1 (UX/UI)  
**Effort:** Small (2-3 jam)  
**File Utama:** `internal/adapter/telegram/handler.go`

---

## Problem

User mobile perlu ketik `/calendar`, `/outlook`, `/backtest` — panjang.  
Power user ingin shortcut `/cal`, `/out`, `/bt`.  
Bot Go Telegram sudah support multiple command registration dengan handler yang sama.

## Solution

### 1. Register Alias di `NewHandler()`
Tambah setelah registrasi command utama:
```go
// Short aliases for power users
bot.RegisterCommand("/c",   h.cmdCOT)
bot.RegisterCommand("/cal", h.cmdCalendar)
bot.RegisterCommand("/out", h.cmdOutlook)
bot.RegisterCommand("/m",   h.cmdMacro)
bot.RegisterCommand("/q",   h.cmdQuant)
bot.RegisterCommand("/bt",  h.cmdBacktest)
bot.RegisterCommand("/r",   h.cmdRank)
bot.RegisterCommand("/s",   h.cmdSentiment)
```

### 2. Hindari Konflik
- `/c` → check tidak bentrok dengan `/clear` (sudah `/clear`, aman)
- `/m` → macro (tidak ada `/m` sebelumnya)
- `/q` → quant (tidak ada `/q`)
- `/r` → rank (tidak ada `/r`)
- `/s` → sentiment (tidak ada `/s`)
- `/bt` → backtest

### 3. Tampilkan Aliases di Smart /help (TASK-026)
Setiap entry di sub-kategori help:
```
/cot · /c — COT institutional positioning
```

### 4. Update Changelog & /help Text
Tambah section "Quick Shortcuts" di help text.

## Acceptance Criteria
- [ ] `/c EUR` berfungsi sama dengan `/cot EUR`
- [ ] `/cal week` berfungsi sama dengan `/calendar week`
- [ ] `/out` berfungsi sama dengan `/outlook`
- [ ] `/m` berfungsi sama dengan `/macro`
- [ ] `/q EUR` berfungsi sama dengan `/quant EUR`
- [ ] `/bt` berfungsi sama dengan `/backtest`
- [ ] Tidak ada konflik dengan command yang sudah ada
- [ ] Update log count di handler.go (saat ini: "registered commands and callbacks")

## Notes
- Zero business logic change — pure alias registration
- Telegram BotFather command list tidak perlu diupdate (aliases tersembunyi dari menu)
- Bisa ditest langsung di bot tanpa rebuild infra
