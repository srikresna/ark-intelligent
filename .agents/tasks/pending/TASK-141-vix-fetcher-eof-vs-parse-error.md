# TASK-141: VIX Fetcher Error Handling — EOF vs Parse Errors

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/vix
**Created by:** Research Agent
**Created at:** 2026-04-02 04:00 WIB
**Siklus:** Refactor

## Deskripsi
VIX CSV parser di `fetcher.go:87, 148` menggunakan `err2` variable dan breaks on ANY error tanpa distinguish `io.EOF` dari actual parse errors. Malformed CSV data silently ignored.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Replace `err2` dengan standard `err` variable
- [ ] Explicitly handle `io.EOF` (normal end) vs other errors (log + return error)
- [ ] Malformed rows: log warning dengan row number, skip row, continue parsing
- [ ] Tidak silent-break pada parse errors

## File yang Kemungkinan Diubah
- `internal/service/vix/fetcher.go`

## Referensi
- `.agents/research/2026-04-02-04-tech-refactor-post-merge-audit.md`
