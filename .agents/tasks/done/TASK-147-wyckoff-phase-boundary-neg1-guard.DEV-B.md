# TASK-147: Wyckoff Phase Boundary -1 Guard

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/wyckoff
**Created by:** Research Agent
**Created at:** 2026-04-02 05:00 WIB
**Siklus:** BugHunt

## Deskripsi
Di `phase.go:111-112`, jika AR_D event tidak ditemukan (`arDistIdx = -1`), Phase A end boundary jadi -1. `eventsInRange(events, 0, -1)` includes ALL events karena `e.BarIndex > -1` selalu true.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Guard: jika arDistIdx == -1, skip distribution phase construction atau use len(bars)-1 sebagai fallback
- [ ] `eventsInRange()` harus handle negative boundaries gracefully (return empty)
- [ ] Audit semua idx variables di phase.go — pastikan -1 case handled
- [ ] Test: Wyckoff analysis pada flat/ranging market tanpa clear AR events

## File yang Kemungkinan Diubah
- `internal/service/wyckoff/phase.go`

## Referensi
- `.agents/research/2026-04-02-05-bug-hunting-gex-wyckoff-vix-circuitbreaker.md`
