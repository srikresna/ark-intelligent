# TASK-251: handler_backtest.go — Tambah Keyboard Navigasi ke Semua Final Output

**Priority:** medium
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram/handler_backtest.go
**Created by:** Research Agent
**Created at:** 2026-04-02 04:00 WIB

## Deskripsi

`handler_backtest.go` memiliki 20+ `h.bot.SendHTML()` calls untuk output utama (bukan error) tanpa keyboard navigasi apapun. Hanya satu tempat (line 143) yang menggunakan `BacktestMenu` keyboard.

Semua subcommand output (`signals`, `timing`, `wf`, `weights`, `sm`, `excursion`, `trend`, `baseline`, `regime`, `matrix`, `mc`, `portfolio`, `cost`, `dedup`, `ruin`, `audit`, dan currency drill-down) mengirim plain HTML. User tidak bisa navigate kembali ke BacktestMenu atau main menu.

## Gap Saat Ini

```go
// handler_backtest.go:161 (signal type detail) — tanpa keyboard:
_, err = h.bot.SendHTML(ctx, chatID, htmlOut)

// handler_backtest.go:178 (signals detail) — tanpa keyboard:
_, err = h.bot.SendHTML(ctx, chatID, htmlOut)

// handler_backtest.go:231 (signals by currency) — tanpa keyboard:
_, err = h.bot.SendHTML(ctx, chatID, html+suppressNote)

// ... dan 17+ lainnya
```

## Implementasi

### Pattern yang Dipakai

Untuk semua final output (bukan error/not-available message) di handler_backtest.go:

```go
// Sebelum:
_, err = h.bot.SendHTML(ctx, chatID, htmlOut)

// Sesudah:
navKb := ports.InlineKeyboard{Rows: [][]ports.InlineButton{
    {{Text: "📊 Backtest Menu", CallbackData: "cmd:backtest:all"},
     {Text: btnHome, CallbackData: "nav:home"}},
}}
_, err = h.bot.SendWithKeyboard(ctx, chatID, htmlOut, navKb)
```

### Baris yang Perlu Diupdate (Final Output Only)

Identifikasi semua `_, err = h.bot.SendHTML(ctx, chatID, html` yang merupakan output sukses (bukan error), lalu tambahkan keyboard navigasi.

Error messages seperti `"No signal data available yet"`, `"Backtest data not available yet"` boleh tetap plain HTML.

## Acceptance Criteria

- [ ] Semua subcommand backtest output punya tombol `📊 Backtest Menu` + `🏠 Menu Utama`
- [ ] Error dan "data not available" messages tetap plain HTML (tidak perlu keyboard)
- [ ] Klik `📊 Backtest Menu` → tampil ulang BacktestMenu overview
- [ ] `go build ./...` clean
- [ ] Tidak ada regresi pada fungsi backtest yang sudah berjalan

## Referensi

- `.agents/research/2026-04-02-04-ux-audit-navigation-context-settings-putaran12.md` — Temuan 2
- `handler_backtest.go:143` — satu-satunya place yang sudah pakai BacktestMenu (referensi)
- `handler_backtest.go:161,178,231` — contoh final output tanpa keyboard
- `TASK-227` — pola serupa untuk levels/price handler
- `TASK-228` — pola referensi home button
