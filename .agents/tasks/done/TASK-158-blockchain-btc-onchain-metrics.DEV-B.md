# TASK-158: Blockchain.com BTC On-Chain Metrics Integration

**Priority:** medium
**Type:** data
**Estimated:** M
**Area:** internal/service/onchain/ (new)

## Deskripsi

Integrasi Blockchain.com free API untuk BTC on-chain metrics. Saat ini zero on-chain data di codebase. Data ini memberikan miner health, network demand, dan activity surge signals.

## Detail Teknis

- Charts API: `GET https://api.blockchain.info/charts/{chartName}?timespan=30days&format=json`
- Stats API: `GET https://api.blockchain.info/stats` (real-time aggregate)
- Auth: None
- Rate limit: ~1 req/sec

## Metrics Target

1. `hash-rate` — Network hash rate (TH/s) → miner health
2. `mempool-size` — Mempool size (bytes) → congestion
3. `transaction-fees` — Total fees (BTC) → demand
4. `difficulty` — Mining difficulty → security
5. `/stats` endpoint — n_tx, total_fees_btc, market_price_usd

## File Changes

- `internal/service/onchain/client.go` — NEW: Blockchain.com HTTP client
- `internal/service/onchain/models.go` — NEW: OnChainMetrics struct
- `internal/service/onchain/analyzer.go` — NEW: Trend detection (hash rate drop = miner capitulation, fee spike = activity surge)
- `internal/adapter/telegram/handler.go` — Add /onchain command routing
- `internal/adapter/telegram/formatter_onchain.go` — NEW: On-chain metrics formatting

## Acceptance Criteria

- [ ] Fetch 5 BTC on-chain metrics (hash rate, mempool, fees, difficulty, aggregate stats)
- [ ] Detect: miner capitulation (hash rate -10% 7d), fee surge (>2x 30d avg), mempool congestion (>100MB)
- [ ] /onchain command shows BTC network health dashboard
- [ ] Cache di BadgerDB, refresh every 4h
- [ ] Graceful degradation jika API down
- [ ] Unit tests untuk trend detection logic
