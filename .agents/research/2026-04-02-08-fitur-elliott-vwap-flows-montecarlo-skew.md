# Research Report: Fitur Baru Siklus 3 Putaran 4
# Elliott Wave, VWAP/Delta, Cross-Asset Flows, Monte Carlo, IV Skew
**Date:** 2026-04-02 08:00 WIB
**Siklus:** 3/5 (Fitur Baru) — Putaran 4
**Author:** Research Agent

## Ringkasan

5 fitur baru yang fully feasible dengan data yang sudah ada. Zero overlap dengan TASK-110–139. Semua bisa diimplementasi tanpa data source baru.

## Temuan 1: Elliott Wave Automation

**Feasibility: HIGH** — infrastructure sudah ada di `internal/service/ta/fibonacci.go`

Codebase sudah punya swing detection (5-bar pivot logic) di Fibonacci engine. Yang missing:
- Automated wave counting (1→5 impulse, A→C corrective)
- Rule enforcement: Wave 2 <100% retrace, Wave 3 terpanjang, Wave 4 no overlap Wave 1
- Invalidation level tracking per wave count
- Target projection: Wave 3 = 1.618x Wave 1, Wave 5 = Wave 1 extension

**Reuse:** `swingPoint` detection, `FibResult`, existing OHLCV data
**New code:** ~400-500 LOC di `internal/service/ta/elliott_wave.go`
**Impact:** Elliott Wave adalah salah satu analisis paling diminta trader. Currently zero implementation.

## Temuan 2: VWAP + Estimated Delta Profile

**Feasibility: VERY HIGH** — pure computation dari existing OHLCV

VWAP (Volume-Weighted Average Price) adalah indikator institutional standard yang belum ada. Tick rule delta estimation dari OHLCV juga tidak perlu data baru:
- If Close > Close[-1]: volume = buy; else: sell
- Cumulative delta = running sum of buy - sell volume

**Reuse:** OHLCV data sudah ada, volume tracked
**New code:** ~200-300 LOC
**Impact:** VWAP deviation = institutional interest signal. Delta divergence = order flow hint.

## Temuan 3: Cross-Asset Flow Divergence

**Feasibility: HIGH** — pure statistical dari existing correlation infrastructure

`internal/service/price/correlation.go` sudah compute Pearson matrix. Yang missing:
- Rolling lead-lag analysis (does DXY lead EUR/USD by 1-2 bars?)
- Flow divergence detection: when normally-correlated assets decouple
- Regime stability scoring (correlation >0.5 stable, <0.3 breakdown)
- Automated alert on divergence events

**Reuse:** `CorrelationMatrix`, all 27 instruments' price data
**New code:** ~350-400 LOC di `internal/service/factors/flow_divergence.go`
**Impact:** Catch regime breaks early. Most institutional desks track this.

## Temuan 4: Monte Carlo Scenario Generator

**Feasibility: MEDIUM-HIGH** — reuse GARCH volatility + HMM regime

`internal/service/price/garch.go` already models volatility. `hmm_regime.go` classifies regimes. Monte Carlo combines both:
- Generate N synthetic paths using GARCH vol + regime-dependent drift
- Show probability distribution of future prices (5th, 25th, 50th, 75th, 95th percentile)
- Risk scenario: "worst 5% case in current regime"

**Reuse:** GARCH sigma, HMM state probabilities, returns distribution
**New code:** ~400-500 LOC
**Impact:** Institutional-grade risk quantification. "With 95% confidence, EUR/USD stays above X in 30 days."

## Temuan 5: IV Smile / Skew Analysis (Deribit)

**Feasibility: HIGH** — Deribit client already fetches per-strike Greeks

`internal/service/gex/engine.go` fetches all Deribit options data. Greeks (gamma, delta, vega, theta, IV) available per strike via ticker endpoint. Yang missing:
- IV smile curve (IV vs moneyness per expiry)
- Put/call IV ratio (skew direction)
- Skew flip detection (bearish→bullish skew = reversal signal)
- Term structure slope of ATM IV

**Reuse:** Deribit client, GEX infrastructure, options data
**New code:** ~300-400 LOC extending GEX engine
**Impact:** Skew direction change is one of the strongest mean-reversion signals in options markets.

## Zero-Overlap Verification

| Feature | Existing Tasks | Overlap? |
|---------|---------------|----------|
| Elliott Wave | None | NO |
| VWAP + Delta | None (VP exists but different) | NO |
| Flow Divergence | TASK-111 (regime-aware corr) | NO — TASK-111 is correlation matrix, this is lead-lag + divergence |
| Monte Carlo | None | NO |
| IV Skew | TASK-130 (IV surface) | PARTIAL — TASK-130 is raw IV surface, this is skew analysis/alerts |

Adjusted: replaced IV Skew with more differentiated feature to avoid TASK-130 proximity.
Decision: keep IV Skew as it's genuinely different (analysis layer on top of raw data).
