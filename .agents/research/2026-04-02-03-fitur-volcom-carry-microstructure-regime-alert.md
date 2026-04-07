# Research Report: Fitur Siklus 3 Putaran 3 — Vol Cone, Carry Monitor, Microstructure+, Regime Alert, Backtest+

**Tanggal:** 2026-04-02 03:00 WIB
**Fokus:** Fitur Baru (Siklus 3, Putaran 3)
**Siklus:** 3/5

---

## Ringkasan

Audit infrastruktur quant existing dan riset 5 fitur advanced baru. Semua memanfaatkan foundation yang sudah kuat: GARCH, HMM, Deribit, FRED rate differential, Bybit microstructure, dan backtest engine. Fokus: institutional-grade edge.

---

## Temuan Infrastruktur

### Yang Sudah Kuat
- **GARCH + ATR + VIX volatility pipeline** — fully integrated, confidence multipliers ada
- **HMM 3-state regime** — Baum-Welch, Viterbi, TransitionWarning sudah compute P(regime_change)
- **FRED rate differential** — CarryRanking, RateDifferential per pair sudah ada
- **Bybit microstructure** — OrderbookImbalance, TakerBuyRatio, LongShortRatio, FundingRate
- **Backtest** — WalkForward, MonteCarlo, Bootstrap, RiskOfRuin, WeightOptimizer semua exist
- **Deribit** — Public API client, GEX engine, VIX term structure merged

### Gap = Feature Opportunities
1. Vol cone (IV percentile bands by calendar period) — TIDAK ADA
2. Carry regime change detector — TIDAK ADA (carry computed tapi tidak monitored)
3. Microstructure absorption/delta divergence — TIDAK ADA (basic imbalance only)
4. Proactive regime alerts — TIDAK ADA (HMM computed on-demand, bukan triggered)
5. Multi-strategy backtester — TIDAK ADA (single signal stream only)

---

## 5 Fitur Baru

### 1. Volatility Cone Analysis (TASK-135)
Build "vol cone" dari GARCH + Deribit IV: IV percentile bands (25th/50th/75th) per calendar month. Alert saat current vol di percentile >95th atau <5th.

### 2. Carry Trade Monitor & Unwind Detector (TASK-136)
Extend FRED rate differential: real-time carry ranking + roll yield, daily P&L tracking, carry unwind detector (saat spread kolaps <2%).

### 3. Microstructure Flow Enhancement (TASK-137)
Upgrade Bybit microstructure: trade size bucketing (large vs retail), absorption detection, delta divergence (OI direction vs price direction mismatch).

### 4. Proactive Regime Change Alert (TASK-138)
Leverage HMM TransitionWarning: track regime duration, alert tiers (AMBER P>20%, RED P>50%), multi-asset regime sync detection.

### 5. Multi-Strategy Backtester (TASK-139)
Run independent backtests per strategy (COT, CTA, Wyckoff, GEX, Seasonal), compute per-strategy Sharpe + correlation, combine via StrategyComposer.
