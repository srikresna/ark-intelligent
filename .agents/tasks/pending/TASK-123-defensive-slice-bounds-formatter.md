# TASK-123: Defensive Slice Bounds di Formatter

**Priority:** low
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 00:00 WIB
**Siklus:** BugHunt

## Deskripsi
Di `formatter_quant.go:146`, `len(fxCurrencies) >= 4` lalu `topFX := fxCurrencies[:4]` — safe tapi fragile. Gunakan `min()` pattern untuk defensive bounds agar future refactoring tidak break.

## Konteks
- `formatter_quant.go:146` — boundary exact match
- Pattern ini mungkin ada di formatter lain juga
- Go 1.21+ punya built-in `min()` function
- Ref: `.agents/research/2026-04-01-24-bug-hunting-bounds-divzero-goroutine.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Replace `fxCurrencies[:4]` dengan `fxCurrencies[:min(4, len(fxCurrencies))]`
- [ ] Audit semua formatter files untuk pattern serupa (hardcoded slice indices)
- [ ] Khusus formatter.go (4489 LOC) — cek semua slice access

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/formatter_quant.go`
- `internal/adapter/telegram/formatter.go` (audit)

## Referensi
- `.agents/research/2026-04-01-24-bug-hunting-bounds-divzero-goroutine.md`
