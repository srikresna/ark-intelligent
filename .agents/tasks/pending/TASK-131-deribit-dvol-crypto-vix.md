# TASK-131: Deribit DVOL Index (Crypto VIX Equivalent)

**Priority:** high
**Type:** data
**Estimated:** M
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-02 02:00 WIB
**Siklus:** Data

## Deskripsi
Integrasikan Deribit DVOL (Deribit Volatility Index) sebagai crypto-native volatility indicator. DVOL = crypto VIX equivalent. Show alongside existing CBOE VIX untuk cross-asset vol comparison.

## Konteks
- Endpoint: `public/get_volatility_index_data` dengan resolution 1s/60s/1h/12h/1D
- Currencies: BTC, ETH (mungkin SOL via USDC)
- Deribit client sudah ada di GEX engine
- CBOE VIX sudah ada di sentiment service
- Tambahkan DVOL sebagai pelengkap — "VIX naik tapi DVOL turun" = divergence signal
- Ref: `.agents/research/2026-04-02-02-data-deribit-expanded-tradingeconomics-finviz.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Fetch DVOL candles (OHLC) via `get_volatility_index_data` untuk BTC dan ETH
- [ ] Juga fetch `get_historical_volatility` untuk realized vol comparison (IV vs HV)
- [ ] Cache di BadgerDB (TTL 1h)
- [ ] Expose via `/sentiment` command — tambah section "Crypto Volatility"
- [ ] Show: DVOL current, 24h change, IV-HV spread, comparison vs CBOE VIX
- [ ] Alert: jika DVOL spike >20% dalam 24h (volatility surge)

## File yang Kemungkinan Diubah
- `internal/service/gex/deribit_client.go` (tambah DVOL + HV endpoints)
- `internal/service/sentiment/` (integrate DVOL sebagai vol indicator)
- `internal/adapter/telegram/formatter.go` (sentiment section — add crypto vol)

## Referensi
- `.agents/research/2026-04-02-02-data-deribit-expanded-tradingeconomics-finviz.md`
- Deribit API: https://docs.deribit.com/#public-get_volatility_index_data
