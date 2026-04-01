# TASK-040: handler_cta.go Cleanup & Separation of Concerns

**Type:** refactor  
**Priority:** HIGH  
**Effort:** M (3-4h)  
**Phase:** Tech Refactor Phase 3 (TECH-003)  
**Assignee:** unassigned

---

## Problem

`internal/adapter/telegram/handler_cta.go` adalah 1,618 LOC yang mencampur tiga concern berbeda:
1. Command dispatch (`cmdCTA`, `cmdCTABT`, callback routing)
2. Business logic (indicator parsing, signal aggregation)
3. Formatting (output construction)

Pattern yang salah: handler langsung membangun output string alih-alih memanggil formatter.

## Solution

### Split menjadi:
```
internal/adapter/telegram/
├── handler_cta.go          ← hanya orchestration: get data → format → send (<300 LOC)

internal/adapter/telegram/format/     (hasil TASK-015 jika sudah selesai)
└── cta.go                  ← semua FormatCTA* functions pindah ke sini

Atau jika TASK-015 belum selesai:
internal/adapter/telegram/formatter.go  ← tambahkan FormatCTA* functions di sini
```

### State cache:
`ctaStateCache` struct (saat ini embedded di handler_cta.go) → bisa stay di handler_cta.go (internal, non-exported)

## Acceptance Criteria
- [ ] `handler_cta.go` < 400 LOC setelah refactor
- [ ] Handler functions hanya orchestrate (fetch → format → send) tanpa string building
- [ ] `go build ./...` sukses
- [ ] `go test ./...` tidak ada test yang break
- [ ] Behavior Telegram output IDENTIK (format text tidak berubah)

## Notes
- **NO behavior change** — pure refactor
- Cek TASK-015 dulu: jika formatter split sudah ada, taruh CTA format di sana
- Koordinasi dengan Dev yang mengerjakan TASK-015/TASK-016 untuk avoid conflicts
