# TASK-001: Register /compare Command

**Priority:** HIGH (Critical Bug)
**Type:** Bug Fix
**Estimated effort:** XS (1 line change)
**Ref:** research/2026-04-05-12-ux-audit-cycle1.md

---

## Problem

`cmdCompare` di `handler_cot_compare.go` sudah fully implemented tapi tidak ada
`RegisterCommand("/compare", ...)` di mana pun. Command tidak accessible oleh user.

Verifikasi: grep -rn "RegisterCommand.*compare" . --include="*.go" → no results.

---

## Acceptance Criteria

- [ ] `/compare EUR GBP` dapat diakses user dan menampilkan side-by-side COT
- [ ] `/compare` terdaftar di `handler.go` block "Register all commands"
- [ ] `/compare` ditambahkan ke `relatedCommands` map di `keyboard_help.go` (optional, secondary)
- [ ] `go build ./...` bersih

---

## Implementation

**File:** `internal/adapter/telegram/handler.go`

Tambah di antara registrasi `/history` dan `/briefing` (sekitar baris 292-296):

```go
d.Bot.RegisterCommand("/compare", h.cmdCompare)   // COT side-by-side comparison
```

Tidak perlu `With*` pattern karena `cmdCompare` hanya menggunakan `h.cotRepo`
yang sudah di-inject via `HandlerDeps`.

**Optional (keyboard_help.go):** Tambah entry di `relatedCommands`:
```go
"compare": {{Label: "📉 COT", Callback: "cot"}, {Label: "📈 Bias", Callback: "bias"}},
```

---

## Notes

- Tidak ada perubahan logic, hanya wiring
- Tidak ada dependencies baru
- Test: kirim `/compare EUR GBP` ke bot
