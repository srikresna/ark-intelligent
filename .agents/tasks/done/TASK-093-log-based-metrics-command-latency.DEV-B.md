# TASK-093: Log-Based Metrics — Command Latency & Error Rate (TECH-015 minimal)

**Priority:** MEDIUM
**Type:** Tech Refactor / Observability
**Ref:** TECH-015 in TECH_REFACTOR_PLAN.md
**Branch target:** dev-b atau dev-c
**Estimated size:** Medium (M) — 100-200 LOC
**Created by:** Research Agent
**Created at:** 2026-04-01 15:30 WIB
**Siklus:** 4 — Technical Refactor

---

## Problem

Saat ini tidak ada visibility ke performa production bot:
- Tidak tahu command mana yang slow (>/5 detik response?)
- Tidak tahu error rate per command
- Tidak tahu API call count (rate limit risk tidak terdeteksi)
- Saat ada incident, debugging hanya dari Telegram error messages

TECH-015 menargetkan Prometheus metrics, tapi itu kompleks.
**Langkah pertama yang pragmatis: log-based metrics** yang bisa diparse dari log output.

---

## Solusi: Structured Latency Logging di Handler Middleware

### 1. Buat pkg/metrics/latency.go (log-based)

```go
package metrics

import (
    "time"
    "github.com/rs/zerolog/log"
)

// RecordCommand logs command execution metrics in a structured way
// that can be parsed by log analysis tools.
func RecordCommand(command string, userID int64, duration time.Duration, err error) {
    event := log.Info().
        Str("metric", "command_exec").
        Str("command", command).
        Int64("user_id", userID).
        Dur("latency_ms", duration).
        Bool("success", err == nil)
    
    if err != nil {
        event = event.Err(err)
    }
    
    if duration > 5*time.Second {
        log.Warn().
            Str("metric", "slow_command").
            Str("command", command).
            Dur("latency_ms", duration).
            Msg("command exceeded 5s threshold")
    }
    
    event.Msg("command_metrics")
}
```

### 2. Integrasikan di middleware.go

Di `internal/adapter/telegram/middleware.go`, tambahkan timing ke existing middleware chain:

```go
func (m *Middleware) WithMetrics(next HandlerFunc) HandlerFunc {
    return func(ctx context.Context, b *bot.Bot, update *models.Update) {
        start := time.Now()
        command := extractCommand(update)
        userID := extractUserID(update)
        
        defer func() {
            metrics.RecordCommand(command, userID, time.Since(start), nil)
        }()
        
        next(ctx, b, update)
    }
}
```

### 3. Tambahkan API call counter di key services

Di FRED cache, COT fetcher — log saat fetch ke external API:
```go
log.Info().
    Str("metric", "api_call").
    Str("service", "fred").
    Str("series", seriesID).
    Msg("external_api_call")
```

---

## Acceptance Criteria

- [ ] Buat `pkg/metrics/latency.go` dengan `RecordCommand()` function
- [ ] Integrasikan di middleware.go (atau handler dispatch point)
- [ ] Setiap command execution menghasilkan structured log dengan field `metric`, `command`, `latency_ms`, `success`
- [ ] Slow command (>5s) menghasilkan Warn level log
- [ ] `go build ./...` dan `go vet ./...` clean
- [ ] `go test ./...` semua test lama tetap pass
- [ ] Dokumentasikan format log di komentar (untuk future Prometheus migration)

---

## Catatan

- Ini adalah stepping stone menuju TECH-015 full Prometheus
- Log format harus machine-parseable (zerolog JSON output)
- Jangan add Prometheus library dulu — terlalu besar perubahan untuk satu PR
- Future: saat traffic meningkat, log → Prometheus migration akan lebih mudah karena pola sudah ada
