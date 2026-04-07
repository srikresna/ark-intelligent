# Research — Siklus 1: UX Audit (Putaran 17)

**Date:** 2026-04-02 ~25:xx WIB
**Focus:** Dead callback buttons, loading feedback inconsistency, cmdCOT UX
**Files Analyzed:** `internal/adapter/telegram/handler.go`, `keyboard.go`, `api.go`, `handler_ict.go`, `handler_smc.go`, `handler_cta.go`, `handler_wyckoff.go`

---

## Bug Ditemukan

### BUG-UX1: `nav:cot` Silently Ignored di cbNav

**File:** `internal/adapter/telegram/handler.go:1641`

**Problem:**
`cbNav` hanya handle satu action:
```go
func (h *Handler) cbNav(...) error {
    action := strings.TrimPrefix(data, "nav:")
    switch action {
    case "home":
        _ = h.bot.DeleteMessage(ctx, chatID, msgID)
        return h.cmdStart(ctx, chatID, userID, "")
    default:
        return nil  // ← silent no-op
    }
}
```

Namun `keyboard.go` mendefinisikan `nav:cot` di **3 tempat**:
- `MainMenu()` (line 766) — "📊 COT Analysis" button
- `StarterKitMenu("beginner")` (line 1293) — "📊 COT (Posisi Big Player)"
- `StarterKitMenu("intermediate")` (line 1309) — "📊 COT Analysis"

User experience: tekan tombol → tidak ada toast, tidak ada navigasi, tidak ada feedback sama sekali. Tombol **completely broken**.

**Fix:**
```go
case "cot":
    _ = h.bot.DeleteMessage(ctx, chatID, msgID)
    return h.cmdCOT(ctx, chatID, userID, "")
```

**Severity:** HIGH — tombol primary navigation di MainMenu dan onboarding StarterKitMenu

---

### BUG-UX2: `cmd:cta` Missing dari cbQuickCommand Switch

**File:** `internal/adapter/telegram/handler.go:1593`

**Problem:**
`cbQuickCommand` handles banyak `cmd:` callbacks, tetapi tidak ada `case "cta"`:
```go
// Missing:
case "cta":
    return h.cmdCTA(ctx, chatID, userID, args)
```

`keyboard.go` mendefinisikan `cmd:cta` di 2 tempat:
- `StarterKitMenu("intermediate")` (line 1313) — "📉 CTA Dashboard"
- `StarterKitMenu("advanced")` (line 1337) — "📉 CTA + Backtest"

Intermediate dan advanced users yang onboarding melalui StarterKit, tombol CTA mereka **tidak berfungsi**.

**Fix:**
```go
case "cta":
    return h.cmdCTA(ctx, chatID, userID, args)
```

(Perlu verifikasi nama method: `cmdCTA` atau `cmdCTADashboard`)

**Severity:** HIGH — menimpa primary CTA button untuk intermediate/advanced users di onboarding

---

## UX Issues (Non-Bug)

### ISSUE-UX3: cmdRank dan cmdBias Tidak Menggunakan SendLoading

**Files:** `handler.go:1956` (cmdRank), `handler.go:1255` (cmdBias)

**Problem:**
```go
// cmdRank:
loadingID, _ := h.bot.SendHTML(ctx, chatID, "📈 Menghitung currency strength ranking... ⏳")
// ... do work ...
_ = h.bot.DeleteMessage(ctx, chatID, loadingID)  // ← delete loading, send result separately

// cmdBias:
loadingID, _ := h.bot.SendHTML(ctx, chatID, "🎯 Mendeteksi directional bias... ⏳")
// ... do work ...
_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
```

Pola ini:
1. Tidak mengirim `SendTyping` terlebih dahulu (tidak ada typing indicator di chat)
2. Mengirim pesan baru (bukan edit) → selalu ada flash/jump di chat
3. `SendLoading` sudah ada di api.go dan otomatis kirim typing + HTML, dan hasilnya bisa di-edit

**Pola yang benar** (sudah dipakai di cmdMacro, cmdSentiment, cmdSMC, cmdQuant, dll):
```go
loadingID, _ := h.bot.SendLoading(ctx, chatID, "📈 Menghitung ranking... ⏳")
// ... do work ...
if err != nil {
    h.editUserError(ctx, chatID, loadingID, err, "rank")
    return nil
}
h.bot.EditMessage(ctx, chatID, loadingID, result)
```

**Severity:** MEDIUM — inkonsistensi UX (no typing indicator, message flash)

---

### ISSUE-UX4: cmdCOT Overview Tidak Ada Loading Feedback

**File:** `handler.go:550`

**Problem:**
`cmdCOT` (tanpa args, overview mode) tidak ada loading indicator sama sekali — langsung query DB dan kirim hasil. Jika DB query butuh waktu (cold start, BadgerDB scan), user tidak tahu bot sedang bekerja.

Semua handler berat lainnya sudah pakai SendLoading atau minimal SendTyping. cmdCOT overview adalah satu-satunya command utama yang tanpa feedback.

**Fix:**
Tambah `h.bot.SendTyping(ctx, chatID)` di awal fungsi, atau gunakan pola SendLoading + EditMessage jika ingin konsisten.

**Severity:** LOW-MEDIUM — fungsional tapi inconsistent dengan handler lain

---

### ISSUE-UX5: Pattern Fragil — cbNav & cbQuickCommand Tanpa Exhaustive Case Coverage

**Files:** `handler.go:1593`, `handler.go:1643`

**Problem:**
Saat ini kedua switch handler (`cbNav` dan `cbQuickCommand`) pakai `default: return nil` yang silent. Ketika keyboard button baru ditambahkan di `keyboard.go` tanpa menambahkan corresponding case di handler, bug tidak terdeteksi sampai user melaporkan.

Tidak ada mekanisme:
- Compile-time check untuk callback coverage
- Test untuk verify semua keyboard callbacks ter-handle
- Log warning untuk unhandled (silent no-op)

**Contoh nyata:** BUG-UX1 dan BUG-UX2 di atas — kemungkinan sudah ada sejak fitur onboarding ditambahkan, tapi tidak ada yang notice karena tidak ada coverage test.

**Fix:**
1. `default` case harus setidaknya log warning:
   ```go
   default:
       log.Warn().Str("data", data).Msg("unhandled nav action")
       _ = h.bot.AnswerCallback(ctx, cbID, "⚠️ Aksi tidak tersedia")
       return nil
   ```
2. Unit test yang enumerate semua `CallbackData` dari keyboard.go dan verify setiap prefix ter-handle

**Severity:** MEDIUM — prevention measure, tidak langsung user-facing

---

## Temuan Positif (Already Done Well)

- `api.go` punya `SendLoading()` helper yang bagus (SendTyping + SendHTML sekaligus)
- `errors.go` punya `userFriendlyError()` dan `sendUserError()` yang comprehensive
- `cbQuickCommand` sudah handle 14+ cmd: cases dengan baik
- Handler berat (cmdSMC, cmdQuant, cmdAlpha, cmdWyckoff, cmdGEX, cmdVP, cmdCTA, cmdMacro, cmdSentiment) semua sudah pakai SendLoading pattern

---

## Tasks Dibuat

- TASK-275: Fix BUG-UX1 — nav:cot tidak di-handle di cbNav
- TASK-276: Fix BUG-UX2 — cmd:cta tidak di-handle di cbQuickCommand
- TASK-277: Refactor cmdRank + cmdBias ke pola SendLoading
- TASK-278: Tambah loading indicator di cmdCOT overview
- TASK-279: Tambah log warning + unit test coverage untuk unhandled callbacks
