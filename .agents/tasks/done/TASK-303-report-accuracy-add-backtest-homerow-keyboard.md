# TASK-303: Tambah HomeRow Keyboard ke /report dan /accuracy Output

**Priority:** medium
**Type:** ux-improvement
**Estimated:** XS
**Area:** internal/adapter/telegram/handler_backtest.go
**Created by:** Research Agent
**Created at:** 2026-04-03 00:00 WIB
**Siklus:** UX-1 (Siklus 1 Putaran 22)
**Ref:** research/2026-04-03-00-ux-alpha-subcommands-history-backtest-keyboard-putaran22.md

## Deskripsi

Dua command backtest yang sering diakses memiliki output final tanpa keyboard:

### 1. cmdReport (handler_backtest.go:13)

```go
htmlOut := h.fmt.FormatWeeklyReport(report)
_, err = h.bot.SendHTML(ctx, chatID, htmlOut)  // ← no keyboard
return err
```

`/report` menghasilkan weekly performance summary. Setelah membaca, user tidak punya tombol navigasi.

### 2. cmdAccuracy (handler_backtest.go:261)

```go
_, err = h.bot.SendHTML(ctx, chatID, html)  // ← no keyboard
return err
```

`cmdAccuracy` bahkan sudah punya text hint: `"<i>Use /backtest for detailed breakdown</i>"` tapi ini teks biasa, bukan tombol. User harus retype `/backtest` padahal tombol yang bisa diklik jauh lebih baik.

## Keyboard yang Diinginkan

```
[📊 Backtest Menu]  [🏠 Menu Utama]
```

Keyboard minimal ini memberi user escape route ke BacktestMenu atau main menu.

## Perubahan yang Diperlukan

### Buat BacktestBackKeyboard di keyboard.go (jika belum ada)

```go
// BacktestBackRow returns a keyboard with Backtest Menu + Home buttons.
func (kb *KeyboardBuilder) BacktestBackRow() ports.InlineKeyboard {
    return ports.InlineKeyboard{
        Rows: [][]ports.InlineButton{
            {
                {Text: "📊 Backtest Menu", CallbackData: "cmd:backtest"},
                {Text: btnHome, CallbackData: "nav:home"},
            },
        },
    }
}
```

(Jika sudah ada fungsi serupa di keyboard.go, gunakan yang sudah ada.)

### Update cmdReport (handler_backtest.go:27)

```go
// Sebelum:
_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
return err

// Sesudah:
kb := h.kb.BacktestBackRow()
_, err = h.bot.SendWithKeyboard(ctx, chatID, htmlOut, kb)
return err
```

### Update cmdAccuracy (handler_backtest.go:297)

```go
// Sebelum:
html += "\n<i>Use /backtest for detailed breakdown</i>"
_, err = h.bot.SendHTML(ctx, chatID, html)
return err

// Sesudah:
// Hapus atau keep text hint, tambah keyboard:
kb := h.kb.BacktestBackRow()
_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
return err
```

**Catatan:** Hapus baris `html += "\n<i>Use /backtest for detailed breakdown</i>"` karena keyboard button sudah menggantikan fungsi hint tersebut. Atau pertahankan jika ingin tetap ada teks hint untuk user yang forward message.

## File yang Harus Diubah

1. `internal/adapter/telegram/keyboard.go` — tambah `BacktestBackRow()` jika belum ada
2. `internal/adapter/telegram/handler_backtest.go` — 2 function, masing-masing 1-2 lines

## Verifikasi

```bash
go build ./...
# Test: /report → pastikan keyboard "Backtest Menu" + "Menu Utama" muncul
# Test: /accuracy → pastikan keyboard muncul
# Klik "Backtest Menu" → tampil BacktestMenu keyboard
# Klik "Menu Utama" → tampil main menu
```

## Acceptance Criteria

- [ ] `/report` output punya keyboard dengan tombol "📊 Backtest Menu" + "🏠 Menu Utama"
- [ ] `/accuracy` output punya keyboard yang sama
- [ ] Tombol "📊 Backtest Menu" navigate ke BacktestMenu (via `cmd:backtest`)
- [ ] Tombol "🏠 Menu Utama" navigate ke main menu (via `nav:home`)
- [ ] Error paths ("Report data not available yet", dll) tetap plain HTML
- [ ] `go build ./...` clean

## Referensi

- `handler_backtest.go:596` — `BacktestMenu()` keyboard definition
- `handler_backtest.go:144` — satu-satunya tempat BacktestMenu sudah dipakai (referensi pattern)
- TASK-251 — keyboard untuk semua backtest sub-outputs (lebih besar scope, TASK-303 fokus pada /report + /accuracy dulu)
- research/2026-04-03-00-ux-alpha-subcommands-history-backtest-keyboard-putaran22.md — Temuan 4
