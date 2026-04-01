# UX Audit — Navigation, Context Carry-Over, Settings — Putaran 12

**Date:** 2026-04-02 04:00 WIB
**Cycle:** 1/5 (UX Audit)
**Reference:** .agents/UX_AUDIT.md

---

## Temuan Utama

### 1. Selector Menus Tanpa Home Button (BARU)

`BacktestMenu` (keyboard.go:594), `SeasonalMenu` (keyboard.go:654), dan `PriceMenu` (keyboard.go:939) **tidak memiliki tombol "🏠 Menu Utama"** (`nav:home`) di keyboard selector mereka.

TASK-228 (merged) sudah fix ICT/SMC/GEX handler keyboards. Tapi 3 keyboard selector ini (yang muncul saat `/backtest`, `/seasonal`, `/price` tanpa args) terlewat.

Dampak: User yang buka `/backtest` atau `/seasonal` tidak bisa kembali ke main menu tanpa mengetik `/help`.

```
BacktestMenu (line 594): hanya punya currency rows, tidak ada home row
SeasonalMenu (line 654): hanya punya currency selector rows, tidak ada home row
PriceMenu (line 939): hanya punya currency rows, tidak ada home row
```

### 2. handler_backtest.go — Banyak Final Output Tanpa Keyboard (BARU)

`handler_backtest.go` memiliki **20+ `SendHTML` calls** untuk final output (bukan error) tanpa keyboard navigasi apapun. Semua subcommands (`signals`, `timing`, `wf`, `weights`, dll) mengirim plain HTML.

Hanya satu tempat (line 143-144) yang menggunakan `BacktestMenu` keyboard. Semua drill-down result tidak punya navigation.

### 3. cmdSeasonal Mengabaikan userID (BARU)

`handler_seasonal.go:18`: `func (h *Handler) cmdSeasonal(ctx context.Context, chatID string, _ int64, args string)` — userID diabaikan dengan `_`.

Signature `CommandHandler` sudah support userID. Tapi cmdSeasonal membuang parameter ini, sehingga:
- Tidak bisa memanggil `saveLastCurrency`
- Tidak bisa membaca `prefs.ExperienceLevel`
- Tidak bisa personalisasi output

cmdPrice (handler_price.go:14) dan cmdLevels (handler_levels.go:14) sudah punya `userID int64` tapi TIDAK memanggil `saveLastCurrency` setelah berhasil menampilkan data.

### 4. SettingsMenu Tidak Bisa Mengubah ExperienceLevel (BARU)

`SettingsMenu` (keyboard.go:320) dan `FormatSettings` (formatter.go:932) **tidak menampilkan ExperienceLevel** dan **tidak ada tombol untuk mengubahnya**.

`ExperienceLevel` dikontrol hanya di `/start` onboarding. Setelah set, tidak ada cara untuk ubah kecuali reset database atau delete `/start` handler workaround.

Dampak: User yang di-onboard sebagai "pemula" dan sekarang sudah "pro" tidak bisa mengubah experience level. Ini mempengaruhi `StarterKitMenu` yang ditampilkan setelah tutorial.

### 5. FormatSettings Tidak Menampilkan LastCurrency + ExperienceLevel (BARU)

`formatter.go:932 FormatSettings()` tidak menampilkan dua field penting dari UserPrefs:
- `LastCurrency` — user tidak tahu currency terakhir yang tersimpan
- `ExperienceLevel` — user tidak tahu level yang terset saat onboarding

Kedua field ada di struct, tapi tidak ditampilkan di settings view. User discovery sangat rendah untuk fitur ini.

---

## UX Issues Yang Sudah Punya Task (Tidak Duplikat)

| Issue | Task |
|-------|------|
| ICT/SMC/GEX missing home button | TASK-228 |
| Standardize back button language | TASK-076 |
| Loading indicator alpha/rank/bias | TASK-075 |
| Timestamp format standardization | TASK-054 |
| Context carry-over lastCurrency (COT/CTA/Quant) | TASK-175 |
| Levels/Price homerow keyboard | TASK-227 |
| Settings OutputMode toggle | TASK-229 |
| Daily briefing command | TASK-029 |

---

## Rekomendasi Tasks

1. **TASK-250** — Tambah Home Button ke BacktestMenu, SeasonalMenu, PriceMenu selector
2. **TASK-251** — handler_backtest.go: Semua final output SendHTML tanpa keyboard
3. **TASK-252** — cmdPrice + cmdLevels: Panggil saveLastCurrency setelah berhasil
4. **TASK-253** — cmdSeasonal: Fix signature untuk gunakan userID + saveLastCurrency
5. **TASK-254** — SettingsMenu + FormatSettings: Tampilkan + ubah ExperienceLevel
