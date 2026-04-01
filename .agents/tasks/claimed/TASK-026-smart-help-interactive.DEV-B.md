# TASK-026: Smart /help — Keyboard Interaktif Kategori

**Priority:** HIGH  
**Siklus:** 1 (UX/UI)  
**Effort:** Medium (1 hari)  
**File Utama:** `internal/adapter/telegram/handler.go`, `internal/adapter/telegram/keyboard.go`

---

## Problem

`sendHelp()` saat ini mengirim wall of text (30+ command dalam satu message).  
User baru overwhelmed. User lama tidak ingat exact command.  
Tidak ada pengelompokan yang visual dan interaktif.

## Solution

### 1. Redesign `sendHelp()` — Hanya Tampilkan Kategori
```
🦅 ARK Intelligence Terminal
<i>Pilih kategori untuk melihat commands tersedia:</i>
```

Keyboard:
```
[📊 Market & COT]    [🔬 Research & Alpha]
[🧠 AI & Outlook]   [⚡ Signals & Alerts]
[⚙️ Settings]        [🆕 What's New]
```

### 2. Tambah Callback Handler `help:` prefix
- `help:market` → expand market commands (COT, Rank, Bias, Price, Levels, Calendar)
- `help:research` → Alpha, CTA, Quant, VP, Backtest, Accuracy
- `help:ai` → Outlook, Seasonal, Sentiment, Macro, Impact
- `help:settings` → Settings, Language, Model, Alert preferences
- `help:changelog` → tampilkan changelog (sudah ada field `h.changelog`)

### 3. Format Sub-Category Response
Setiap sub-kategori: list command dengan deskripsi singkat 1 baris + "Back to Help" button.
```
📊 <b>Market & Data Commands</b>

/cot — COT institutional positioning
/rank — Currency strength ranking
/bias — Directional bias summary
/calendar — Economic calendar
/price — Daily OHLC context
/levels — Support/resistance levels

[<< Kembali ke Menu]
```

### 4. Tambah `KeyboardBuilder.HelpCategoryMenu()` dan `HelpSubMenu(category string)`

## Acceptance Criteria
- [ ] `/help` tampilkan kategori keyboard, bukan wall of text
- [ ] Setiap kategori keyboard berfungsi expand ke sub-list
- [ ] Sub-list ada back button ke kategori
- [ ] `help:changelog` menampilkan versi terbaru dari `h.changelog`
- [ ] Admin categories hanya muncul untuk admin
- [ ] Tidak ada regresi pada `/start` (tetap register chatID)

## Notes
- Pertahankan backward compat: `/help market` (args) bisa langsung expand market category
- Changelog sudah di-inject via `NewHandler(..., changelog string, ...)`
