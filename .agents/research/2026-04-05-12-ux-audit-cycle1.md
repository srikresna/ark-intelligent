# Research Report — UX/UI Audit (Cycle 1)
**Date:** 2026-04-05
**Fokus:** Siklus 1 — UX/UI Improvement
**Ref:** .agents/UX_AUDIT.md

---

## Metodologi

Analisis mendalam terhadap:
- `internal/adapter/telegram/handler_*.go` (50 file)
- `internal/adapter/telegram/keyboard_*.go`
- `internal/adapter/telegram/formatter*.go`
- `cmd/bot/main.go` (wiring)
- `internal/domain/prefs.go`

---

## Temuan Utama

### 🔴 CRITICAL BUG: `/compare` Tidak Teregistrasi

**File:** `internal/adapter/telegram/handler_cot_compare.go`
**Issue:** `cmdCompare` sudah diimplementasikan lengkap (side-by-side COT positioning), tapi **tidak ada `RegisterCommand("/compare", ...)`** di mana pun — tidak di `handler.go`, tidak ada `WithCompare()` method, tidak di `cmd/bot/main.go`.

Command `/compare EUR GBP` dari UX_AUDIT TASK-UX-013 **tidak dapat diakses user** sama sekali.

**Fix:** Tambah `d.Bot.RegisterCommand("/compare", h.cmdCompare)` di `handler.go` setelah registrasi `/history`. Tidak perlu `With*` pattern karena `cmdCompare` hanya butuh `h.cotRepo` yang sudah ada di `Handler`.

---

### 🟠 Loading Feedback Inconsistency (TASK-UX-004)

Pattern `SendLoading` + `EditMessage` sudah diimplementasikan di banyak handler, **tapi 14 handler masih hanya menggunakan `SendTyping`:**

| Handler | Command |
|---------|---------|
| `handler_price.go` | `/price` |
| `handler_carry.go` | `/carry` |
| `handler_bis.go` | `/bis` |
| `handler_onchain.go` | `/onchain` |
| `handler_orderflow.go` | `/orderflow` |
| `handler_briefing.go` | `/briefing` |
| `handler_levels.go` | `/levels` |
| `handler_scenario.go` | `/scenario` |
| `handler_defi.go` | `/defi` |
| `handler_vix_cmd.go` | `/vix` |
| `handler_regime.go` | `/regime` |

`SendTyping` hanya menampilkan "typing..." selama max 5 detik. Command yang butuh >5s tidak ada feedback visual setelah typing hilang.

---

### 🟠 `OutputMinimal` Mode — Dead Code (TASK-UX-010)

`internal/domain/prefs.go` mendefinisikan `OutputMinimal OutputMode = "minimal"`.
Settings keyboard menampilkan toggle compact → full → minimal.
**Tapi:** Tidak ada satu pun handler yang mengecek `prefs.OutputMode == domain.OutputMinimal`.

Hanya 2 tempat mengecek OutputMode sama sekali:
- `handler_cot_cmd.go`: cek `OutputFull` vs `OutputCompact`
- `handler_macro_cmd.go`: cek `OutputFull`

Mode `minimal` bisa disimpan di prefs tapi **tidak ada efek** pada output. User yang klik "Minimal" di settings merasa ada bug.

---

### 🟡 Navigation Label Inconsistency (TASK-UX-001 partial)

Dua label berbeda untuk tombol "home":
- `"🏠 Home"` (beberapa keyboard)
- `"🏠 Menu Utama"` (beberapa keyboard lain)

---

### 🟡 Context Carry-Over Tidak Merata (TASK-UX-007)

`getLastCurrency`/`saveLastCurrency` sudah ada di: `/cot`, `/cta`, `/quant`, `/price`, `/levels`, `/bias`, `/seasonal`, `/signal`, `/history`

**Belum ada di:**
- `/vp`, `/ict`, `/wyckoff`, `/smc`, `/elliott`, `/session`

---

### 🟡 RelatedCommandsKeyboard Coverage Gap

**Belum ada** di response: `/vp`, `/ict`, `/wyckoff`, `/smc`, `/session`, `/treasury`, `/bis`, `/carry`

---

## Status vs UX_AUDIT

| Task | Status |
|------|--------|
| TASK-UX-001: Unified Nav Bar | Partial — label inconsistent |
| TASK-UX-002: Onboarding Flow | ✅ Done |
| TASK-UX-003: Smart /help | ✅ Done |
| TASK-UX-004: Loading Feedback | Partial — 11 handler masih SendTyping only |
| TASK-UX-006: Command Shortcuts | ✅ Done |
| TASK-UX-007: Context Carry-Over | Partial — missing VP/ICT/Wyckoff/SMC/Elliott/Session |
| TASK-UX-008: Smart Alerts | ✅ Done |
| TASK-UX-009: Daily Briefing | ✅ Done |
| TASK-UX-010: Message Length Minimal | ❌ Dead code — setting exists, no effect |
| TASK-UX-012: Share Feature | ✅ Done |
| TASK-UX-013: History & Compare | Partial — /history OK, /compare NOT REGISTERED (bug!) |
| TASK-UX-014: Pin & Favorites | ✅ Done |
