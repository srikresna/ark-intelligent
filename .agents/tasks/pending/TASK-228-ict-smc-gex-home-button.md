# TASK-228: Tambah Home Button ke ICT, SMC, dan GEX Keyboards

**Priority:** medium
**Type:** ux
**Estimated:** XS
**Area:** internal/adapter/telegram/
**Created by:** Research Agent
**Created at:** 2026-04-02 18:00 WIB

## Deskripsi

`ictNavKeyboard` (handler_ict.go:295), `smcNavKeyboard` (handler_smc.go:472), dan `gexKeyboard` (handler_gex.go:127) tidak memiliki tombol "🏠 Menu Utama" (callback `nav:home`).

Pola standar di codebase (lihat keyboard.go:478, keyboard.go:818) adalah: setiap nav row di-end dengan `{Text: btnHome, CallbackData: "nav:home"}` bersanding dengan back button. Ketiga handler ini dibuat belakangan dan melewatkan pattern ini.

Efek: User yang masuk ke /ict EUR H4, /smc XAUUSD, atau /gex ETH terjebak — "◀ Kembali" hanya kembali ke symbol selector, tidak ada exit ke main menu. Harus ketik /help atau /start manually.

## File yang Harus Diubah

- `internal/adapter/telegram/handler_ict.go` — update `ictNavKeyboard()`
- `internal/adapter/telegram/handler_smc.go` — update `smcNavKeyboard()`
- `internal/adapter/telegram/handler_gex.go` — update `gexKeyboard()`

## Implementasi

### handler_ict.go — ictNavKeyboard
```go
// Ubah actionRow dari:
actionRow := []ports.InlineButton{
    {Text: "🔄 Refresh", CallbackData: "ict:refresh:"},
    {Text: "◀ Kembali", CallbackData: "ict:sym:"},
}
// Menjadi:
actionRow := []ports.InlineButton{
    {Text: "🔄 Refresh", CallbackData: "ict:refresh:"},
    {Text: "◀ Kembali", CallbackData: "ict:sym:"},
    {Text: "🏠 Menu Utama", CallbackData: "nav:home"},
}
```

### handler_smc.go — smcNavKeyboard (pola identik dengan ICT)

### handler_gex.go — gexKeyboard
Tambah home row setelah refresh row:
```go
rows = append(rows, []ports.InlineButton{
    {Text: "🏠 Menu Utama", CallbackData: "nav:home"},
})
```

## Acceptance Criteria

- [ ] /ict EUR H4 tampil dengan tombol 🏠 Menu Utama di keyboard
- [ ] /smc EURUSD H4 tampil dengan tombol 🏠 Menu Utama di keyboard
- [ ] /gex BTC tampil dengan tombol 🏠 Menu Utama di keyboard
- [ ] Klik 🏠 Menu Utama dari ketiga view → kembali ke help menu
- [ ] Tombol lain (TF selector, Refresh, Kembali) tetap berfungsi normal

## Referensi

- `.agents/research/2026-04-02-18-ux-navigation-keyboard-gaps-putaran7.md` — Temuan 4
- `keyboard.go:478` — pola btnHome di COT detail
- `handler_ict.go:295` — ictNavKeyboard (perlu update)
- `handler_smc.go:472` — smcNavKeyboard (perlu update)
- `handler_gex.go:127` — gexKeyboard (perlu update)
