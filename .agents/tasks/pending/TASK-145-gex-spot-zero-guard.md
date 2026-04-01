# TASK-145: GEX Spot Price Zero Guard — Prevent Silent Data Corruption

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/service/gex
**Created by:** Research Agent
**Created at:** 2026-04-02 05:00 WIB
**Siklus:** BugHunt

## Deskripsi
Di `calculator.go:40`, jika Deribit API return no UnderlyingPrice dan no MarkPrice, spot=0. `multiplier = contractSize * spot * spot = 0`, menyebabkan SEMUA GEX values menjadi 0 tanpa error. User lihat flat GEX profile yang menyerupai data valid — ini data corruption.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Tambah `if spot <= 0 { return nil, fmt.Errorf("invalid spot price: %f", spot) }` sebelum multiplier calculation
- [ ] Audit semua tempat spot price di-source — pastikan fallback chain robust
- [ ] Log warning saat fallback dari UnderlyingPrice ke MarkPrice
- [ ] Error message user-friendly di formatter jika GEX calculation fails

## File yang Kemungkinan Diubah
- `internal/service/gex/calculator.go`
- `internal/adapter/telegram/handler_gex.go` (error handling)

## Referensi
- `.agents/research/2026-04-02-05-bug-hunting-gex-wyckoff-vix-circuitbreaker.md`
