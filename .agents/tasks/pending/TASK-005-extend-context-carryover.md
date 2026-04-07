# TASK-005: Extend Context Carry-Over ke VP, ICT, Wyckoff, SMC, Elliott, Session

**Priority:** MEDIUM
**Type:** UX Improvement
**Ref:** UX_AUDIT.md TASK-UX-007, research/2026-04-05-12-ux-audit-cycle1.md

---

## Problem

`getLastCurrency` / `saveLastCurrency` sudah diimplementasikan di:
✅ `/cot`, `/cta`, `/quant`, `/price`, `/levels`, `/bias`, `/seasonal`, `/signal`, `/history`

Tapi belum di: `/vp`, `/ict`, `/wyckoff`, `/smc`, `/elliott`, `/session`

Kalau user baru pakai `/cot EUR` lalu ketik `/vp`, bot tidak otomatis pakai EUR.
User harus selalu ketik pair manual di semua command tersebut.

---

## Acceptance Criteria

- [ ] `/vp` membaca `getLastCurrency` saat args kosong, menyimpan dengan `saveLastCurrency`
- [ ] `/ict` membaca `getLastCurrency` saat args kosong, menyimpan dengan `saveLastCurrency`
- [ ] `/wyckoff` membaca `getLastCurrency` saat args kosong, menyimpan dengan `saveLastCurrency`
- [ ] `/smc` membaca `getLastCurrency` saat args kosong, menyimpan dengan `saveLastCurrency`
- [ ] `/elliott` membaca `getLastCurrency` saat args kosong, menyimpan dengan `saveLastCurrency`
- [ ] `/session` membaca `getLastCurrency` saat args kosong, menyimpan dengan `saveLastCurrency`
- [ ] `go build ./...` bersih

---

## Implementation Pattern

Lihat `handler_signal_cmd.go` (baris 24-35) sebagai reference:

```go
func (h *Handler) cmdVP(ctx context.Context, chatID string, userID int64, args string) error {
    symbol := strings.ToUpper(strings.TrimSpace(args))
    if symbol == "" {
        if lc := h.getLastCurrency(ctx, userID); lc != "" {
            symbol = lc  // or convert to symbol format: lc+"USD"
        }
    }
    // ... existing logic ...
    h.saveLastCurrency(ctx, userID, currency) // save after successful parse
}
```

**Perhatian untuk VP, ICT, SMC, Wyckoff, Elliott:**
- Command ini menggunakan format `EURUSD` bukan `EUR`
- `getLastCurrency` menyimpan `EUR` format
- Perlu mapping: `lc + "USD"` atau `currencyToSymbol(lc)`
- Cek apakah `currencyToContractCode()` atau `currencyToSymbol()` helper sudah ada

**Perhatian untuk Session:**
- `/session` menggunakan currency format (EUR, GBP, dll) — lebih straightforward

---

## Files to Modify

- `internal/adapter/telegram/handler_vp.go` — fungsi `cmdVP`
- `internal/adapter/telegram/handler_ict.go` — fungsi `cmdICT`
- `internal/adapter/telegram/handler_wyckoff.go` — fungsi `cmdWyckoff`
- `internal/adapter/telegram/handler_smc.go` — fungsi `cmdSMC`
- `internal/adapter/telegram/handler_elliott.go` — fungsi `cmdElliott`
- `internal/adapter/telegram/handler_session.go` — fungsi `cmdSession`
