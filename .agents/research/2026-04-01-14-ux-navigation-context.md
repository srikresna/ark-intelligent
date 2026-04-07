# Research Report: UX/UI — Navigation, Context & Help System

**Siklus:** 1 — UX/UI Improvement  
**Tanggal:** 2026-04-01 14:xx WIB  
**Researcher:** Agent Research

---

## Temuan Codebase

### 1. Struktur Handler & Command
- Total commands terdaftar: 37 commands + 9 callback prefixes (handler.go:170-213)
- `/start` dan `/help` sama-sama panggil `sendHelp()` — tidak ada perbedaan UX
- `sendHelp()` hanya teks dump string panjang + `MainMenu()` keyboard
- Tidak ada onboarding step, role selection, atau "getting started" flow

### 2. MainMenu Keyboard (keyboard.go:726)
- MainMenu sudah ada (5 rows, 8 buttons) tetapi **hanya muncul di /help dan /start**
- Command-command lain (COT detail, Alpha, Quant, dsb) tidak append MainMenu/nav bar
- Tidak ada "sticky nav" row universal di setiap response

### 3. UserPrefs (prefs.go)
- Field yang ada: `AlertMinutes`, `AlertImpacts`, `AlertsEnabled`, `Language`, `PreferredModel`, `CalendarFilter`, `CalendarView`
- **TIDAK ADA:** `LastCurrency`, `CompactMode`, `FavoriteCommands`, `DailyBriefingEnabled`
- `CalendarFilter` dan `CalendarView` sudah menyimpan state — pola ini bisa diperluas

### 4. Gap UX yang Belum Ada Task
- **Unified Nav Bar** (nav row di setiap response) — BELUM ADA task
- **Smart /help** dengan kategori interaktif — BELUM ADA task  
- **Command Shortcuts** (`/c` → `/cot`) — BELUM ADA task
- **Context Carry-Over** (simpan `LastCurrency` di prefs) — BELUM ADA task
- **Daily Briefing** command — BELUM ADA task

### 5. Callback Routing Pattern
- Callback prefix `cmd:` → `cbQuickCommand` (handler.go:210) sudah memetakan ke commands
- Pattern: `cmd:macro` → panggil cmdMacro
- Pattern ini bisa dipakai untuk shortcuts di nav bar

### 6. Formatter Output
- Beberapa formatter output tidak konsisten (ditemukan di audit sebelumnya)
- Belum ada template header/footer universal
- WIB timestamp ada di formatter calendar tapi tidak di semua response

### 7. Task TASK-001 s.d TASK-005 Sudah Ada
- TASK-001: interactive-onboarding ✓
- TASK-002: standardisasi-button-labels ✓  
- TASK-003: typing-indicator-feedback ✓
- TASK-004: compact-mode-output ✓
- TASK-005: user-friendly-errors ✓

---

## Peluang UX yang Belum Tertutup

### Peluang A: Sticky Nav Row di Setiap Response
Hampir semua response saat ini tidak ada "escape route" ke menu utama.
Solution: `KeyboardBuilder.NavFooter()` — row 4 button kecil yang di-append ke semua keyboard.

### Peluang B: Smart /help Interaktif
/help sekarang = teks dump. Bisa jadi kategori keyboard: Market, Research, Signals, Settings, Tutorial.

### Peluang C: Context Carry-Over (LastCurrency)
Saat ini `CalendarFilter` dan `CalendarView` sudah disimpan di prefs.
Perpanjang pola: tambah field `LastCurrency string` di `UserPrefs`.
Impact: `/cta` setelah `/cot EUR` bisa otomatis tampilkan EUR.

### Peluang D: Command Shortcuts/Aliases
Go bot handler bisa register alias: `/c` → same handler as `/cot`.
Zero infrastructure cost, langsung produktif.

### Peluang E: Daily Briefing
Command `/briefing` atau auto-push pagi.
Gabungkan: event hari ini + top signal + market context.
Bisa manfaatkan scheduler yang sudah ada.

---

## Rekomendasi Task (Next 5 Tasks)

TASK-025: Sticky nav footer keyboard (HIGH)
TASK-026: Smart /help interaktif dengan kategori (HIGH)
TASK-027: Context carry-over - LastCurrency di UserPrefs (MEDIUM)
TASK-028: Command shortcuts/aliases pendek (MEDIUM)
TASK-029: Daily briefing /briefing command (MEDIUM)
