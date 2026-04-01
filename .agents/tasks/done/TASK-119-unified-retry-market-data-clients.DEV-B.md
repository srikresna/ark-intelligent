# TASK-119: Unified Retry-With-Backoff untuk Market Data API Clients

**Status:** done
**Completed by:** Dev-B
**Completed at:** 2026-04-02
**PR branch:** feature/task-119-unified-retry-market-data
**Priority:** medium
**Type:** refactor
**Estimated:** M
**Area:** pkg
**Created by:** Research Agent
**Created at:** 2026-04-01 23:00 WIB
**Siklus:** Refactor

## Deskripsi
Hanya Telegram dan Gemini clients yang punya retry-with-backoff. Market data fetchers (Bybit, CoinGecko, Massive/Polygon, FRED, WorldBank) fail immediately pada network error. Buat shared retry utility dan wire ke market data clients.

## Konteks
- `marketdata/bybit/client.go` — no retry
- `marketdata/coingecko/client.go` — no retry
- `marketdata/massive/client.go` — no retry
- `service/price/fetcher.go` — no retry
- `service/worldbank/client.go` — no retry
- Single network hiccup = data unavailable untuk user
- Ref: `.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat `pkg/retry/retry.go` dengan function `Do(ctx, fn, opts)`:
  - Exponential backoff: base 1s, factor 2x, max 30s
  - Max attempts: configurable, default 3
  - Jitter: random 0-500ms per attempt
  - Context-aware (use select pattern, bukan time.Sleep)
  - Retryable error detection: network errors, 429, 500-503
  - Non-retryable: 400, 401, 403, 404
- [ ] Wire retry ke minimal 3 market data clients (Bybit, CoinGecko, Massive)
- [ ] Log setiap retry attempt (warn level)
- [ ] Tidak retry jika context sudah cancelled

## File yang Kemungkinan Diubah
- `pkg/retry/retry.go` (baru)
- `internal/service/marketdata/bybit/client.go` (wire retry)
- `internal/service/marketdata/coingecko/client.go` (wire retry)
- `internal/service/marketdata/massive/client.go` (wire retry)

## Referensi
- `.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`
- `.agents/TECH_REFACTOR_PLAN.md`
