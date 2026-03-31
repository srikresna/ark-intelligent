# Research Report — Siklus 1: UX/UI Improvement (Putaran 3)

**Date:** 2026-04-01 19:00 WIB  
**Agent:** Research  
**Focus:** UX gap yang belum tertangani dari UX_AUDIT.md (post TASK-050-054)

---

## Konteks

Siklus 1 sebelumnya sudah mencakup:
- TASK-001: interactive-onboarding
- TASK-025: sticky-nav-footer
- TASK-026: smart-help-interactive
- TASK-027: context-carryover-lastcurrency
- TASK-028: command-shortcuts-aliases
- TASK-029: daily-briefing-command
- TASK-050: share-forward-feature
- TASK-051: reaction-feedback-buttons
- TASK-052: smart-alerts-per-pair
- TASK-053: history-command-cot-4week
- TASK-054: timestamp-format-standardization

Putaran ini fokus pada **4 UX gap yang masih terbuka** dari UX_AUDIT.md + temuan codebase baru.

---

## Temuan Baru

### 1. Missing Loading Indicator di /alpha, /rank, /bias (UX-004 Partial)
**File:** `handler.go`, `handler_alpha.go`

`cmdAlpha` langsung memanggil `computeAlphaState()` tanpa loading message:
```go
func (h *Handler) cmdAlpha(...) error {
    state, err := h.computeAlphaState(ctx) // LANGSUNG — bisa 5-10 detik
    ...
}
```

`cmdRank` langsung fetch FRED + price ctx tanpa feedback ke user.
`cmdBias` langsung fetch COT history + risk context tanpa feedback.

Pembanding: `handler_quant.go:152` dan `handler_vp.go:133` sudah ada ⏳.
Pattern terbaik: `SendHTML("⏳...")` → compute → `DeleteMessage`.

Tidak ada `SendChatAction("typing")` di mana pun. Telegram API mendukung ini
untuk menunjukkan "typing..." di chat.

**Gap:** /alpha, /rank, /bias tidak ada loading feedback → user tidak tahu bot hang atau proses.

---

### 2. Language Inconsistency di Back Buttons (UX-005)
**File:** `keyboard.go`

Audit semua back button labels:
```
L190: "<< Kembali ke Ringkasan"    (macro)
L206: "<< Ringkasan"               (macro)
L443: "<< Back to Overview"        (cot)
L538: "<< Back to Categories"      (impact)
L680: "<< Grid Overview"           (seasonal)
L720: "<< Back to Overview"        (cot)
L783: "<< Kembali ke Ringkasan"    (alpha)
L848: "<< Kembali ke Ringkasan"    (cta)
L1006: "<< Kembali ke Dashboard"   (quant)
L1057: "<< Kembali ke Dashboard"   (vp)
```

Masalah:
- Mix Bahasa Indonesia dan Inggris: "Back to Overview" vs "Kembali ke Ringkasan"
- Tidak konsisten dalam context yang sama: "Kembali ke Ringkasan" vs "Ringkasan" (macro)
- Tidak ada unified standard

Rekomendasi: Standardisasi ke Bahasa Indonesia (sesuai `domain/prefs.go` default lang "id"):
- `"◀ Ringkasan"` untuk semua "back to summary"
- `"◀ Dashboard"` untuk back ke dashboard level
- `"◀ Kategori"` untuk back to categories
- Hilangkan "<<" ganti dengan "◀" (lebih clean)

---

### 3. Compact Mode — Belum Ada di Settings (UX-010)
**File:** `domain/prefs.go`, `handler.go` cmdSettings

`UserPrefs` tidak punya field `OutputMode`:
```go
type UserPrefs struct {
    // ... ada: Language, CalendarFilter, CalendarView, ClaudeModel
    // TIDAK ADA: OutputMode / CompactMode
}
```

Saat ini semua output full detail, tidak ada toggle untuk compact/minimal.
COT detail bisa sangat panjang (20+ baris per currency).
Quant output bisa 30+ baris.

