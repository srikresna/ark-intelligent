# Feature Index Audit — Cycle 3 Putaran 19
**Date:** 2026-04-02 | **Siklus:** 3/5 (Feature Index) | **Putaran:** 19

---

## Metodologi

Audit mendalam terhadap FEATURE_INDEX.md vs implementasi aktual di codebase. Fokus pada:
- Fitur yang ada di index tapi dead code / tidak diekspos ke user
- Gap kuantitatif yang disebutkan di "Area Riset Potensial"
- Peluang incremental improvement pada engine yang sudah ada

---

## GAP 1: `/carry` Command — FormatCarryRanking Dead Code ⚠️ KRITIS

### Status
- `internal/service/fred/rate_differential.go` → `FetchCarryRanking()` — **LENGKAP**
- `internal/adapter/telegram/formatter_quant.go:279` → `FormatCarryRanking()` — **LENGKAP**
- `internal/scheduler/scheduler.go:428-430` — dipakai untuk COT adjustment, **bukan untuk user output**

### Gap
**Tidak ada `/carry` command yang terdaftar.** Formatter ditulis, engine ditulis, tapi user tidak bisa mengaksesnya. Dead code formatter yang sudah production-ready.

### Evidence
```bash
grep -rn "RegisterCommand.*carry\|cmdCarry" internal/ → empty
grep -rn "FormatCarryRanking" internal/ → 1 result: hanya definisi
```

### Impact
User tidak bisa melihat carry trade ranking secara standalone. Data yang paling berguna untuk carry trader (AUD/JPY, NZD/JPY carry pairs) tersembunyi di dalam COT adjustment dan tidak diekspos.

### Fix
1 fungsi `cmdCarry` + 1 `RegisterCommand` di `handler.go`. Estimasi: 30 menit.

---

## GAP 2: GJR-GARCH Asymmetric Volatility — Belum Implemented

### Status
- GARCH(1,1) standard → **LENGKAP** di `internal/service/price/garch.go`
- GJR-GARCH (leverage effect) → **TIDAK ADA**
- EGARCH → **TIDAK ADA**

### Gap
FEATURE_INDEX secara eksplisit menyebut: "EGARCH, GJR-GARCH untuk volatility asymmetry". Volatility asymmetry penting untuk FX: downward moves menghasilkan volatility lebih tinggi dari upward moves (leverage effect).

### GJR-GARCH Formula
```
σ²(t) = ω + α·ε²(t-1) + γ·I(ε(t-1)<0)·ε²(t-1) + β·σ²(t-1)
```
- I = indicator function: 1 jika return negatif, 0 jika positif
- γ > 0 → asymmetry/leverage effect confirmed
- Total persistence = α + γ/2 + β

### Manfaat Praktis
- Deteksi apakah pair sedang dalam "fear regime" (downside vol > upside vol)
- Signal untuk /quant: "GJR-GARCH persistence=0.97, asymmetry=0.08 → downside risk elevated"
- Input yang lebih akurat untuk position sizing saat market sedang jatuh

### Implementation Path
Tambah `EstimateGJRGARCH()` di `price/garch.go` menggunakan variance targeting + grid search untuk (α, γ, β). Return `GJRGARCHResult` dengan field `AsymmetryCoeff float64` dan `AsymmetryLabel string`.

---

## GAP 3: COT 4-Week OI Accumulation Momentum — Belum Ada

### Status
- OITrend (1-week): **ADA** di `analyzer.go:302-306`
- OIPctChange (1-week): **ADA**
- Multi-week OI momentum: **TIDAK ADA**

### Gap
Current OITrend hanya 1-week change (current vs prev). Tidak ada analisis apakah OI sedang ACCUMULATED (naik 4 minggu berturut) atau DISTRIBUTED (turun 4 minggu berturut).

### Signal Logic
```
Jika OI[W-1] > OI[W-2] > OI[W-3] > OI[W-4] → OI4WTrend = "ACCUMULATING" → conviction +5
Jika OI[W-1] < OI[W-2] < OI[W-3] < OI[W-4] → OI4WTrend = "DISTRIBUTING" → conviction -5
Else → OI4WTrend = "MIXED"
```

