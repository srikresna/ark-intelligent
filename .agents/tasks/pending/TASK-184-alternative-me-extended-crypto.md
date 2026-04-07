# TASK-184: Alternative.me Extended — Global Crypto + Ticker Data

**Priority:** medium
**Type:** data
**Estimated:** S
**Area:** internal/service/sentiment/

## Deskripsi

Extend existing Alternative.me integration (currently only Fear & Greed) dengan /v2/global/ dan /v2/ticker/ endpoints. BTC dominance trend dan volume-to-mcap ratio sebagai sinyal tambahan.

## Endpoints

- Global: `GET https://api.alternative.me/v2/global/`
- Ticker: `GET https://api.alternative.me/v2/ticker/?limit=20`

Auth: None.

## Data Points (Global)

- Total market cap USD
- BTC dominance %
- Active currencies count
- Active markets count

## Data Points (Ticker)

- Top 20 cryptos with 1h/24h/7d % changes
- Volume, market cap, circulating supply per coin
- Rank changes

## File Changes

- `internal/service/sentiment/sentiment.go` — Add FetchCryptoGlobal(), FetchTopCryptoTickers() methods
- `internal/service/sentiment/models.go` — Add CryptoGlobal, CryptoTicker types
- `internal/adapter/telegram/formatter.go` — Add crypto global section to /sentiment output

## Acceptance Criteria

- [ ] Fetch crypto global data (total mcap, BTC dominance)
- [ ] Fetch top 20 crypto tickers with % changes
- [ ] Compute volume-to-mcap ratio (liquidity health)
- [ ] BTC dominance trend: rising = BTC outperforming, falling = altseason
- [ ] Display in /sentiment and /cryptoalpha outputs
- [ ] Complement existing CoinGecko data (alternative source)
- [ ] Cache with 2h TTL
