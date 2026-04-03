# Research — Siklus 1: UX Audit (Putaran 22)

**Date:** 2026-04-03 00:xx WIB
**Focus:** Alpha standalone sub-commands tanpa keyboard, /history dan /report tanpa keyboard, backtest sub-handlers tanpa loading indicator
**Files Analyzed:** `handler_alpha.go`, `handler_backtest.go`, `handler.go` (cmdBias, cmdHistory, cmdAccuracy, cmdReport), `keyboard.go`

---

## Ringkasan Temuan

Putaran ini ditemukan **5 UX gap genuine** yang belum ada di pending tasks. Semua berkaitan dengan output akhir handler yang tidak punya keyboard navigasi atau loading indicator — pola yang konsisten di beberapa area yang belum pernah di-audit mendalam.

---

## Temuan 1: Alpha Standalone Sub-Commands — Tidak Ada Keyboard (6 Commands)

### File: `handler_alpha.go:523–690`

**Commands terdampak:** `/xfactors`, `/playbook`, `/heat`, `/rankx`, `/transition`, `/cryptoalpha`

Semua enam command di atas menggunakan `h.bot.SendHTML(ctx, chatID, ...)` di final output **tanpa keyboard**:

```go
// /xfactors (line 536)
_, err = h.bot.SendHTML(ctx, chatID, formatFactorRanking(result))
return err

// /playbook (line 570)
_, err = h.bot.SendHTML(ctx, chatID, formatPlaybook(result))
return err

// /heat (line 601)
_, err = h.bot.SendHTML(ctx, chatID, formatHeat(result.Heat))
return err

// /rankx (line 621)
_, err = h.bot.SendHTML(ctx, chatID, formatRankX(result))
return err

// /transition (line 653, 666)
_, _ = h.bot.SendHTML(ctx, chatID, formatTransition(result.Transition, macroRegime))
_, err := h.bot.SendHTML(ctx, chatID, formatTransition(tw, macroRegime))

// /cryptoalpha (line 690)
_, err := h.bot.SendHTML(ctx, chatID, formatCryptoAlpha(results, symbols))
```

`AlphaDetailMenu()` keyboard sudah ada di `keyboard.go:814` (Back + Home) dan **sudah digunakan di callbacks** (`handleAlphaCallback`). Tapi tidak digunakan di command entry points ini.

**Perbandingan:** Ketika user navigate via `/alpha` dashboard, semua detail views punya keyboard. Tapi kalau user ketik `/xfactors` langsung, output tidak ada keyboard. Inkonsistensi besar.

**Effort:** XS — ganti `SendHTML` → `SendWithKeyboard(..., h.kb.AlphaDetailMenu())` di 6 tempat.

---

## Temuan 2: Alpha Standalone Sub-Commands — Tidak Ada Loading Indicator

### File: `handler_alpha.go:523–690`

TASK-075 sudah cover `/alpha`, `/rank`, `/bias` loading indicator. Tapi **6 alpha sub-commands** di atas tidak ada loading indicator sama sekali.

Setiap command ini memanggil `h.alpha.ProfileBuilder.BuildProfiles(ctx)` atau `h.alpha.FactorEngine.Rank(profiles)` yang bisa 3-8 detik tergantung data. User ketik `/playbook`, bot diam 5 detik, lalu tiba-tiba keluar output. Tidak ada tanda loading.

**Fix pattern:**
```go
func (h *Handler) cmdPlaybook(ctx context.Context, chatID string, _ int64, _ string) error {
    if h.alpha == nil || ... {
        _, err := h.bot.SendHTML(ctx, chatID, "⚙️ Strategy Engine not configured.")
        return err
    }
    loadingID, _ := h.bot.SendLoading(ctx, chatID, "🎯 Menyusun Strategy Playbook... ⏳")
    profiles, err := h.alpha.ProfileBuilder.BuildProfiles(ctx)
    if loadingID > 0 { _ = h.bot.DeleteMessage(ctx, chatID, loadingID) }
    // ... rest of handler
}
```

