# TASK-140: Fix fmt.Sprintf Multipart Builder Pattern in api.go

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 04:00 WIB
**Siklus:** Refactor

## Deskripsi
Di `api.go:254`, `fmt.Sprintf` (function pointer) di-assign ke variable `writer`, lalu dipanggil sebagai `writer("--%s\r\n", boundary)`. Ini works by accident tapi semantically incorrect dan bisa confuse developers + linters.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Replace function pointer pattern dengan direct `fmt.Sprintf()` calls atau proper helper function
- [ ] Verify multipart form upload masih berfungsi (test: kirim foto/chart via bot)

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/api.go`

## Referensi
- `.agents/research/2026-04-02-04-tech-refactor-post-merge-audit.md`
