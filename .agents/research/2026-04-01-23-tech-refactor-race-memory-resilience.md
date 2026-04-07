# Research Report: Tech Refactor Siklus 4 Putaran 2 — Race Conditions, Memory Leaks, API Resilience

**Tanggal:** 2026-04-01 23:00 WIB
**Fokus:** Technical Refactor & Tech Debt (Siklus 4, Putaran 2)
**Siklus:** 4/5

---

## Ringkasan

Audit mendalam terhadap concurrency safety, memory management, dan API resilience. Ditemukan 20 tech debt items baru. 5 dipilih untuk task berdasarkan severity dan impact tertinggi.

---

## Temuan Kritis

### 1. HIGH: Unbounded sync.Map Growth di middleware.go (TECH-017/021)
- **File:** `internal/adapter/telegram/middleware.go:27, 54-57`
- Per-user mutexes di `sync.Map` TIDAK PERNAH di-evict
- Setiap user baru = entry baru yang persist selamanya
- Long-running deployment = unbounded memory growth
- Comment di line 435-437 explicitly notes ini tapi tidak fix

### 2. HIGH: chartPath Temp File Leak (TECH-019/020)
- **Files:** `handler_quant.go:442-443`, `handler_vp.go:410-411`
- `defer os.Remove(inputPath)` dan `defer os.Remove(outputPath)` ada, tapi `chartPath` TIDAK
- Setiap kali subprocess gagal, PNG file tertinggal di /tmp
- Mirip TASK-071 tapi ini di file berbeda (quant & VP handlers)

### 3. HIGH: Blocking Sleep Ignores Context di gemini.go (TECH-023)
- **File:** `internal/service/ai/gemini.go:76, 120`
- Retry logic pakai `time.Sleep(backoff)` bukan `select { case <-time.After(): case <-ctx.Done(): }`
- Jika caller cancel context, function tetap block selama full sleep duration
- Wasted compute dan user harus tunggu timeout

### 4. MEDIUM: No HTTP Connection Pooling (TECH-025)
- **Files:** `fred/cache.go:68`, `worldbank/client.go:68`, `sentiment/cboe.go:71`
- HTTP clients hanya set Timeout, tanpa Transport configuration
- Tidak ada MaxIdleConns, MaxConnsPerHost, IdleConnTimeout
- Under load bisa resource exhaustion (too many connections)

### 5. MEDIUM: No Retry for Market Data APIs (TECH-026)
- **Files:** `marketdata/bybit/client.go`, `marketdata/coingecko/client.go`, `marketdata/massive/client.go`, `price/fetcher.go`, `worldbank/client.go`
- Hanya Telegram dan Gemini yang punya retry-with-backoff
- Market data fetchers fail immediately pada network error
- Single hiccup = data unavailable

### Temuan Lainnya (tidak dijadikan task sekarang)
- TECH-016: aiCooldown map race condition (medium, edge case)
- TECH-022/034: Dead code subprocess in handler_quant.go (low)
- TECH-024: context.Background() di handler subprocess (medium)
- TECH-027: No singleflight for cache (TASK-074 sudah ada untuk sentiment)
- TECH-028: Tight coupling di handler.go (large effort, covered by TASK-016)
- TECH-029: Duplicate subprocess pattern (low, nice-to-have)
- TECH-030/031: Global cache variables (low, works fine)
- TECH-035: No circuit breaker for WorldBank (pkg/circuitbreaker/ exists, just not wired)

---

## Task Recommendations (Top 5)

1. **TASK-115**: Bounded LRU user mutex di middleware — fix memory leak [HIGH]
2. **TASK-116**: chartPath defer cleanup di quant & VP handlers [HIGH]
3. **TASK-117**: Context-aware retry sleep di gemini.go [HIGH]
4. **TASK-118**: HTTP client factory dengan connection pooling [MEDIUM]
5. **TASK-119**: Unified retry-with-backoff untuk market data API clients [MEDIUM]
