# Research Report — Siklus 1: UX/UI Improvement (Sesi 2)

**Date:** 2026-04-01 18:00 WIB  
**Agent:** Research  
**Focus:** UX/UI gaps belum tercakup di task existing (TASK-025–029)

---

## Konteks

Siklus 1 sebelumnya (TASK-025–029) sudah mencakup:
- Sticky NavFooter di semua keyboard
- Smart /help interaktif
- Context carry-over lastCurrency
- Command shortcuts/aliases
- Daily /briefing command

Siklus ini fokus pada **4 UX pain point belum ada task-nya** dari UX_AUDIT.md, plus satu tambahan dari analisis codebase.

---

## Temuan

### 1. Share/Forward Feature — Belum Ada (UX-012)
`keyboard.go` tidak punya tombol share di manapun (hanya ada "🔄 Walk-Forward" sebagai false positive).
Output analysis (COT, Outlook, Alpha) sering di-screenshot atau di-forward manual oleh user.
Format HTML `<code>`, `<b>` tidak bisa langsung di-forward karena rendering Telegram-specific.

**Gap:** Tidak ada mekanisme generate "clean text" untuk forward.

### 2. Reaction Feedback — Belum Ada (UX-011)
Tidak ada `👍 Helpful`, `👎 Not helpful`, atau `🔔 Alert on change` di output manapun.
Bot tidak punya mekanisme feedback loop dari user.
Ini penting untuk tuning weight scoring conviction dan memahami command mana yang paling berguna.

**Gap:** Zero feedback mechanism.

### 3. Smart Alerts per Pair — Belum Ada (UX-008)
`domain/prefs.go` — `UserPrefs.COTAlertsEnabled` hanya master switch (bool).
`CurrencyFilter []string` ada, tapi hanya untuk economic calendar events, bukan untuk COT alerts.
User tidak bisa set "alert saat EUR COT berubah signifikan" tanpa alert semua currency.
Tidak ada threshold custom per pair.

**Gap:** Alert granularity sangat kasar.

### 4. History Command — Belum Ada (UX-013)
Tidak ada `/history <currency>` command di handler.go.
`cotRepo.GetAllLatestAnalyses()` ada, dan `cotRepo.GetHistoricalRecords()` / `GetRecordsForContract()` ada di codebase.
Data COT historical tersedia tapi tidak exposed via command.
UX audit minta: 4 minggu history dalam satu view + `/compare EUR GBP`.

**Gap:** Historical view tidak accessible oleh user.

### 5. Message Timestamp Inconsistency — Analisis Codebase
`formatter.go` punya 3 format timestamp berbeda:
- `"02 Jan 15:04"` (FRED macro, sentiment)
- `"02 Jan 15:04 WIB"` (alpha composites)
- `"15:04 WIB"` (calendar events)

Tidak ada "Updated: HH:MM WIB" footer konsisten di semua response.
UX audit merekomendasikan template: HEADER + CONTENT + TIMESTAMP + KEYBOARD.
TASK-002 (standardisasi button labels) tidak mencakup timestamp format.

**Gap:** Timestamp inconsistency merusak professional look.

---

## Tasks Dibuat

- TASK-050: Share/Forward feature (📤 button + clean text generator)
- TASK-051: Reaction feedback buttons (👍/👎/🔔)
- TASK-052: Smart alerts per pair dengan threshold (extend UserPrefs)
- TASK-053: /history command — COT positioning 4-week view
- TASK-054: Timestamp format standardization di semua formatter output

---

## Files Dianalisis

- `internal/adapter/telegram/handler.go` (2100+ lines)
- `internal/adapter/telegram/keyboard.go` (800+ lines)
- `internal/adapter/telegram/formatter.go` (2600+ lines)
- `internal/domain/prefs.go`
- `.agents/tasks/pending/` (50 tasks existing)
