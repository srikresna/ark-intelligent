# TASK-302: Tambah Week Switcher Keyboard ke /history Command

**Priority:** medium
**Type:** ux-improvement
**Estimated:** S
**Area:** internal/adapter/telegram/handler.go, internal/adapter/telegram/keyboard.go
**Created by:** Research Agent
**Created at:** 2026-04-03 00:00 WIB
**Siklus:** UX-1 (Siklus 1 Putaran 22)
**Ref:** research/2026-04-03-00-ux-alpha-subcommands-history-backtest-keyboard-putaran22.md

## Deskripsi

`cmdHistory` (handler.go:1656) menghasilkan output COT history yang sudah support 3 mode (4/8/12 minggu via arg) tapi berakhir dengan:

```go
// handler.go:1729
_, err = h.bot.SendHTML(ctx, chatID, b.String())
return err
```

Tidak ada keyboard. Setelah lihat history 4 minggu, user harus retype `/history EUR 8` untuk lihat 8 minggu. Friction yang tidak perlu.

`/history` command sudah fully implemented (sparkline, table, delta). Yang kurang hanya keyboard navigasi.

## Keyboard yang Diinginkan

```
[4 Minggu] [8 Minggu] [12 Minggu]
[🏠 Menu Utama]
```

Dengan callback data format: `hist:{currency}:{weeks}` → misalnya `hist:EUR:4`, `hist:EUR:8`, `hist:EUR:12`.

Tombol untuk minggu yang sedang ditampilkan diberi marker aktif (misalnya tanda ✓ atau bold di text).

## Perubahan yang Diperlukan

### 1. Tambah `HistoryNavKeyboard` di keyboard.go

```go
// HistoryNavKeyboard builds the navigation keyboard for /history command.
// currentWeeks: 4, 8, or 12 — highlights the active selection.
func (kb *KeyboardBuilder) HistoryNavKeyboard(currency string, currentWeeks int) ports.InlineKeyboard {
    label := func(w int) string {
        if w == currentWeeks {
            return fmt.Sprintf("✓ %dW", w)
        }
        return fmt.Sprintf("%dW", w)
    }
    return ports.InlineKeyboard{
        Rows: [][]ports.InlineButton{
            {
                {Text: label(4), CallbackData: fmt.Sprintf("hist:%s:4", currency)},
                {Text: label(8), CallbackData: fmt.Sprintf("hist:%s:8", currency)},
                {Text: label(12), CallbackData: fmt.Sprintf("hist:%s:12", currency)},
            },
            kb.HomeRow(),
        },
    }
}
```

### 2. Update cmdHistory di handler.go — ganti SendHTML dengan SendWithKeyboard

```go
// handler.go:1729 — sebelum:
_, err = h.bot.SendHTML(ctx, chatID, b.String())
return err

// Sesudah:
kb := h.kb.HistoryNavKeyboard(currency, weeks)
_, err = h.bot.SendWithKeyboard(ctx, chatID, b.String(), kb)
return err
```

### 3. Tambah cbHistory callback handler di handler.go

```go
// cbHistory handles "hist:" prefixed callbacks for /history week navigation.
func (h *Handler) cbHistory(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
    // data format: "hist:{currency}:{weeks}"
    parts := strings.Split(strings.TrimPrefix(data, "hist:"), ":")
    if len(parts) != 2 {
        return nil
    }
    currency := strings.ToUpper(parts[0])
    weeks, err := strconv.Atoi(parts[1])
    if err != nil || weeks < 2 || weeks > 52 {
        return nil
    }

    // Re-run cmdHistory with new params (edit existing message)
    // Note: cmdHistory currently sends new message; for edit behavior,
    // refactor to accept optional msgID for edit-in-place.
    // Simplest approach: just invoke cmdHistory (sends new message, old one stays).
    return h.cmdHistory(ctx, chatID, userID, fmt.Sprintf("%s %d", currency, weeks))
}
```

**Alternatif lebih baik (edit-in-place):** Refactor `cmdHistory` agar bisa dipanggil dengan msgID untuk edit message daripada send baru. Tapi ini opsional — versi simplest (send new message) sudah jauh lebih baik dari kondisi saat ini.

### 4. Daftarkan callback di Register() (handler.go:~251)

```go
bot.RegisterCallback("hist:", h.cbHistory)
```

## File yang Harus Diubah

1. `internal/adapter/telegram/keyboard.go` — tambah `HistoryNavKeyboard()` function
2. `internal/adapter/telegram/handler.go` — update cmdHistory final output + tambah cbHistory + Register

## Verifikasi

```bash
go build ./...
# Test: /history EUR → pastikan keyboard 4W/8W/12W muncul
# Klik 8W → output history 8 minggu + keyboard dengan "✓ 8W"
# Klik 12W → output history 12 minggu
# Klik Home → kembali ke main menu
```

## Acceptance Criteria

- [ ] `/history EUR` output punya keyboard dengan 3 tombol week (4W, 8W, 12W)
- [ ] Week yang sedang aktif ditandai (✓ prefix)
- [ ] Klik week lain memuat ulang history untuk jumlah minggu tersebut
- [ ] Tombol Home berfungsi
- [ ] `hist:` callback prefix terdaftar di `Register()`
- [ ] `go build ./...` clean

## Referensi

- `handler.go:1656` — `cmdHistory` implementation
- `handler.go:235-236` — command registration (`/history`, `/h`)
- `handler.go:251` — callback registration list (tambah `hist:` di sini)
- research/2026-04-03-00-ux-alpha-subcommands-history-backtest-keyboard-putaran22.md — Temuan 3
