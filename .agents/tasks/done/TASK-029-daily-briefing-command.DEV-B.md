# TASK-029: Daily Briefing — Command /briefing

**Priority:** MEDIUM  
**Siklus:** 1 (UX/UI)  
**Effort:** Medium (1-2 hari)  
**File Utama:** `internal/adapter/telegram/handler.go`, `internal/adapter/telegram/formatter.go`, `internal/scheduler/`

---

## Problem

User trader harus run banyak command setiap pagi untuk dapat gambaran market.  
Tidak ada satu command yang aggregate: calendar hari ini + top signals + bias + market context.  
UX_AUDIT.md menyebut ini sebagai TASK-UX-009: Daily Briefing.

## Solution

### 1. Register `/briefing` command
```go
bot.RegisterCommand("/briefing", h.cmdBriefing)
bot.RegisterCommand("/br", h.cmdBriefing) // alias
```

### 2. Implementasi `cmdBriefing`
Aggregate data dari services yang sudah ada:
```go
func (h *Handler) cmdBriefing(ctx context.Context, chatID string, userID int64, args string) error {
    // 1. Economic events hari ini (filter High + Medium impact)
    // 2. Top 3 COT conviction scores
    // 3. Currency bias summary (dari /bias)
    // 4. Alpha playbook highlight jika tersedia
    // Return: ringkas, max 15 baris
}
```

### 3. Format Briefing (max 15 baris)
```
🌅 <b>ARK Daily Briefing</b>
<i>📅 Selasa, 1 April 2026 · 06:00 WIB</i>

📅 <b>Events Hari Ini</b>
08:30 • USD NFP ⚠️ HIGH
10:00 • EUR CPI 📊 MEDIUM

🎯 <b>Top 3 COT Signals</b>
🟢 EUR: BULLISH (Conv: 82%)
🔴 JPY: BEARISH (Conv: 75%)  
🟢 GBP: BULLISH (Conv: 68%)

📊 <b>Bias Summary</b>
USD Strong · EUR Neutral · GBP Weak

<code>Updated: 06:00 WIB</code>
```

### 4. Optional: Auto Push Briefing Pagi
Di scheduler, tambah daily briefing task jam 06:00 WIB untuk user dengan `AIReportsEnabled = true`.  
Gunakan infrastruktur push yang sudah ada (scheduler existing).

### 5. `KeyboardBuilder.BriefingMenu()`
```
[🔄 Refresh] [📅 Calendar] [📊 COT Detail] [🏠 Home]
```

## Acceptance Criteria
- [ ] `/briefing` menampilkan summary pagi (<15 baris)
- [ ] Events hari ini dengan filter High+Medium impact
- [ ] Top 3 COT conviction scores
- [ ] Currency bias 1-liner
- [ ] Refresh button (callback `briefing:refresh`)
- [ ] Auto push pagi jam 06:00 WIB (optional, toggle di settings)
- [ ] `/br` alias berfungsi
- [ ] Test: dapat run tanpa error saat data kosong/belum tersedia

## Notes
- Start dari manual command dulu, auto push bisa jadi follow-up
- Gunakan goroutine dengan timeout untuk aggregate parallel
- Jangan panggil AI outlook (terlalu lambat untuk briefing)
- Jika alpha service nil → skip alpha section, jangan error
