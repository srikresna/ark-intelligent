# TASK-149: Circuit Breaker Negative Duration + Race Fix

**Priority:** low
**Type:** fix
**Estimated:** S
**Area:** pkg/circuitbreaker
**Created by:** Research Agent
**Created at:** 2026-04-02 05:00 WIB
**Siklus:** BugHunt

## Deskripsi
Di `breaker.go:82-84`, error message computes `resetTimeout - time.Since(lastFailure)` yang bisa negative. Race condition: `lastFailure` bisa di-update antara `allowRequest()` check dan error formatting.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Clamp retry duration ke minimum 0: `max(0, resetTimeout - time.Since(lastFailure))`
- [ ] Snapshot lastFailure before allowRequest check untuk avoid race
- [ ] Format: "retry after ~Xs" (approximate) bukan exact duration

## File yang Kemungkinan Diubah
- `pkg/circuitbreaker/breaker.go`

## Referensi
- `.agents/research/2026-04-02-05-bug-hunting-gex-wyckoff-vix-circuitbreaker.md`
