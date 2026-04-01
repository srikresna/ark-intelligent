# TASK-227: Tambah HomeRow Keyboard ke /levels dan /price Output

**Priority:** medium
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram/
**Created by:** Research Agent
**Created at:** 2026-04-02 18:00 WIB

## Deskripsi

`handler_levels.go` mengirim semua output utama dengan `h.bot.SendHTML()` tanpa keyboard sama sekali (lines 79, 96). Demikian juga beberapa path di `handler_price.go` (lines 86, 104) yang menggunakan plain SendHTML.

Sesuai pola standar di codebase (lihat COT, Macro, Outlook), setiap final output harus ada minimal keyboard dengan home button agar user bisa navigate kembali ke menu utama tanpa mengetik command.

## File yang Harus Diubah

- `internal/adapter/telegram/handler_levels.go` — ubah `h.bot.SendHTML` → `h.bot.SendWithKeyboard` dengan HomeRow
- `internal/adapter/telegram/handler_price.go` — ubah path yang masih plain SendHTML menjadi SendWithKeyboard

## Implementasi

Untuk setiap final SendHTML di handler_levels.go dan handler_price.go yang mengirim konten utama (bukan error atau "data not available"):

```go
// Sebelum
_, err = h.bot.SendHTML(ctx, chatID, htmlOut)

// Sesudah
kb := ports.InlineKeyboard{Rows: [][]ports.InlineButton{
    h.kb.HomeRow(),
}}
_, err = h.bot.SendWithKeyboard(ctx, chatID, htmlOut, kb)
```

Untuk /levels: tambahkan juga tombol currency switcher jika context tersedia:
```go
{Text: "🔄 Refresh", CallbackData: "cmd:/levels " + symbol},
```

## Acceptance Criteria

- [ ] Output `/levels EUR` tampil dengan minimal tombol 🏠 Menu Utama
- [ ] Output `/price EUR` (daily history view) tampil dengan tombol home
- [ ] Error dan "data not available" messages tetap boleh tanpa keyboard
- [ ] Tidak ada regresi — command yang sudah punya keyboard tidak berubah

## Referensi

- `.agents/research/2026-04-02-18-ux-navigation-keyboard-gaps-putaran7.md` — Temuan 3
- `handler_levels.go:79,96` — SendHTML tanpa keyboard
- `handler_price.go:86,104` — SendHTML tanpa keyboard
- `keyboard.go:55` — HomeRow() function
