# Research Report: Data Siklus 2 Putaran 4
# BIS API, Bybit Funding, Natural Gas, BTC On-Chain, MOVE Index
**Date:** 2026-04-02 07:30 WIB
**Siklus:** 2/5 (Data & Integrasi Baru) — Putaran 4
**Author:** Research Agent

## Ringkasan

Putaran ini fokus pada 5 sumber data GRATIS yang belum terintegrasi dan tidak overlap dengan TASK-105–134. Semua endpoint telah diverifikasi live.

## Temuan 1: BIS Statistics API — Central Bank & Global Liquidity

**URL:** `https://stats.bis.org/api/v2/`
**Protocol:** SDMX RESTful v2.1.0, no API key
**Format:** CSV, JSON, XML

Dataset relevan untuk forex:
- `WS_CBPOL` — Central bank policy rates (semua major central bank)
- `WS_CREDIT_GAP` — Credit-to-GDP gaps (krisis indicator)
- `WS_GLI` — Global liquidity indicators (USD credit non-US)
- `WS_DSR` — Debt service ratios (carry trade risk)
- `WS_EER` — Effective exchange rates (real vs nominal)
- `WS_LBS_D_PUB` — Cross-border banking flows

**Impact:** BIS data memberikan institutional-grade macro context yang tidak ada di FRED. Credit-to-GDP gap adalah salah satu indikator krisis terbaik (BIS sendiri yang riset). Global liquidity indicators menunjukkan USD funding stress yang langsung impact FX carries.

**Existing coverage:** Zero. Tidak ada integrasi BIS di codebase.

## Temuan 2: Bybit Funding Rate Historical

**Endpoint:** `GET https://api.bybit.com/v5/market/funding/history`
**Auth:** None required (public)
**Params:** category=linear, symbol=BTCUSDT, limit=200
**Rate limit:** ~10 req/sec

Codebase sudah punya Bybit client (`internal/service/marketdata/bybit/client.go`) tapi HANYA fetch orderbook, trades, tickers, klines, long/short ratio, dan OI. Funding rate history TIDAK di-fetch.

**Impact:** Funding rate adalah sinyal positioning terpenting di crypto perpetuals. Extreme funding → reversal signal. Historical tracking memungkinkan: funding regime detection, carry cost analysis, dan convergence/divergence with spot.

**Implementation:** Tambah method `GetFundingHistory()` ke existing Bybit client. Minimal code change.

## Temuan 3: EIA Natural Gas Expansion

**API:** `https://api.eia.gov/v2/natural-gas/`
**Auth:** Same EIA_API_KEY yang sudah ada di .env

Routes yang tersedia:
- `/natural-gas/stor/wkly` — Weekly storage (inventory builds/draws)
- `/natural-gas/pri/fut` — Henry Hub spot price (daily)
- `/natural-gas/prod` — Production data
- `/natural-gas/cons` — Consumption

Codebase `internal/service/price/eia.go` sudah fetch 5 petroleum series. Natural gas TIDAK di-fetch meskipun key sudah ada.

**Impact:** Natural gas adalah commodity ke-3 paling traded setelah crude oil dan gold. Storage report (Thursday weekly) adalah high-impact event. Henry Hub price berkorelasi dengan energy sector earnings dan utility costs.

## Temuan 4: Blockchain.com BTC On-Chain Metrics

**API:** `https://api.blockchain.info/`
**Auth:** None required
**Rate limit:** ~1 req/sec

Endpoints gratis:
- `/charts/hash-rate` — Network hash rate (miner health)
- `/charts/mempool-size` — Mempool congestion
- `/charts/transaction-fees` — Fee market activity
- `/charts/difficulty` — Mining difficulty adjustments
- `/stats` — Real-time aggregate (hash_rate, n_tx, total_fees_btc, market_price_usd)

**Impact:** Zero on-chain data di codebase saat ini. Hash rate trend = miner capitulation/accumulation signal. Mempool congestion = network demand proxy. Fee spikes = activity surge (often precede volatility).

## Temuan 5: MOVE Index via Yahoo Finance

MOVE index (ICE BofA) = bond market VIX. CBOE TIDAK publish MOVE via CSV (403 forbidden — MOVE is ICE product, not CBOE).

**Free source:** Yahoo Finance ticker `^MOVE`
- Codebase sudah punya Yahoo Finance fetcher (`internal/service/price/fetcher.go`) yang fetch via `query1.finance.yahoo.com`
- Bisa reuse existing infrastructure

**Impact:** MOVE index adalah satu-satunya institutional bond volatility gauge. VIX measures equity vol, MOVE measures Treasury vol. Cross-asset: VIX/MOVE ratio = equity vs bond fear divergence. High MOVE + low VIX = warning signal for FX carry unwind.

## Gap Analysis vs Existing Tasks

| Area | Existing Tasks | New (This Cycle) |
|------|---------------|-----------------|
| Central bank data | TASK-105 (ECB), TASK-106 (SNB) | BIS covers ALL central banks |
| Crypto derivatives | TASK-130 (IV), TASK-131 (DVOL) | Funding rate = different signal |
| Energy | None | Natural gas expansion |
| On-chain | None | BTC blockchain metrics |
| Bond volatility | None | MOVE index |

Tidak ada overlap. Semua 5 task baru adalah integrasi yang genuinely baru.
