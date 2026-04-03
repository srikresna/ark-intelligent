# TASK-180: DefiLlama Integration — TVL, DEX Volume, Stablecoin Supply

**Priority:** high
**Type:** data
**Estimated:** L
**Area:** internal/service/defi/ (new)

## Deskripsi

Integrasi DefiLlama free API untuk DeFi metrics. Zero DeFi data di codebase saat ini. TVL, DEX volume, dan stablecoin supply adalah 3 metrik terpenting untuk crypto institutional analysis.

## Endpoints

- Protocols TVL: `GET https://api.llama.fi/v2/protocols`
- Historical TVL: `GET https://api.llama.fi/v2/historicalChainTvl`
- DEX Volume: `GET https://api.llama.fi/overview/dexs`
- Stablecoins: `GET https://stablecoins.llama.fi/stablecoins`
- Yields: `GET https://yields.llama.fi/pools`

Auth: None. No key required.

## File Changes

- `internal/service/defi/client.go` — NEW: DefiLlama HTTP client
- `internal/service/defi/models.go` — NEW: TVLData, DEXVolume, StablecoinSupply types
- `internal/service/defi/analyzer.go` — NEW: TVL trend, stablecoin flow direction, yield ranking
- `internal/adapter/telegram/handler.go` — Add /defi command routing
- `internal/adapter/telegram/formatter_defi.go` — NEW: DeFi dashboard formatting

## Acceptance Criteria

- [ ] Fetch total DeFi TVL + top 10 protocols by TVL
- [ ] Fetch 24h DEX volume (total + by chain: Ethereum, Solana, Arbitrum)
- [ ] Fetch stablecoin total supply + 7d change (USDT, USDC, DAI)
- [ ] /defi command shows DeFi health dashboard
- [ ] Detect: TVL drop >5% in 24h = risk-off signal
- [ ] Detect: stablecoin supply growth = incoming liquidity
- [ ] Cache di BadgerDB, refresh every 4h
- [ ] Unit tests for trend detection
