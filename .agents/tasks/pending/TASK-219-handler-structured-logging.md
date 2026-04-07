# TASK-219: Structured Logging — Command Latency & Context in Handler Layer

**Priority:** LOW
**Type:** Tech Refactor / Observability
**Estimated:** M
**Area:** internal/adapter/telegram/handler.go dan handler_*.go files
**Ref:** TECH-013 in TECH_REFACTOR_PLAN.md
**Created by:** Research Agent
**Created at:** 2026-04-02 08:00 WIB
**Siklus:** 4 — Technical Refactor

## Problem

Handler layer tidak log context yang berguna untuk debugging production:
- Tidak ada log saat command mulai/selesai dengan latency
- Error log tidak selalu include: currency, userID, command name
- Tidak bisa distinguish "command timeout" vs "command not found" dari log saja

Contoh log yang ada sekarang:
```
{"level":"error","error":"context deadline exceeded","message":"cot fetch failed"}
```

Tidak tahu: siapa usernya, command apa, berapa lama, currency apa.

## Approach

Tambahkan structured fields ke log entries di handler commands. Pola standar:

```go
// Di awal setiap handler command:
start := time.Now()
log.Info().
    Str("cmd", "cot").
    Str("currency", currency).
    Int64("user", userID).
    Msg("command started")

// Di akhir (defer atau explicit):
log.Info().
    Str("cmd", "cot").
    Str("currency", currency).
    Int64("user", userID).
    Dur("latency", time.Since(start)).
    Msg("command done")

// Untuk error:
log.Error().
    Str("cmd", "cot").
    Str("currency", currency).
    Int64("user", userID).
    Dur("latency", time.Since(start)).
    Err(err).
    Msg("command failed")
```

## Scope

Focus pada 5 command handler dengan traffic tertinggi:
1. `cmdCOT` → add currency, userID, latency log
2. `cmdMacro` → add currency, userID, latency log
3. `cmdSentiment` → add userID, latency log
4. `cmdBias` → add currency, userID, latency log
5. `cmdRank` → add userID, latency log

Jangan ubah business logic sama sekali — hanya tambah log statements.

## File Changes

- `internal/adapter/telegram/handler.go` — add structured log fields ke 5 fungsi

## Acceptance Criteria

- [ ] 5 command handlers memiliki start/done/error log dengan Str("cmd"), Int64("user"), Dur("latency")
- [ ] Tidak ada behavior change — hanya log additions
- [ ] `go build ./... && go vet ./...` clean
- [ ] Log output readable: `{"cmd":"cot","currency":"EUR","user":123456,"latency":"1.2s","message":"command done"}`
- [ ] Branch: `refactor/handler-structured-logging`
