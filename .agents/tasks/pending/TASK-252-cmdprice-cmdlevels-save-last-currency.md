# TASK-252: cmdPrice + cmdLevels — Panggil saveLastCurrency Setelah Berhasil

**Priority:** low
**Type:** ux
**Estimated:** XS
**Area:** internal/adapter/telegram/handler_price.go, handler_levels.go
**Created by:** Research Agent
**Created at:** 2026-04-02 04:00 WIB

## Deskripsi

`cmdPrice` (handler_price.go:14) dan `cmdLevels` (handler_levels.go:14) menerima `userID int64` sebagai parameter, tapi **tidak pernah memanggil `saveLastCurrency`** setelah berhasil menampilkan data suatu currency.

Helper `saveLastCurrency` sudah ada di handler.go:1765. `LastCurrency` field sudah ada di UserPrefs. TASK-175 meng-cover cmdCOT/cmdCTA/cmdQuant, tapi tidak mencakup cmdPrice dan cmdLevels.

Akibatnya: user yang sering pakai `/price EUR` atau `/levels XAU` tidak mendapat manfaat dari context carry-over — `LastCurrency` mereka tidak terupdate saat menggunakan command ini.

## File yang Harus Diubah

- `internal/adapter/telegram/handler_price.go` — `priceDetail()`
- `internal/adapter/telegram/handler_levels.go` — `levelsDetail()`

## Implementasi

### handler_price.go — priceDetail()

```go
// priceDetail shows daily price data for one instrument.
func (h *Handler) priceDetail(ctx context.Context, chatID string, mapping *domain.PriceSymbolMapping) error {
    // ... existing logic ...
    htmlOut := h.fmt.FormatDailyPrice(prices, mapping.Currency, mapping.ContractCode)
    kb := h.kb.PriceMenu()
    _, err = h.bot.SendWithKeyboard(ctx, chatID, htmlOut, kb)
    return err
}
```

Perlu: tambah `userID int64` ke signature priceDetail, lalu panggil `h.saveLastCurrency(ctx, userID, mapping.Currency)`.

### handler_levels.go — levelsDetail()

Sama: tambah `userID int64` ke signature levelsDetail, lalu:

```go
func (h *Handler) levelsDetail(ctx context.Context, chatID string, userID int64, mapping *domain.PriceSymbolMapping) error {
    // ... existing logic ...
    h.saveLastCurrency(ctx, userID, mapping.Currency) // tambahan
    htmlOut := h.fmt.FormatLevels(lc, mapping.Currency)
    _, err = h.bot.SendHTML(ctx, chatID, htmlOut)
    return err
}
```

**Catatan:** Update juga semua call sites dari `priceDetail` dan `levelsDetail` untuk meneruskan userID.

## Acceptance Criteria

- [ ] `/price EUR` → `prefs.LastCurrency` tersimpan sebagai "EUR"
- [ ] `/levels XAU` → `prefs.LastCurrency` tersimpan sebagai "XAU"
- [ ] Setelah `/price EUR`, ketik `/cot` → LastCurrency "EUR" tersedia untuk context
- [ ] `priceDetail()` dan `levelsDetail()` signature menerima userID
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-04-ux-audit-navigation-context-settings-putaran12.md` — Temuan 3
- `handler.go:1765` — saveLastCurrency() helper (sudah ada)
- `handler_price.go:14` — cmdPrice (perlu update priceDetail call)
- `handler_levels.go:14` — cmdLevels (perlu update levelsDetail call)
- `TASK-175` — cover COT/CTA/Quant (tidak cover Price/Levels)
- `domain/prefs.go:71` — LastCurrency field (sudah ada)
