# TASK-144: Complete interface{} → any Modernization Cleanup

**Priority:** low
**Type:** refactor
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 04:00 WIB
**Siklus:** Refactor

## Deskripsi
`api.go:212` masih pakai `map[string]interface{}` sementara rest of file sudah `map[string]any`. Sisa dari TASK-044 yang terlewat. Cleanup untuk konsistensi.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Replace `interface{}` dengan `any` di api.go:212
- [ ] Grep seluruh codebase untuk `interface{}` yang tersisa — fix semua
- [ ] Pastikan Go version di go.mod >= 1.18

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/api.go`
- Possibly other files found via grep

## Referensi
- `.agents/research/2026-04-02-04-tech-refactor-post-merge-audit.md`
- TASK-044 (original modernization task)
