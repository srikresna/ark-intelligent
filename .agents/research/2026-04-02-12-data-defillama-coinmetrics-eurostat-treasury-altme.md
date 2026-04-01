# Research Report: Data Siklus 2 Putaran 5
# DefiLlama, CoinMetrics, Eurostat, TreasuryDirect, Alternative.me Extended
**Date:** 2026-04-02 12:00 WIB
**Siklus:** 2/5 (Data & Integrasi Baru) — Putaran 5
**Author:** Research Agent

## Ringkasan

5 sumber data baru yang GRATIS dan VERIFIED live. Semua no-auth (kecuali noted). Tidak overlap dengan 20+ existing data tasks.

## Temuan 1: DefiLlama — DeFi TVL, DEX Volume, Stablecoins, Yields

**Endpoints verified:**
- `https://api.llama.fi/v2/protocols` — 7,256 protocols with TVL
- `https://api.llama.fi/overview/dexs` — 1,059 DEX volume data
- `https://api.llama.fi/overview/fees` — 1,969 protocols fees/revenue
- `https://stablecoins.llama.fi/stablecoins` — Stablecoin supply tracking
- `https://yields.llama.fi/pools` — DeFi yield aggregation
- `https://bridges.llama.fi/bridges` — Cross-chain bridge volumes

**Auth:** None. No key. No rate limit documented.
**Format:** JSON

**Impact:** DeFi TVL is the #1 metric institutions track for crypto health. Stablecoin supply growth/contraction = liquidity proxy. DEX volume surge = retail activity spike. Zero DeFi data in codebase currently.

## Temuan 2: CoinMetrics Community API — On-Chain + Exchange Flows

**Endpoint:** `https://community-api.coinmetrics.io/v4/timeseries/asset-metrics`
**Params:** `assets=btc&metrics=ReferenceRateUSD,TxCnt,AdrActCnt,HashRate,FlowInExNtv,FlowOutExNtv&frequency=1d`

**Free metrics verified:**
- `ReferenceRateUSD` — Reference price
- `TxCnt` — Transaction count
- `AdrActCnt` — Active addresses
- `HashRate` — Network hash rate
- `FlowInExNtv` — Exchange inflow (native units)
- `FlowOutExNtv` — Exchange outflow (native units)
- `IssTotNtv` — Total issuance
- `CapMrktEstUSD` — Market cap

**Auth:** None for community tier. Covers 5,700+ assets.
**Key differentiator from TASK-158 (Blockchain.com):** Exchange flow data (inflow/outflow). This is the whale tracking signal — coins leaving exchanges = accumulation, coins entering = distribution.

## Temuan 3: Eurostat API — EU Macro Data

**Endpoints verified:**
- HICP Inflation: `https://ec.europa.eu/eurostat/api/dissemination/statistics/1.0/data/prc_hicp_manr?geo=EA20&coicop=CP00&format=JSON`
- Unemployment: `https://ec.europa.eu/eurostat/api/dissemination/statistics/1.0/data/une_rt_m?geo=EA20&s_adj=SA&format=JSON`
- GDP Growth: `https://ec.europa.eu/eurostat/api/dissemination/statistics/1.0/data/namq_10_gdp?geo=EA20&unit=CLV_PCH_PRE&format=JSON`

**Auth:** None. Public EU government data.
**Impact:** Critical for EUR pair trading. Complements ECB SDW (TASK-105) with granular EU-wide statistics. FRED has some EU proxies but Eurostat is the primary source.

## Temuan 4: TreasuryDirect Auction API — Bond Auction Results

**Endpoint:** `https://www.treasurydirect.gov/TA_WS/securities/search?type=Note&pagesize=10&format=json`

**Data available:**
- Auction date, issue date, maturity
- High yield, interest rate
- Bid-to-cover ratio (demand indicator)
- Direct bidder %, indirect bidder % (foreign central bank demand proxy)
- Allotted amount, total accepted

**Auth:** None. US government public data.
**Impact:** Treasury auction results move FX markets significantly. Indirect bidder % dropping = foreign central bank reducing USD reserves = bearish USD. Bid-to-cover ratio < 2.0 = weak demand = higher yields ahead.

## Temuan 5: Alternative.me Extended Crypto Endpoints

**Beyond Fear & Greed (already integrated):**
- `https://api.alternative.me/v2/global/` — BTC dominance, total market cap, active currencies count
- `https://api.alternative.me/v2/ticker/` — Top crypto with 1h/24h/7d % changes, rank, volume

**Auth:** None.
**Impact:** Complements CoinGecko with alternative data source. BTC dominance trend = altcoin rotation signal. Volume-to-market-cap ratio = liquidity health.

## Gap Analysis vs Existing Tasks

| Area | Existing Coverage | New (This Cycle) |
|------|------------------|-----------------|
| DeFi | Zero | DefiLlama (TVL, DEX, stablecoins) |
| Exchange flows | Zero | CoinMetrics (in/out flow) |
| EU macro | FRED proxies only | Eurostat (primary source) |
| Bond auctions | FRED yields only | TreasuryDirect (auction details) |
| Crypto market | CoinGecko only | Alternative.me extended |

Zero overlap with existing 25 data tasks.