**Effort:** S — apply to 5 commands (tidak perlu ke `/transition` yang compute-nya ringan).

---

## Temuan 3: `/history` Command — Output Tanpa Keyboard + Tidak Ada Week Switcher

### File: `handler.go:1729`

`cmdHistory` menghasilkan output COT history yang informatif tapi berakhir dengan:
```go
_, err = h.bot.SendHTML(ctx, chatID, b.String())
return err
```

Tidak ada keyboard sama sekali. Padahal command ini sudah support 3 mode: 4 minggu, 8 minggu, 12 minggu (via arg). Setelah lihat 4 minggu, user harus ketik ulang `/history EUR 8` untuk lihat lebih panjang.

**Keyboard ideal:**
```
[4W] [8W] [12W]       ← week switcher, callback: hist:EUR:4 dll
[🏠 Home]
```

Keyboard ini butuh callback prefix `hist:` yang belum terdaftar di `Register()`.

**Effort:** S — buat `HistoryNavKeyboard(currency, currentWeeks int)`, tambah `hist:` callback prefix.

---

## Temuan 4: `/report` dan `/accuracy` — Final Output Tanpa Keyboard

### File: `handler_backtest.go:27` (cmdReport), `handler_backtest.go:297` (cmdAccuracy)

**cmdReport (line 27):**
```go
htmlOut := h.fmt.FormatWeeklyReport(report)
_, err = h.bot.SendHTML(ctx, chatID, htmlOut)  // ← no keyboard
return err
```

**cmdAccuracy (line 297):**
```go
_, err = h.bot.SendHTML(ctx, chatID, html)  // ← no keyboard
return err
```

`cmdAccuracy` bahkan punya text hint manual: `"<i>Use /backtest for detailed breakdown</i>"` — tapi ini hanya teks, bukan tombol yang bisa diklik.

Kedua command ini sudah di-expose di keyboard BacktestMenu dan bisa diakses dari sana. Setelah hasilnya keluar, user tidak bisa navigate kembali ke BacktestMenu tanpa retype command.

**Fix:** Tambah `HomeRow()` keyboard yang sederhana (atau BacktestBackRow: `[📊 Backtest Menu] [🏠 Home]`).

**Effort:** XS — 2 line changes per command.

---

## Temuan 5: Backtest Sub-Handler Functions — Tidak Ada Loading Indicator

### File: `handler_backtest.go` (semua backtestXxx functions)

`backtestWalkforward`, `backtestMonteCarlo`, `backtestPortfolio`, `backtestSmartMoney`, `backtestExcursion`, `backtestTrend`, `backtestTimingAnalysis` — semua memanggil `calc.ComputeAll(ctx)` atau fungsi komputasi heavy lainnya **tanpa loading indicator**.

```bash
$ grep -n "SendLoading\|SendTyping\|sendLoading" handler_backtest.go
(kosong — tidak ada satupun)
```

Dibandingkan `handler_cta.go:159`:
```go
loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf("⚡ Computing TA for <b>%s</b>... ⏳", ...))
```

Backtest walk-forward bisa 10+ detik. Monte Carlo bisa lebih lama lagi. User yang klik tombol dari BacktestMenu menunggu tanpa feedback apapun.

**Fix:** Tambah loading message di awal setiap backtestXxx function sebelum heavy computation, delete sebelum send result.

**Effort:** S — ~10 functions, masing-masing 2-3 lines.

---

## Kesimpulan & Prioritas

| # | Temuan | Priority | Effort |
|---|--------|----------|--------|
| 1 | Alpha sub-commands tanpa keyboard | HIGH | XS |
| 2 | Alpha sub-commands tanpa loading | MEDIUM | S |
| 3 | /history tanpa week switcher keyboard | MEDIUM | S |
| 4 | /report + /accuracy tanpa keyboard | MEDIUM | XS |
| 5 | Backtest sub-handlers tanpa loading | MEDIUM | S |

Semua 5 temuan dijadikan task baru TASK-300 s/d TASK-304.
