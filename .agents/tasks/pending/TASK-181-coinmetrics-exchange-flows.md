# TASK-181: CoinMetrics Community API — Exchange Flow Tracking

**Priority:** high
**Type:** data
**Estimated:** M
**Area:** internal/service/onchain/ (extend TASK-158)

## Deskripsi

Integrasi CoinMetrics community API untuk exchange flow data. Key differentiator dari Blockchain.com (TASK-158): FlowInExNtv/FlowOutExNtv = whale accumulation/distribution signal.

## Endpoint

`GET https://community-api.coinmetrics.io/v4/timeseries/asset-metrics`
Params: `assets=btc,eth&metrics=FlowInExNtv,FlowOutExNtv,AdrActCnt,TxCnt&frequency=1d&page_size=30`

Auth: None (community tier).

## File Changes

- `internal/service/onchain/coinmetrics.go` — NEW: CoinMetrics client
- `internal/service/onchain/models.go` — Add ExchangeFlow, ActiveAddresses types
- `internal/service/onchain/analyzer.go` — Add flow analysis: net flow direction, accumulation/distribution phase
- `internal/adapter/telegram/formatter_onchain.go` — Add exchange flow section

## Acceptance Criteria

- [ ] Fetch BTC + ETH exchange flows (in/out) for last 30 days
- [ ] Compute net flow: negative = coins leaving exchanges (accumulation)
- [ ] Detect: 3-day net outflow streak = strong accumulation signal
- [ ] Detect: large single-day inflow spike = potential sell pressure
- [ ] Active address trend as network health indicator
- [ ] Display in /onchain command alongside TASK-158 metrics
- [ ] Cache with 6h TTL
