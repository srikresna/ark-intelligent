# TASK-118: HTTP Client Factory dengan Connection Pooling

**Priority:** medium
**Type:** refactor
**Estimated:** M
**Area:** pkg
**Created by:** Research Agent
**Created at:** 2026-04-01 23:00 WIB
**Siklus:** Refactor

## Deskripsi
Multiple service packages membuat HTTP clients dengan hanya `Timeout` set, tanpa Transport configuration. Buat shared HTTP client factory di `pkg/` yang configure connection pooling, keepalive, dan limits.

## Konteks
- `fred/cache.go:68` — `http.Client{Timeout: 15s}` tanpa Transport
- `worldbank/client.go:68` — sama
- `sentiment/cboe.go:71` — sama
- Marketdata clients (bybit, coingecko, massive) — mirip
- Under load, tanpa pooling bisa resource exhaustion (too many TCP connections)
- Ref: `.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat `pkg/httpclient/factory.go` dengan function `New(opts ...Option) *http.Client`
- [ ] Default Transport configuration:
  - MaxIdleConns: 100
  - MaxConnsPerHost: 10
  - IdleConnTimeout: 90s
  - DisableKeepAlives: false
  - TLSHandshakeTimeout: 10s
- [ ] Option pattern untuk override: `WithTimeout(d)`, `WithMaxConnsPerHost(n)`, dll
- [ ] Migrate minimal 3 service clients ke factory (fred, worldbank, sentiment)
- [ ] Tidak ada behavior change — semua requests tetap berjalan identik
- [ ] Existing clients yang sudah custom (Telegram, Claude) TIDAK diganggu

## File yang Kemungkinan Diubah
- `pkg/httpclient/factory.go` (baru)
- `internal/service/fred/cache.go` (migrate)
- `internal/service/worldbank/client.go` (migrate)
- `internal/service/sentiment/cboe.go` (migrate)

## Referensi
- `.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`
- `.agents/TECH_REFACTOR_PLAN.md`
