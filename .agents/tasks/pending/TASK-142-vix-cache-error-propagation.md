# TASK-142: VIX Cache Error Propagation Fix

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/vix
**Created by:** Research Agent
**Created at:** 2026-04-02 04:00 WIB
**Siklus:** Refactor

## Deskripsi
VIX cache di `cache.go:36-42` selalu return `nil` error meskipun FetchTermStructure gagal. Caller tidak bisa distinguish network timeout vs invalid data. Error contract broken.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Return actual error dari cache saat fetch fails (jangan mask dengan nil)
- [ ] `Available: false` + `Error` field tetap sebagai diagnostics, tapi juga propagate error ke caller
- [ ] Callers yang consume VIX data harus handle error (audit)
- [ ] Fallback: jika cache punya stale data, return stale data + warning, bukan error

## File yang Kemungkinan Diubah
- `internal/service/vix/cache.go`
- Callers: audit siapa yang call GetCachedTermStructure

## Referensi
- `.agents/research/2026-04-02-04-tech-refactor-post-merge-audit.md`
