# Research Report: Fitur Siklus 3 Putaran 2 — Regime Overlay, Unified Signal, Session Analysis

**Tanggal:** 2026-04-01 22:00 WIB
**Fokus:** Fitur Baru (Siklus 3, Putaran 2)
**Siklus:** 3/5

---

## Ringkasan

Analisis mendalam terhadap service layer (ta/, price/, cot/, factors/) mengungkap bahwa infrastruktur quant sudah kuat — HMM, GARCH, Hurst, correlation, confluence semua exist. Yang HILANG adalah **layer orkestrasi** yang menyatukan model-model ini menjadi sinyal terpadu dan actionable.

5 fitur baru diusulkan — semua memanfaatkan infrastruktur existing, fokus pada integrasi dan unifikasi.

---

## Analisis Infrastruktur Existing

### Yang Sudah Ada (Strong Foundation)
| Component | File | Status |
|-----------|------|--------|
| HMM Regime (3 states) | service/price/hmm_regime.go (12KB) | ✅ Complete |
| GARCH Volatility | service/price/garch.go (10KB) | ✅ Complete |
| Hurst Exponent | service/price/hurst.go (10KB) | ✅ Complete |
| Correlation Matrix | service/price/correlation.go (9KB) | ✅ Complete |
| TA Confluence v1 | service/ta/confluence.go (12KB) | ✅ 10+ indicators, weighted |
| Position Sizing | service/price/position_size.go (6KB) | ✅ ATR-based, vol-adaptive |
| Seasonal Analysis | service/price/seasonal.go (19KB) | ✅ Complete |
| COT Signal Detection | service/cot/ | ✅ ConvictionScoreV3, 5 strength levels |
| Factor Engine | service/factors/engine.go | ✅ Momentum, carry, trend, crowding |
| VIX Risk Context | domain/risk.go | ✅ 4 risk levels |

### Yang Hilang (Gap = Peluang)
1. **Unified regime overlay** — HMM, GARCH, ADX, COT exist tapi TIDAK digabung
2. **Regime-aware correlation** — Correlation matrix statis, tidak indexed per regime
3. **Portfolio-level risk sizing** — Position sizing hanya per-pair, bukan cross-portfolio
4. **COT + CTA + Quant signal fusion** — Semua terpisah, belum ada "should I be long EUR?"
5. **Session behavior analysis** — Tidak ada klasifikasi London/NY/Tokyo behavior

---

## 5 Fitur Baru yang Diusulkan

### 1. Market Regime Overlay Engine (TASK-110)
Gabungkan HMM state + GARCH vol regime + ADX trend strength + COT sentiment jadi SATU "market health score" (-100 to +100). Overlay ini ditampilkan di header SEMUA analysis output (/cta, /quant, /cot).

**Weights:** HMM 30%, GARCH 25%, ADX 25%, COT 20%
**Output:** Score + color (🟢/🟡/🔴) + human-readable description
**Leverages:** hmm_regime.go, garch.go, indicators.go (ADX), cot regime

### 2. Regime-Aware Correlation Dashboard V2 (TASK-111)
Extend correlation matrix agar di-split per regime. Show: "correlasi normal" vs "correlasi saat krisis". Tag setiap historical bar dengan regime state, recalculate correlation per bucket.

**Leverages:** correlation.go, hmm_regime.go, ADX from ta engine
**New:** RegimeCorrelationMatrix struct, regime-tagged history

### 3. Risk Parity Position Sizer (TASK-112)
Cross-portfolio position sizing yang account total portfolio heat. Kelly fraction dari backtest stats. Volatility regime adjustment.

**Formula:** Kelly f* = (2p - 1) / b, applied per pair, capped at total heat max
**Leverages:** position_size.go, backtest engine, garch.go

### 4. Confluence Score V2 — Unified Signal (TASK-113)
Single directional answer per currency: "STRONG LONG EUR" with confidence. Aggregate COT (30%) + CTA (30%) + Quant Regime (20%) + Sentiment (15%) + Seasonal (5%).

**Key innovation:** Conflict detection — if COT says long but CTA says short, confidence reduced. VotingMatrix shows which sub-systems agree/disagree.

### 5. Session Analysis Engine (TASK-114)
Classify London/NY/Tokyo session behavior per pair. "London trends 65% of time" → suggest breakout. "NY ranges 70%" → suggest mean reversion. Current session context + countdown to next.

**Sessions:** Tokyo 00:00-09:00 UTC, London 08:00-17:00 UTC, NY 13:00-22:00 UTC

---

## Urutan Implementasi Rekomendasi

Phase 1: TASK-110 + TASK-111 (foundational — regime models)
Phase 2: TASK-113 (highest user impact — unified signal)
Phase 3: TASK-112 + TASK-114 (execution layer — sizing + sessions)
