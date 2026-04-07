# TASK-301: Tambah Loading Indicator ke Alpha Standalone Sub-Commands

**Priority:** medium
**Type:** ux-improvement
**Estimated:** S
**Area:** internal/adapter/telegram/handler_alpha.go
**Created by:** Research Agent
**Created at:** 2026-04-03 00:00 WIB
**Siklus:** UX-1 (Siklus 1 Putaran 22)
**Ref:** research/2026-04-03-00-ux-alpha-subcommands-history-backtest-keyboard-putaran22.md

## Deskripsi

TASK-075 sudah cover loading indicator untuk `/alpha`, `/rank`, `/bias`. Tapi enam alpha standalone sub-commands berikut **tidak punya loading indicator**:

- `/xfactors` — memanggil `ProfileBuilder.BuildProfiles(ctx)` + `FactorEngine.Rank()`
- `/playbook` — memanggil `BuildProfiles()` + `StrategyEngine.Generate()`
- `/heat` — identik dengan `/playbook`
- `/rankx` — memanggil `BuildProfiles()` + `FactorEngine.Rank()`
- `/cryptoalpha` — memanggil `MicroEngine.AnalyzeMultiple()` dengan multiple API calls

Command `/transition` tidak butuh loading (compute ringan — hanya ambil cached regime).

`BuildProfiles()` bisa 3-8 detik (fetch FRED, COT, price data). `AnalyzeMultiple()` bisa 5-10 detik (multiple Bybit API calls). Saat ini user ketik command → bot diam → output tiba-tiba muncul.

## Pattern Referensi

Sudah ada di `handler_cta.go:159`:
```go
loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf("⚡ Computing TA for <b>%s</b>... ⏳", html.EscapeString(mapping.Currency)))
// ... heavy computation ...
if loadingID > 0 { _ = h.bot.DeleteMessage(ctx, chatID, loadingID) }
```

Dan di `handler.go:1256` (cmdBias):
```go
loadingID, _ := h.bot.SendHTML(ctx, chatID, "🎯 Mendeteksi directional bias... ⏳")
// ... computation ...
if loadingID > 0 { _ = h.bot.DeleteMessage(ctx, chatID, loadingID) }
```

## Perubahan yang Diperlukan

### /xfactors (handler_alpha.go ~523)
```go
func (h *Handler) cmdXFactors(ctx context.Context, chatID string, _ int64, _ string) error {
    if h.alpha == nil || h.alpha.FactorEngine == nil || h.alpha.ProfileBuilder == nil {
        _, err := h.bot.SendHTML(ctx, chatID, "⚙️ Factor Engine not configured.")
        return err
    }
    loadingID, _ := h.bot.SendLoading(ctx, chatID, "📊 Menghitung Factor Ranking... ⏳")
    profiles, err := h.alpha.ProfileBuilder.BuildProfiles(ctx)
    if loadingID > 0 { _ = h.bot.DeleteMessage(ctx, chatID, loadingID) }
    if err != nil || len(profiles) == 0 {
        _, _ = h.bot.SendHTML(ctx, chatID, "❌ Could not build asset profiles: "+alphaErr(err))
        return nil
    }
    result := h.alpha.FactorEngine.Rank(profiles)
    kb := h.kb.AlphaDetailMenu()
    _, err = h.bot.SendWithKeyboard(ctx, chatID, formatFactorRanking(result), kb)
    return err
}
```

### /playbook (~544)
```go
loadingID, _ := h.bot.SendLoading(ctx, chatID, "🎯 Menyusun Strategy Playbook... ⏳")
// setelah BuildProfiles:
if loadingID > 0 { _ = h.bot.DeleteMessage(ctx, chatID, loadingID) }
```

### /heat (~578)
```go
loadingID, _ := h.bot.SendLoading(ctx, chatID, "🌡️ Menghitung Portfolio Heat... ⏳")
```

### /rankx (~609)
```go
loadingID, _ := h.bot.SendLoading(ctx, chatID, "📈 Menyusun RankX Leaderboard... ⏳")
```

### /cryptoalpha (~674)
```go
loadingID, _ := h.bot.SendLoading(ctx, chatID, "⚡ Menganalisis Crypto Microstructure... ⏳")
// setelah AnalyzeMultiple selesai:
if loadingID > 0 { _ = h.bot.DeleteMessage(ctx, chatID, loadingID) }
```

## Catatan

Jika `h.bot.SendLoading` belum tersedia (TASK-075 belum selesai), gunakan pola alternatif:
```go
loadingID, _ := h.bot.SendHTML(ctx, chatID, "📊 Menghitung... ⏳")
```

Kedua pola ini fungsional — `SendLoading` lebih elegant tapi fallback ke `SendHTML` pun berfungsi.

## File yang Harus Diubah

1. `internal/adapter/telegram/handler_alpha.go` — 5 functions (xfactors, playbook, heat, rankx, cryptoalpha)

## Verifikasi

```bash
go build ./...
# Test manual: ketik /xfactors, /playbook, /heat, /rankx, /cryptoalpha
# Pastikan loading message muncul sebelum komputasi
# Pastikan loading message hilang setelah output tampil
```

## Acceptance Criteria

- [ ] `/xfactors` tampilkan loading message sebelum `BuildProfiles()` + `FactorEngine.Rank()`
- [ ] `/playbook` tampilkan loading message sebelum `BuildProfiles()` + `StrategyEngine.Generate()`
- [ ] `/heat` tampilkan loading message
- [ ] `/rankx` tampilkan loading message sebelum `BuildProfiles()` + `FactorEngine.Rank()`
- [ ] `/cryptoalpha` tampilkan loading message sebelum `MicroEngine.AnalyzeMultiple()`
- [ ] Loading message dihapus sebelum output final dikirim
- [ ] `go build ./...` clean

## Referensi

- `handler_cta.go:159` — contoh `SendLoading` pattern
- `handler.go:1256` — contoh loading via `SendHTML` + `DeleteMessage`
- TASK-075 — loading untuk `/alpha`, `/rank`, `/bias` (related)
- TASK-300 — keyboard untuk alpha sub-commands (implement bersamaan)
- research/2026-04-03-00-ux-alpha-subcommands-history-backtest-keyboard-putaran22.md — Temuan 2
