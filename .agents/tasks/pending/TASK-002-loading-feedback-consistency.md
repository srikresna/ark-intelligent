# TASK-002: Standardize Loading Feedback (SendTyping → SendLoading)

**Priority:** HIGH
**Type:** UX Improvement
**Ref:** UX_AUDIT.md TASK-UX-004, research/2026-04-05-12-ux-audit-cycle1.md

---

## Problem

11+ handlers masih menggunakan `SendTyping` saja alih-alih pola `SendLoading` +
`EditMessage`. `SendTyping` hanya menampilkan indikator selama ≤5 detik —
setelah itu user tidak mendapat feedback visual apapun kalau command butuh waktu lama.

Pattern yang benar (sudah dipakai di handler_ict.go, handler_alpha.go, dll):
```go
loadingID, _ := h.bot.SendLoading(ctx, chatID, "⏳ <command>... ⏳")
// ... proses ...
h.bot.EditMessage(ctx, chatID, loadingID, result)
// atau:
h.bot.EditWithKeyboard(ctx, chatID, loadingID, result, kb)
```

---

## Handlers yang Harus Diupdate

| File | Command | Action |
|------|---------|--------|
| `handler_price.go` | `/price` | Ganti SendTyping → SendLoading |
| `handler_carry.go` | `/carry` | Ganti SendTyping → SendLoading |
| `handler_bis.go` | `/bis` | Ganti SendTyping → SendLoading |
| `handler_onchain.go` | `/onchain` | Ganti SendTyping → SendLoading |
| `handler_briefing.go` | `/briefing` | Ganti SendTyping → SendLoading |
| `handler_levels.go` | `/levels` | Ganti SendTyping → SendLoading |
| `handler_scenario.go` | `/scenario` | Ganti SendTyping → SendLoading |
| `handler_defi.go` | `/defi` | Ganti SendTyping → SendLoading |
| `handler_vix_cmd.go` | `/vix` | Ganti SendTyping → SendLoading |
| `handler_regime.go` | `/regime` | Ganti SendTyping → SendLoading |
| `handler_cot_compare.go` | `/compare` | Ganti SendTyping → SendLoading |

---

## Acceptance Criteria

- [ ] Semua handler di list di atas menggunakan `SendLoading` + `EditMessage`/`EditWithKeyboard`
- [ ] Loading message text deskriptif (bukan hanya "⏳ Loading...")
  - Contoh: "💰 Mengambil data carry trades... ⏳"
  - Contoh: "🏦 Mengambil data BIS policy rates... ⏳"
- [ ] Error path juga menggunakan `EditMessage` (bukan kirim message baru)
- [ ] `go build ./...` bersih

---

## Implementation Notes

Pattern di `handler_orderflow.go` baris 57 hanya `SendTyping` lalu langsung return —
periksa apakah command ini cukup cepat (< 2s) sebelum mengubahnya.

Prioritas upgrade: `/briefing`, `/carry`, `/vix`, `/regime` (semua heavy computations).
