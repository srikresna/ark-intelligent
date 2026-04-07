# Research Report: Data Siklus 2 Putaran 6
# CBOE Index Suite, SEC EDGAR 13F, CryptoCompare, Open Exchange Rates
**Date:** 2026-04-02 17:00 WIB
**Siklus:** 2/5 (Data & Integrasi Baru) — Putaran 6
**Author:** Research Agent

## Ringkasan

4 new verified free data sources. CBOE gives 13 volatility/correlation indices. SEC EDGAR gives institutional 13F holdings. CryptoCompare gives exchange-level crypto data. All free, all verified live.

## Temuan 1: CBOE Full Index Suite — 13 Volatility Indices

**Pattern:** `https://cdn.cboe.com/api/global/us_indices/daily_prices/{INDEX}_History.csv`
**Auth:** None. **Format:** CSV (DATE, VALUE).

Key indices NOT already integrated:
- **SKEW** — S&P 500 tail risk (crash probability). SKEW >140 = elevated tail risk.
- **OVX** — Crude oil volatility. Cross-reference with EIA data.
- **GVZ** — Gold volatility. Safe haven fear gauge.
- **RVX** — Russell 2000 volatility. Small cap risk.
- **VIX9D** — 9-day VIX. Ultra short-term event pricing.
- **COR3M** — 3-month implied correlation. Dispersion signal.

Existing VIX integration only fetches VIX spot + futures (M1/M2/M3). These 13 additional indices provide complete volatility surface across assets and tenors.

## Temuan 2: SEC EDGAR — Institutional 13F Holdings

**Endpoints:** `data.sec.gov/submissions/CIK{cik}.json` + SGML archives
**Auth:** None (requires User-Agent header). **Rate:** 10 req/sec.

Track what Berkshire, Bridgewater, Renaissance, Citadel etc. are buying/selling quarterly. 13F filings publicly available ~45 days after quarter end.

**Trading signal use:** "Berkshire added $5B gold in Q4" → gold bullish signal. Institutional flow analysis.

Data available: issuer name, value ($1000s), share count, put/call, investment discretion.
Company ticker lookup: `sec.gov/files/company_tickers.json` (10,447 companies).

## Temuan 3: CryptoCompare Free Tier

**Base:** `min-api.cryptocompare.com`
**Auth:** None for basic endpoints. **Rate:** 100K calls/month.

Key free endpoints:
- Exchange volume history (daily) — per-exchange volume tracking
- Top by market cap/volume — rankings with 24h changes
- Pair mapping per exchange — 2,868 pairs on Binance alone

**Key differentiator from CoinGecko:** Exchange-specific volume data. CoinGecko aggregates; CryptoCompare shows per-exchange. Useful for volume divergence (Binance vs Coinbase).

## Temuan 4: Open Exchange Rates (Needs Free Signup)

**Note:** Requires free app_id registration. 1,000 req/month. USD base only.
**Trading use:** Backup FX source (173 currencies including exotics). Could complement TwelveData for exotic pairs.
**Decision:** Lower priority — existing FX sources adequate for majors. Only useful for EM currencies.

## Rejected Sources

- CME Group: actively blocks scraping
- Nasdaq Data Link (Quandl): WAF blocks server IPs
- Baltic Exchange: proprietary, no free source
- IMF Data API: DNS unreachable from cloud infra
