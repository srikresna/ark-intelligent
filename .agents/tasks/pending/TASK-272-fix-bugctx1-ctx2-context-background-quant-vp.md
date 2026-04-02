# TASK-272: Fix BUG-CTX1+CTX2 — Ganti context.Background() dengan Parent ctx di handler_quant dan handler_vp

**Priority:** medium
**Type:** bugfix
**Estimated:** XS
**Area:** internal/adapter/telegram/handler_quant.go, internal/adapter/telegram/handler_vp.go
**Created by:** Research Agent
**Created at:** 2026-04-02 24:00 WIB

## Deskripsi

Dua fungsi menggunakan `context.Background()` saat membuat timeout context untuk Python subprocess, alih-alih menggunakan parent `ctx`. Ini menyebabkan subprocess tidak bisa di-cancel ketika request dibatalkan (misalnya user disconnect).

**Lokasi bug:**
1. `handler_quant.go:448` — `context.WithTimeout(context.Background(), 60*time.Second)`
2. `handler_vp.go:422` — `context.WithTimeout(context.Background(), 90*time.Second)`

Referensi yang **benar** sudah ada di `handler_cta.go:714`:
```go
cmdCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
```

## File yang Harus Diubah

### 1. internal/adapter/telegram/handler_quant.go:448

**Sebelum:**
```go
cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
```

**Sesudah:**
```go
cmdCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
```

Verifikasi bahwa `ctx` sudah available di scope fungsi tersebut (pastikan fungsi menerima `ctx context.Context` sebagai parameter).

### 2. internal/adapter/telegram/handler_vp.go:422

**Sebelum:**
```go
cmdCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
```

**Sesudah:**
```go
cmdCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
```

Verifikasi bahwa `ctx` sudah available di scope fungsi `runVPEngine`. Jika `runVPEngine` belum menerima `ctx`, tambahkan parameter `ctx context.Context` dan update semua caller-nya.

## Verifikasi

```bash
go build ./...
go vet ./...
```

## Acceptance Criteria

- [ ] `handler_quant.go:448` menggunakan `context.WithTimeout(ctx, 60*time.Second)`
- [ ] `handler_vp.go:422` menggunakan `context.WithTimeout(ctx, 90*time.Second)`
- [ ] Tidak ada `context.WithTimeout(context.Background()` yang tersisa di kedua file ini
- [ ] Semua fungsi yang terdampak (jika ada chain caller) sudah menerima dan meneruskan ctx
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean

## Referensi

- `.agents/research/2026-04-02-24-bug-hunt-wyckoff-context-ict-putaran16.md` — BUG-CTX1, BUG-CTX2
- `internal/adapter/telegram/handler_quant.go:448` — quant engine context
- `internal/adapter/telegram/handler_vp.go:422` — VP engine context
- `internal/adapter/telegram/handler_cta.go:714` — referensi fix yang benar
- `.agents/TECH_REFACTOR_PLAN.md` — TECH-008 Context Propagation
