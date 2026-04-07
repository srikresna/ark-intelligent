# TASK-250: Tambah Home Button ke BacktestMenu, SeasonalMenu, PriceMenu Selector

**Priority:** medium
**Type:** ux
**Estimated:** XS
**Area:** internal/adapter/telegram/keyboard.go
**Created by:** Research Agent
**Created at:** 2026-04-02 04:00 WIB

## Deskripsi

`BacktestMenu` (keyboard.go:594), `SeasonalMenu` (keyboard.go:654), dan `PriceMenu` (keyboard.go:939) tidak memiliki tombol "🏠 Menu Utama" (`nav:home`).

TASK-228 sudah menambah home button ke ICT, SMC, GEX handler keyboards, tapi 3 selector menu ini terlewat. User yang mengetik `/backtest`, `/seasonal`, atau `/price` tanpa args mendapat selector keyboard tanpa exit ke main menu.

Pola standar di codebase: setiap keyboard yang menjadi "landing page" sebuah command harus memiliki home row sebagai escape route.

## File yang Harus Diubah

- `internal/adapter/telegram/keyboard.go`
  - `BacktestMenu()` — tambah home row setelah currency rows
  - `SeasonalMenu()` — tambah home row setelah crypto/cross rows
  - `PriceMenu()` — tambah home row setelah cross pair rows

## Implementasi

### BacktestMenu() — keyboard.go:~645 (setelah baris terakhir currency)

```go
// Tambah setelah GOLD row:
{
    {Text: btnHome, CallbackData: "nav:home"},
},
```

### SeasonalMenu() — keyboard.go:~707 (setelah XAGEUR/XAGGBP row)

```go
// Tambah setelah cross pair rows:
{
    {Text: btnHome, CallbackData: "nav:home"},
},
```

### PriceMenu() — keyboard.go:~991 (setelah XAGEUR/XAGGBP row)

```go
// Tambah setelah cross pair rows:
{
    {Text: btnHome, CallbackData: "nav:home"},
},
```

**Catatan:** `btnHome` sudah didefinisikan di keyboard.go:41 sebagai `"🏠 Menu Utama"`.

## Acceptance Criteria

- [ ] `/backtest` tanpa args → keyboard ada tombol 🏠 Menu Utama
- [ ] `/seasonal` tanpa args → keyboard ada tombol 🏠 Menu Utama
- [ ] `/price` tanpa args → keyboard ada tombol 🏠 Menu Utama
- [ ] Klik 🏠 Menu Utama dari ketiga view → kembali ke help menu
- [ ] Buttons lain (currency selector) tetap berfungsi normal
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-04-ux-audit-navigation-context-settings-putaran12.md` — Temuan 1
- `keyboard.go:41` — btnHome konstanta
- `keyboard.go:594` — BacktestMenu (perlu update)
- `keyboard.go:654` — SeasonalMenu (perlu update)
- `keyboard.go:939` — PriceMenu (perlu update)
- `TASK-228` — pola referensi (ICT/SMC/GEX sudah fix)
