# TASK-146: Deribit BookSummary Expired Options Filter

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/marketdata/deribit
**Created by:** Research Agent
**Created at:** 2026-04-02 05:00 WIB
**Siklus:** BugHunt

## Deskripsi
`GetBookSummary()` di `client.go:93-96` TIDAK filter expired options, sementara `GetInstruments()` filter `expired=false`. Pada expiry day, expired options masuk ke analysis dan instrument map mismatch.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Tambah filter expired options di GetBookSummary (client-side filter berdasarkan expiry date, atau tambah API param jika supported)
- [ ] Pastikan instrumentMap dan summaryMap konsisten — no orphan entries
- [ ] Test: pada hari expiry, GEX calculation masih akurat

## File yang Kemungkinan Diubah
- `internal/service/marketdata/deribit/client.go`

## Referensi
- `.agents/research/2026-04-02-05-bug-hunting-gex-wyckoff-vix-circuitbreaker.md`
