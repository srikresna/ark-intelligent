# TASK-010: Expose COT Seasonal via /cotseasonal Command

**Priority:** HIGH
**Type:** New Feature (expose existing service)
**Estimated effort:** M (half-to-one day)
**Ref:** research/2026-04-06-11-feature-deep-dive-siklus3.md

---

## Context

`internal/service/cot/seasonal.go` sudah memiliki implementasi lengkap:
- `SeasonalEngine` dengan `Analyze(ctx, contract)` dan `AnalyzeAll(ctx)`
- Menghitung seasonal COT positioning pattern per currency per ISO week
- Output: `COTSeasonalResult` dengan `CurrentNet`, `SeasonalAvg`, `Deviation`, Z-score, `SeasonalBias`

Namun TIDAK ADA command handler, formatter, atau RegisterCommand. Ini dead code —
functionality lengkap yang tidak pernah bisa diakses user.

---

## Implementation

### New file: `internal/adapter/telegram/handler_cot_seasonal.go`

Pattern sama dengan `handler_cot_compare.go` (minimal dependency, simple handler):

```go
// cmdCOTSeasonal handles /cotseasonal [currency] — COT seasonal positioning analysis.
func (h *Handler) cmdCOTSeasonal(ctx context.Context, chatID string, userID int64, args string) error {
    h.bot.SendTyping(ctx, chatID)
    // If no args, analyze all currencies
    // If args provided, analyze single currency
    // Use cot.NewSeasonalEngine(h.cotRepo).Analyze(ctx, contract)
    // Format and send result
}
```

### New file: `internal/adapter/telegram/formatter_cot_seasonal.go`

Format `COTSeasonalResult` as HTML for Telegram:

```
📅 <b>COT Seasonal — EUR</b>

Minggu saat ini: <b>Week 14</b>
Net saat ini: <code>+42,350</code>
Seasonal avg (5yr): <code>+38,200</code>
Deviasi: <code>+4,150</code> (+1.2σ)

Bias Seasonal: 🟢 SEASONALLY BULLISH
Tren musiman: ↑ Biasanya naik pada periode ini

<i>Berdasarkan 5+ tahun historis COT</i>
```

### Register in `internal/adapter/telegram/handler.go`

Add in the "Register all commands" section:

```go
d.Bot.RegisterCommand("/cotseasonal", h.cmdCOTSeasonal)
d.Bot.RegisterCommand("/cs", h.cmdCOTSeasonal) // short alias
```

### Dependencies

`handler.cotRepo` sudah ada (digunakan oleh `/cot`, `/bias`).
`SeasonalEngine` hanya butuh `cotRepo` — tidak ada dependency baru.

---

## Acceptance Criteria

- [ ] `internal/adapter/telegram/handler_cot_seasonal.go` ada dengan `cmdCOTSeasonal()`
- [ ] `internal/adapter/telegram/formatter_cot_seasonal.go` ada dengan formatter HTML
- [ ] `/cotseasonal` dan `/cs` teregistrasi di `handler.go`
- [ ] `/cotseasonal` tanpa args menampilkan semua currencies (multi-message atau ringkasan)
- [ ] `/cotseasonal EUR` menampilkan seasonal analysis untuk EUR saja
- [ ] Graceful degradation: pesan error informatif jika data COT history < 8 minggu
- [ ] `go build ./...` bersih
- [ ] Unit test untuk formatter (minimal 1 test)
