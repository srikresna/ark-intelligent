# TASK-148: VIX SlopePct Zero Guard + NaN Check

**Priority:** low
**Type:** fix
**Estimated:** S
**Area:** internal/service/vix
**Created by:** Research Agent
**Created at:** 2026-04-02 05:00 WIB
**Siklus:** BugHunt

## Deskripsi
Di `fetcher.go:269`, guard `ts.M1 > 0` ada tapi jika CSV data malformed dan M1=0 lolos parsing, SlopePct bisa NaN/Inf. Tambah defensive guard dan NaN check.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Double-check: M1 parsing dari CSV — apa yang terjadi jika field empty atau non-numeric?
- [ ] Tambah explicit `if M1 == 0 { SlopePct = 0; Regime = "FLAT" }` guard
- [ ] Tambah `math.IsNaN(SlopePct)` check setelah calculation
- [ ] Log warning jika CSV parse menghasilkan unexpected values

## File yang Kemungkinan Diubah
- `internal/service/vix/fetcher.go`

## Referensi
- `.agents/research/2026-04-02-05-bug-hunting-gex-wyckoff-vix-circuitbreaker.md`
