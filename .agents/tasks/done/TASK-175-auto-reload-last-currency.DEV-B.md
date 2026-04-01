# TASK-175: Auto-Reload Last Currency When No Args

**Priority:** high
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram/

## Deskripsi

Gunakan `UserPrefs.LastCurrency` saat user ketik command tanpa argument. Contoh: `/cot` tanpa args → load `/cot EUR` jika last currency EUR. Tunjukkan hint: "🔄 Loading EUR (last viewed)... or type `/cot GBP` to switch".

## File Changes

- `internal/adapter/telegram/handler.go` — cmdCOT: check no args → use prefs.LastCurrency
- `internal/adapter/telegram/handler_cta.go` — cmdCTA: same pattern
- `internal/adapter/telegram/handler_quant.go` — cmdQuant: same pattern
- `internal/adapter/telegram/handler.go` — After successful command, update prefs.LastCurrency

## Pattern

```go
func (h *Handler) cmdCOT(ctx, msg) {
    args := parseArgs(msg.Text)
    currency := args.Currency
    if currency == "" {
        prefs := h.prefsRepo.Get(msg.From.ID)
        if prefs.LastCurrency != "" {
            currency = prefs.LastCurrency
        } else {
            // show currency selector keyboard
            return
        }
    }
    // ... execute command ...
    // update last currency
    h.prefsRepo.SetLastCurrency(msg.From.ID, currency)
}
```

## Acceptance Criteria

- [ ] /cot tanpa args → load last currency jika ada
- [ ] /cta tanpa args → load last currency jika ada
- [ ] /quant tanpa args → load last currency jika ada
- [ ] Last currency updated setelah setiap successful command
- [ ] First-time user (no last currency) → show selector keyboard as usual
- [ ] Hint message shown: "Loading X (last viewed)"
