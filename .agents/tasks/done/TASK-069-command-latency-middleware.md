# TASK-069: Command Latency Tracking via Middleware (TECH-015)

**Priority:** LOW  
**Type:** Tech Refactor / Observability  
**Ref:** TECH-015 in TECH_REFACTOR_PLAN.md  
**Branch target:** dev-c  
**Estimated size:** Small (30-50 LOC)

---

## Problem

Tidak ada cara untuk mengukur berapa lama setiap Telegram command butuh waktu untuk respond.
Jika `/cot` tiba-tiba butuh 15 detik (vs biasanya 3 detik), tidak ada alerting atau log metric.

Middleware sudah ada (`internal/adapter/telegram/middleware.go` — 20 fungsi) namun tidak include timing.

---

## Solution

Tambah timing wrapper di existing middleware chain. Gunakan log-based metrics (zero external dependency).

```go
// Di middleware.go atau handler dispatch di bot.go

func (b *Bot) withLatencyLog(command string, handler func(ctx context.Context, msg *tgbotapi.Message)) func(ctx context.Context, msg *tgbotapi.Message) {
    return func(ctx context.Context, msg *tgbotapi.Message) {
        start := time.Now()
        handler(ctx, msg)
        elapsed := time.Since(start)
        
        // Log-based metric — queryable dari log aggregator
        log.Info().
            Str("command", command).
            Dur("latency_ms", elapsed).
            Int64("user_id", msg.From.ID).
            Msg("command handled")
        
        // Warn jika > threshold
        if elapsed > 10*time.Second {
            log.Warn().
                Str("command", command).
                Dur("latency_ms", elapsed).
                Msg("slow command response")
        }
    }
}
```

---

## Implementation Steps

1. Tambah `withLatencyLog` wrapper di `middleware.go`
2. Di command registration (cek `bot.go` atau `core.go` handler router), wrap handler dengan `withLatencyLog`
3. Threshold slow warning: 10 detik (configurable via constant)

---

## Acceptance Criteria

- [ ] `withLatencyLog` atau equivalent middleware di middleware.go
- [ ] Semua command handlers di-wrap (atau minimal: `/cot`, `/macro`, `/sentiment`, `/price`, `/outlook`)
- [ ] Log output: `{"command":"/cot","latency_ms":2341,"level":"info","message":"command handled"}`
- [ ] Slow warning (>10s) muncul di log
- [ ] `go build ./...` clean

---

## Notes

- Zero external dependency — hanya pakai zerolog yang sudah ada
- JANGAN buat Prometheus metrics (dependency baru) — gunakan log-based saja untuk sekarang
- Ini bisa jadi dasar untuk TECH-015 Phase 2 (Prometheus) di masa depan
- Lihat TECH_REFACTOR_PLAN.md TECH-015 untuk konteks
