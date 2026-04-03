# TASK-207: CryptoCompare Exchange Volume Tracking

**Priority:** medium
**Type:** data
**Estimated:** M
**Area:** internal/service/marketdata/

## Deskripsi

Integrasi CryptoCompare free tier untuk exchange-specific volume tracking. CoinGecko aggregates volume; CryptoCompare shows per-exchange — useful for volume divergence detection.

## Endpoints

- Exchange daily volume: `GET https://min-api.cryptocompare.com/data/exchange/histoday?tsym=USD&e=Binance&limit=30`
- Top by volume: `GET https://min-api.cryptocompare.com/data/top/totalvolfull?limit=20&tsym=USD`
- Pair mapping: `GET https://min-api.cryptocompare.com/data/pair/mapping/exchange?e=Binance`

Auth: None for basic. Rate: 100K calls/month.

## File Changes

- `internal/service/marketdata/cryptocompare/client.go` — NEW: CryptoCompare client
- `internal/service/marketdata/cryptocompare/models.go` — NEW: ExchangeVolume, TopAsset types
- `internal/service/marketdata/cryptocompare/analyzer.go` — NEW: Volume divergence detection (Binance vs Coinbase shift)
- `internal/adapter/telegram/formatter.go` — Add exchange volume section to /cryptoalpha

## Acceptance Criteria

- [ ] Fetch daily volume for Binance, Coinbase, OKX, Bybit (top 4)
- [ ] Detect volume divergence: exchange gaining/losing market share
- [ ] Top 20 assets by volume with 24h change
- [ ] Display in /cryptoalpha output
- [ ] Cache with 4h TTL
