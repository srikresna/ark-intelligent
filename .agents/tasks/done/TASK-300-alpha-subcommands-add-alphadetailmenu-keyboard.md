# TASK-300: Tambah AlphaDetailMenu Keyboard ke Alpha Standalone Sub-Commands

**Priority:** high
**Type:** ux-improvement
**Estimated:** XS
**Area:** internal/adapter/telegram/handler_alpha.go
**Created by:** Research Agent
**Created at:** 2026-04-03 00:00 WIB
**Siklus:** UX-1 (Siklus 1 Putaran 22)
**Ref:** research/2026-04-03-00-ux-alpha-subcommands-history-backtest-keyboard-putaran22.md

## Deskripsi

Enam command alpha standalone menghasilkan output via `SendHTML` tanpa keyboard:
- `/xfactors` (handler_alpha.go:536)
- `/playbook` (handler_alpha.go:570)
- `/heat` (handler_alpha.go:601)
- `/rankx` (handler_alpha.go:621)
- `/transition` (handler_alpha.go:653, 666)
- `/cryptoalpha` (handler_alpha.go:690)

Saat user navigate via `/alpha` dashboard, semua detail views sudah punya `AlphaDetailMenu()` keyboard (Back + Home). Tapi jika user ketik command langsung (misalnya `/playbook`), output tidak ada keyboard. Inkonsistensi yang membingungkan.

`AlphaDetailMenu()` keyboard sudah ada di `keyboard.go:814` dan `AlphaCryptoDetailMenu()` di `keyboard.go:826`. Keduanya sudah digunakan di `handleAlphaCallback` — hanya belum dipakai di command entry points.

## Gap Saat Ini

```go
// handler_alpha.go:570 — cmdPlaybook
_, err = h.bot.SendHTML(ctx, chatID, formatPlaybook(result))
return err
// ← Tidak ada keyboard

// handler_alpha.go:601 — cmdHeat
_, err = h.bot.SendHTML(ctx, chatID, formatHeat(result.Heat))
return err
// ← Tidak ada keyboard
```

## Perubahan yang Diperlukan

Untuk setiap command, ganti `SendHTML` di success output path dengan `SendWithKeyboard`:

### /xfactors (handler_alpha.go ~536)
```go
// Sebelum:
_, err = h.bot.SendHTML(ctx, chatID, formatFactorRanking(result))
return err

// Sesudah:
kb := h.kb.AlphaDetailMenu()
_, err = h.bot.SendWithKeyboard(ctx, chatID, formatFactorRanking(result), kb)
return err
```

### /playbook (~570)
```go
// Sesudah:
kb := h.kb.AlphaDetailMenu()
_, err = h.bot.SendWithKeyboard(ctx, chatID, formatPlaybook(result), kb)
return err
```

### /heat (~601)
```go
// Sesudah:
kb := h.kb.AlphaDetailMenu()
_, err = h.bot.SendWithKeyboard(ctx, chatID, formatHeat(result.Heat), kb)
return err
```

### /rankx (~621)
```go
// Sesudah:
kb := h.kb.AlphaDetailMenu()
_, err = h.bot.SendWithKeyboard(ctx, chatID, formatRankX(result), kb)
return err
```

### /transition (~653 dan ~666)
Kedua path (dengan full playbook dan fallback) perlu keyboard:
```go
// Path 1 (~653):
kb := h.kb.AlphaDetailMenu()
_, _ = h.bot.SendWithKeyboard(ctx, chatID, formatTransition(result.Transition, macroRegime), kb)
return nil

// Path 2 (~666, fallback):
kb := h.kb.AlphaDetailMenu()
_, err := h.bot.SendWithKeyboard(ctx, chatID, formatTransition(tw, macroRegime), kb)
return err
```

### /cryptoalpha (~690)
Gunakan `AlphaCryptoDetailMenu()` (sudah ada symbol selector BTC/ETH/SOL/BNB + Back + Home):
```go
// Sesudah:
kb := h.kb.AlphaCryptoDetailMenu()
_, err := h.bot.SendWithKeyboard(ctx, chatID, formatCryptoAlpha(results, symbols), kb)
return err
```

## File yang Harus Diubah

1. `internal/adapter/telegram/handler_alpha.go` — 6 function, masing-masing 1-2 line changes

## Verifikasi

```bash
go build ./...
# Test manual: ketik /xfactors, /playbook, /heat, /rankx, /transition, /cryptoalpha
# Pastikan keyboard muncul di setiap output
# Pastikan tombol Back di keyboard membawa ke /alpha dashboard (via "alpha:back" callback)
# Pastikan tombol Home berfungsi
```

## Acceptance Criteria

- [ ] `/xfactors` output punya `AlphaDetailMenu()` keyboard (Back + Home)
- [ ] `/playbook` output punya `AlphaDetailMenu()` keyboard
- [ ] `/heat` output punya `AlphaDetailMenu()` keyboard
- [ ] `/rankx` output punya `AlphaDetailMenu()` keyboard
- [ ] `/transition` output (semua path) punya `AlphaDetailMenu()` keyboard
- [ ] `/cryptoalpha` output punya `AlphaCryptoDetailMenu()` keyboard (symbol switcher + Back + Home)
- [ ] Error paths ("⚙️ Engine not configured") tetap plain HTML (tidak perlu keyboard)
- [ ] `go build ./...` clean

## Referensi

- `handler_alpha.go:814` — `AlphaDetailMenu()` keyboard definition
- `handler_alpha.go:826` — `AlphaCryptoDetailMenu()` keyboard definition
- `handler_alpha.go:232-337` — `handleAlphaCallback` yang sudah pakai keyboard di detail views
- research/2026-04-03-00-ux-alpha-subcommands-history-backtest-keyboard-putaran22.md — Temuan 1