### Manfaat
- "ACCUMULATING + BULLISH spec net" = strong long conviction
- "DISTRIBUTING + BULLISH spec net" = caution, smart money reducing despite bullish price
- Lebih robust dari 1-week OI change (menghilangkan noise)

### Implementation Path
Tambah `OI4WTrend string` + `OI4WMomentum float64` ke `domain.COTAnalysis`. Hitung di `analyzer.go` pada section 5 dengan `history[:minInt(5, len(history))]`.

---

## GAP 4: HMM 4-State Regime — TRENDING State Missing

### Status
- HMM 3-state (RISK_ON, RISK_OFF, CRISIS): **ADA** di `price/hmm_regime.go`
- TRENDING state: **TIDAK ADA**

### Gap
FEATURE_INDEX menyebut "lebih banyak state". 3-state HMM menggabungkan "calm trending" dan "volatile risk-on" ke satu state RISK_ON. Ini menciptakan false signals: market yang strongly trending tapi tidak volatile dikategorikan sama dengan market choppy risk-on.

### TRENDING State Definition
```
State 3: TRENDING
- Low volatility (< historical median)
- Positive drift (> 0.3% per week)
- High Hurst exponent (> 0.55, estimated from return autocorrelation)
- State color: BLUE (vs RISK_ON = GREEN, RISK_OFF = YELLOW, CRISIS = RED)
```

### Implikasi Sinyal
- TRENDING → increase confidence on directional signals, reduce reversion signals
- RISK_ON (choppy) → reduce directional confidence, prefer mean-reversion

### Implementation Path
`hmmNumStates = 4`, update emission priors, transition matrix priors. Rebalance emission bins untuk TRENDING (tight distribution, positive mean). Update `EstimateHMMRegime()` state labels.

**Note:** Sebelum implement ini, TASK-171 (minimum boundary fix) harus selesai dulu.

---

## GAP 5: VIX M3 + Full Term Structure Curve Slope — Tidak Diekspos

### Status
- M3 data: **DIFETCH** di `vix/fetcher.go:218-219`
- M3 diekspos ke user: **TIDAK** (tidak ada di formatter manapun)
- M3Symbol: sama, tidak diekspos

### Gap
VIX term structure M3 (3rd month futures) difetch tapi tidak pernah digunakan di output manapun. Padahal M3 memberikan informasi penting:
- **Full curve slope** = (M3-M1)/M1 → lebih stabil dari M2-M1
- **Contango steepness** → semakin curam = semakin risk-on, carry positif untuk vol sellers
- **Calendar spread** = M3-M2 → expected vol change in month 2-3

### Manfaat untuk /sentiment
```
VIX Term Structure:
Spot: 18.2 | M1: 19.4 | M2: 20.1 | M3: 20.8
Contango: 13.2% (M3 vs Spot) — Normal risk-on
Full Slope: +7.2% (M3/M1-1) — Moderately steep
Calendar Spread M2→M3: +0.7 — Consistent premium
```

### Implementation Path
- Tambah M3 ke `FormatVIX()` / `/sentiment` output
- Tambah `FullSlopePct float64` ke `VIXTermStructure`: `(M3-M1)/M1 * 100`
- Surface M3Symbol di sentiment formatter untuk context

---

## Summary: 5 Gaps Prioritized

| Task | Gap | Effort | Value |
|------|-----|--------|-------|
| TASK-285 | /carry command dead code | S | HIGH |
| TASK-286 | GJR-GARCH asymmetric vol | M | HIGH |
| TASK-287 | COT 4-week OI accumulation | M | HIGH |
| TASK-288 | HMM 4-state (TRENDING) | M | MEDIUM |
| TASK-289 | VIX M3 full term structure expose | S | MEDIUM |

---

## Referensi Codebase

- `internal/adapter/telegram/formatter_quant.go:279` — FormatCarryRanking (dead code)
- `internal/service/price/garch.go` — GARCH(1,1) implementation
- `internal/service/cot/analyzer.go:280-306` — OI trend section
- `internal/service/price/hmm_regime.go:54` — HMM 3-state
- `internal/service/vix/fetcher.go:208-219` — M3 fetch (never surfaced)
- `internal/service/vix/types.go:11` — M3 field (never used in formatters)
