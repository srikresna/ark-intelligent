# TASK-304: Tambah Loading Indicator ke Backtest Sub-Handler Functions

**Priority:** medium
**Type:** ux-improvement
**Estimated:** S
**Area:** internal/adapter/telegram/handler_backtest.go
**Created by:** Research Agent
**Created at:** 2026-04-03 00:00 WIB
**Siklus:** UX-1 (Siklus 1 Putaran 22)
**Ref:** research/2026-04-03-00-ux-alpha-subcommands-history-backtest-keyboard-putaran22.md

## Deskripsi

`handler_backtest.go` tidak memiliki **satu pun** loading indicator:

```bash
$ grep -c "SendLoading\|SendTyping" handler_backtest.go
0
```

Semua backtest sub-handlers yang compute-heavy memanggil langsung tanpa feedback:

```go
// backtestWalkforward — ~10-30 detik
stats, err := calc.ComputeWalkForward(ctx)
_, err = h.bot.SendHTML(ctx, chatID, htmlOut)

// backtestMonteCarlo — bisa >30 detik
result, err := calc.ComputeMonteCarlo(ctx, 1000)
_, err = h.bot.SendHTML(ctx, chatID, htmlOut)

// backtestPortfolio, backtestSmartMoney, backtestExcursion, dll — serupa
```

User yang klik tombol dari BacktestMenu ("🔄 Walk-Forward", "🎲 Monte Carlo", dll) menunggu sampai 30+ detik tanpa feedback apapun. Bot terlihat hang.

## Functions yang Perlu Loading Indicator

| Function | Estimasi Duration | Loading Message |
|----------|-------------------|-----------------|
| `backtestWalkforward` | 10-30s | "🔄 Menjalankan Walk-Forward Analysis... ⏳" |
| `backtestMonteCarlo` | 5-30s | "🎲 Menjalankan Monte Carlo Simulation (1000 runs)... ⏳" |
| `backtestPortfolio` | 3-8s | "📈 Menghitung Portfolio-level Stats... ⏳" |
| `backtestSmartMoney` | 3-8s | "🧠 Menganalisis Smart Money Signals... ⏳" |
| `backtestExcursion` | 3-5s | "📊 Menghitung MFE/MAE Excursion... ⏳" |
| `backtestTrend` | 3-5s | "📈 Menghitung Trend Filter Analysis... ⏳" |
| `backtestTimingAnalysis` | 3-5s | "⏱ Menganalisis Signal Timing... ⏳" |

Fungsi ringan (`backtestAll`, `backtestBySignalType`, `backtestRegime`, `backtestMatrix`) yang hanya call `calc.ComputeAll(ctx)` sudah cukup cepat (< 2s) dan bisa skip.

## Pattern Implementasi

Gunakan pattern yang sama dengan `handler_cta.go:159`:

```go
func (h *Handler) backtestWalkforward(ctx context.Context, chatID string, calc *backtestsvc.StatsCalculator) error {
    loadingID, _ := h.bot.SendLoading(ctx, chatID, "🔄 Menjalankan Walk-Forward Analysis... ⏳")

    result, err := calc.ComputeWalkForward(ctx)

    if loadingID > 0 {
        _ = h.bot.DeleteMessage(ctx, chatID, loadingID)
    }

    if err != nil {
        h.sendUserError(ctx, chatID, err, "backtest")
        return nil
    }
    // ... rest of function
}
```

### Jika SendLoading belum tersedia:

```go
loadingMsg := "🔄 Menjalankan Walk-Forward Analysis... ⏳"
loadingID, _ := h.bot.SendHTML(ctx, chatID, loadingMsg)
// ... computation ...
if loadingID > 0 { _ = h.bot.DeleteMessage(ctx, chatID, loadingID) }
```

## Catatan: Tipe Parameter

Perlu dicek signature setiap fungsi karena beberapa mungkin tidak menerima `chatID` langsung — mungkin dipanggil via `cmdBacktest` dispatcher. Periksa call chain:

```go
// cmdBacktest dispatch:
case "wf":
    return h.backtestWalkforward(ctx, chatID, calc)
case "mc":
    return h.backtestMonteCarlo(ctx, chatID, calc)
```

Jika fungsi-fungsi ini tidak memiliki `chatID` parameter (mungkin hanya return data), perlu tambah parameter atau move loading ke caller (`cmdBacktest`).

## File yang Harus Diubah

1. `internal/adapter/telegram/handler_backtest.go` — 7 functions, masing-masing ~3 lines tambahan

## Verifikasi

```bash
go build ./...
# Test: klik tombol Walk-Forward dari BacktestMenu
# Pastikan loading message muncul dalam 1 detik
# Pastikan loading message hilang dan digantikan hasil
# Test Monte Carlo — loading harus persist selama compute (bisa 10-30s)
```

## Acceptance Criteria

- [ ] `backtestWalkforward` tampilkan loading indicator
- [ ] `backtestMonteCarlo` tampilkan loading indicator
- [ ] `backtestPortfolio` tampilkan loading indicator
- [ ] `backtestSmartMoney` tampilkan loading indicator
- [ ] `backtestExcursion` tampilkan loading indicator
- [ ] `backtestTrend` tampilkan loading indicator
- [ ] `backtestTimingAnalysis` tampilkan loading indicator
- [ ] Loading message dihapus sebelum output final dikirim
- [ ] Error paths juga delete loading message sebelum mengirim error
- [ ] `go build ./...` clean

## Referensi

- `handler_cta.go:159` — contoh `SendLoading` pattern
- `handler_backtest.go` — semua backtestXxx functions (grep "^func (h \*Handler) backtest")
- TASK-075 — loading untuk /alpha, /rank, /bias (pattern yang sama)
- TASK-251 — keyboard untuk backtest sub-outputs (implement bersamaan untuk konsistensi)
- research/2026-04-03-00-ux-alpha-subcommands-history-backtest-keyboard-putaran22.md — Temuan 5
