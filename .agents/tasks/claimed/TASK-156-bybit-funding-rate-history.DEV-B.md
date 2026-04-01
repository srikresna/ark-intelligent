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
