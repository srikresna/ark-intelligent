# TASK-217: Context Propagation Fix — handler_cta, handler_quant, handler_vp

**Priority:** MEDIUM
**Type:** Bug Fix / Tech Refactor
**Estimated:** S
**Area:** internal/adapter/telegram/handler_cta.go, handler_quant.go, handler_vp.go
**Ref:** TECH-008 in TECH_REFACTOR_PLAN.md
**Created by:** Research Agent
**Created at:** 2026-04-02 08:00 WIB
**Siklus:** 4 — Technical Refactor

## Problem

Tiga handler utama membuat `context.Background()` baru di tengah request lifecycle, membuang context dari caller (yang sudah punya deadline/cancellation dari Telegram polling):

```go
// handler_cta.go:581
ctx := context.Background()   // ← WRONG: caller ctx sudah ada

// handler_quant.go:484
ctx := context.Background()   // ← WRONG

// handler_vp.go:422
cmdCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
// ← Seharusnya: context.WithTimeout(ctx, 90*time.Second)

// handler_quant.go:448
cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
// ← Seharusnya: context.WithTimeout(ctx, 60*time.Second)
```

**Dampak:** Jika Telegram connection drop/user disconnect, request tetap jalan sampai selesai karena pakai context.Background() yang tidak bisa di-cancel.

## Approach

Untuk setiap titik:
1. Cek apakah function param sudah ada `ctx context.Context`
2. Ganti `context.Background()` dengan `ctx`
3. Untuk `context.WithTimeout(context.Background(), X)` → ganti base-nya ke `ctx`
4. Pastikan `ctx` dari outer function dipass ke downstream calls

Urutan fix:
- `handler_vp.go:422` — cmdCtx base context
- `handler_quant.go:448,484` — dua titik di satu file
- `handler_cta.go:581` — ctx assignment

## File Changes

- `internal/adapter/telegram/handler_vp.go` — fix line ~422
- `internal/adapter/telegram/handler_quant.go` — fix lines ~448, ~484
- `internal/adapter/telegram/handler_cta.go` — fix line ~581

## Acceptance Criteria

- [ ] 0 `context.Background()` di ketiga file untuk in-request context creation
- [ ] `context.WithTimeout` menggunakan `ctx` dari caller sebagai base
- [ ] `go build ./... && go vet ./...` clean
- [ ] No behavior change untuk happy path
- [ ] Branch: `fix/context-propagation-handlers`
