# TASK-044: Modernisasi interface{} → any (Go 1.22)

**Type:** refactor  
**Priority:** LOW  
**Effort:** S (1-2h)  
**Phase:** Tech Refactor Phase 1 — Quick Win  
**Assignee:** unassigned

---

## Problem

Codebase menggunakan Go 1.22 (dari `go.mod`) tapi masih ada 32 lokasi yang menggunakan `interface{}` instead of `any` (alias yang diperkenalkan Go 1.18).

Lokasi utama:
- `internal/service/ai/claude.go` — 7 lokasi (`map[string]interface{}`, `interface{}` dalam struct)
- `internal/service/ai/tool_executor.go` — 2 lokasi
- `internal/adapter/telegram/handler_quant.go` — 3 lokasi (JSON struct tags)
- `internal/adapter/telegram/bot.go` — 15 lokasi (`map[string]interface{}` untuk API params)

## Solution

Gantikan semua `interface{}` → `any` secara mechanical:
```bash
# Preview:
grep -rn "interface{}" --include="*.go" internal/ pkg/

# Bisa dilakukan dengan sed per file atau manual edit
```

Untuk `bot.go` yang menggunakan `map[string]interface{}` untuk Telegram params:
- Ganti ke `map[string]any` dulu (quick win, zero behavior change)
- Typed struct adalah pekerjaan terpisah (TASK-041 optional phase)

## Acceptance Criteria
- [ ] Semua `interface{}` → `any` di non-test files
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` bersih
- [ ] Zero behavior change (murni alias rename)

## Notes
- Quick win — cocok untuk warm-up task atau Dev baru ke codebase
- Test files boleh juga diupdate tapi tidak wajib
- **Tidak ada logic change** sama sekali — ini murni syntactic modernization
- Bisa dikerjakan bersamaan dengan task lain (no conflict risk karena beda lokasi)
