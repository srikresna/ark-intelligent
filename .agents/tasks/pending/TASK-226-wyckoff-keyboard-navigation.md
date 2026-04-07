# TASK-226: Tambah Keyboard Navigation ke /wyckoff Command

**Priority:** medium
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram/
**Created by:** Research Agent
**Created at:** 2026-04-02 18:00 WIB

## Deskripsi

`cmdWyckoff` (handler_wyckoff.go:112) mengirim hasil analisis dengan `h.bot.SendHTML(ctx, chatID, output)` tanpa keyboard sama sekali. Tidak ada tombol Refresh, Home, atau timeframe switcher setelah result muncul.

Pembanding: `/ict` dan `/smc` punya `ictNavKeyboard`/`smcNavKeyboard` dengan timeframe selector (H1/H4/D1), tombol Refresh, dan tombol Kembali.

## File yang Harus Diubah

- `internal/adapter/telegram/handler_wyckoff.go` — tambah keyboard function + update SendHTML → SendWithKeyboard
- `internal/adapter/telegram/handler_wyckoff.go` — tambah callback handler untuk refresh dan timeframe switch

## Implementasi

Tambah fungsi keyboard baru:
```go
func wyckoffNavKeyboard(symbol, currentTF string) ports.InlineKeyboard {
    tfRow := []ports.InlineButton{
        {Text: tfLabel("D1", currentTF), CallbackData: "wyckoff:tf:" + symbol + ":daily"},
        {Text: tfLabel("H4", currentTF), CallbackData: "wyckoff:tf:" + symbol + ":4h"},
        {Text: tfLabel("H1", currentTF), CallbackData: "wyckoff:tf:" + symbol + ":1h"},
    }
    actionRow := []ports.InlineButton{
        {Text: "🔄 Refresh", CallbackData: "wyckoff:refresh:" + symbol + ":" + currentTF},
        {Text: "◀ Kembali", CallbackData: "wyckoff:back:"},
        {Text: "🏠 Menu Utama", CallbackData: "nav:home"},
    }
    return ports.InlineKeyboard{Rows: [][]ports.InlineButton{tfRow, actionRow}}
}
```

Daftarkan callback di `WithWyckoff()`:
```go
h.bot.RegisterCallback("wyckoff:", h.cbWyckoff)
```

## Acceptance Criteria

- [ ] Hasil `/wyckoff EURUSD` tampil dengan keyboard (TF selector + Refresh + Home)
- [ ] Klik H4 → analisis di-refresh untuk timeframe 4h
- [ ] Klik Refresh → re-run analisis dengan timeframe yang sama
- [ ] Klik 🏠 Menu Utama → kembali ke help menu
- [ ] Loading indicator tetap ada sebelum hasil muncul
- [ ] Error states juga menggunakan sendUserError() pattern

## Referensi

- `.agents/research/2026-04-02-18-ux-navigation-keyboard-gaps-putaran7.md` — Temuan 2
- `handler_ict.go:278` — ictNavKeyboard sebagai referensi pattern
- `handler_wyckoff.go:112` — current SendHTML without keyboard
