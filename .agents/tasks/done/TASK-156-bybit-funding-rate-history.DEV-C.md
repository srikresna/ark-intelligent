# TASK-156: Bybit Funding Rate Historical Tracking

**Priority:** high
**Type:** data
**Estimated:** M
**Area:** internal/service/marketdata/bybit/

## Deskripsi

Extend existing Bybit client untuk fetch funding rate history. Endpoint sudah public (verified), client sudah ada, tinggal tambah method + storage.

## Detail Teknis

- Endpoint: `GET https://api.bybit.com/v5/market/funding/history`
- Params: `category=linear`, `symbol=BTCUSDT`, `limit=200`
- Auth: None (public)
- Response: `{fundingRate, fundingRateTimestamp}[]`
- Rate limit: ~10 req/sec

## Symbols Target

- BTCUSDT, ETHUSDT, SOLUSDT, XRPUSDT (top 4 perps by volume)

## File Changes

- `internal/service/marketdata/bybit/client.go` — Add `GetFundingHistory(symbol, limit)` method
- `internal/service/marketdata/bybit/models.go` — Add `FundingRate` struct
- `internal/service/microstructure/engine.go` — Integrate funding rate into microstructure analysis
- `internal/adapter/telegram/formatter.go` — Add funding rate section to crypto analysis output

## Acceptance Criteria

- [ ] Fetch 200 historical funding rates per symbol
- [ ] Compute: current rate, 7d average, 30d average, extreme percentile
- [ ] Detect funding regime: positive bias (bullish crowd), negative (bearish), neutral
- [ ] Display funding rate in /micro output
- [ ] Cache funding data di BadgerDB, refresh every 8h (funding settles 3x/day)
- [ ] Handle Bybit API errors gracefully

---
## Completion — DEV-C
**Agent:** Dev-C
**Completed:** 2026-04-02 03:40 WIB
**Branch:** feat/TASK-156-bybit-funding-rate-history
**PR:** #207
**Status:** done

### What was done
- Added GetFundingHistory() to Bybit V5 client (200 records, public endpoint)
- Added FundingRateStats with current, 7d/30d avg, min/max, regime, percentile
- Integrated into microstructure Analyze() for automatic funding history fetch
- Enhanced /cryptoalpha formatter with detailed stats display
- Enhanced Indonesian interpretation with regime and percentile awareness