Compact mode berguna untuk trader yang hanya ingin signal + bias direction,
tanpa full statistical breakdown.

**Gap:** Tidak ada OutputMode field di UserPrefs, tidak ada toggle di /settings.

---

### 4. Pin & Favorites — Belum Ada (UX-014)
**File:** `domain/prefs.go`, keyboard.go

UX_AUDIT TASK-UX-014: user bisa "pin" command favorit, shortcut muncul di keyboard.
`UserPrefs` tidak ada `PinnedCommands []string` atau sejenisnya.
`MainMenu()` di keyboard.go static, tidak bisa customize per user.

Use case: institutional trader hanya butuh /cot + /outlook + /macro.
Retail trader hanya butuh /bias + /calendar.
Saat ini semua user lihat keyboard yang sama.

**Gap:** No personalized keyboard based on user preferences.

---

### 5. Number Formatting Inconsistency (Format & Visual section UX_AUDIT)
**File:** `internal/adapter/telegram/formatter.go` dan files formatter lain

Audit codebase untuk format angka:
- `fmt.Sprintf("%.0f", netPos)` → "123456" (no thousand separator)
- Beberapa tempat pakai `fmt.Sprintf("%+.0f", val)` → "+123456"
- Tidak ada centralized number formatter

UX_AUDIT merekomendasikan:
- Selalu gunakan separator ribuan: 123,456 bukan 123456
- Persentase: 1 decimal (67.3%)
- Harga forex: 5 decimal major, 2 decimal JPY

**Gap:** Tidak ada `FormatNumber()` / `FormatThousands()` utility function yang dipakai konsisten.

---

## Rekomendasi Task

### TASK-075: Loading Indicator untuk /alpha, /rank, /bias + Telegram ChatAction
Priority: HIGH | Effort: S (2 jam)
- Tambah SendHTML("⏳...") sebelum compute di cmdAlpha, cmdRank, cmdBias
- Pattern: send → compute → DeleteMessage (mirip quant/vp pattern)
- Tambah helper `bot.SendChatAction(ctx, chatID, "typing")` di bot.go
- Panggil sebelum AI-heavy commands (outlook, alpha)

### TASK-076: Standardize Back Button Language ke Indonesia
Priority: MEDIUM | Effort: S (1-2 jam)
- Audit semua "<<" / "Back" / "Kembali" di keyboard.go
- Standardisasi ke "◀ Ringkasan", "◀ Dashboard", "◀ Kategori"
- Pilih satu bahasa (Indonesia) untuk semua navigation UI text

### TASK-077: Compact Mode — OutputMode di UserPrefs + /settings toggle
Priority: MEDIUM | Effort: M (3-4 jam)
- Tambah `OutputMode string` ke UserPrefs ("full", "compact", "minimal")
- Tambah toggle di cmdSettings keyboard
- Implementasi compact format di COT detail formatter (hanya top summary)
- DefaultPrefs: "full"

### TASK-078: Personalized Main Keyboard (PinnedCommands)
Priority: LOW | Effort: M (3-4 jam)
- Tambah `PinnedCommands []string` ke UserPrefs (max 4)
- Command `/pin cot EUR` → tambah ke PinnedCommands
- `MainMenu()` di keyboard.go: jika user punya pins, tambah row "⭐ Pinned" di atas
- `/unpin` untuk hapus

### TASK-079: Number Formatting Utility — FormatThousands + FormatPct
Priority: MEDIUM | Effort: S (2 jam)
- Buat `pkg/format/numbers.go` dengan fungsi:
  - `FormatInt(n int64) string` → "123,456"
  - `FormatFloat(f float64, decimals int) string` → "123,456.78"
  - `FormatPct(f float64) string` → "67.3%"
  - `FormatForex(f float64, isJPY bool) string` → 5 atau 2 decimal
- Refactor formatter.go untuk pakai utility ini
- Test coverage di pkg/format/numbers_test.go

