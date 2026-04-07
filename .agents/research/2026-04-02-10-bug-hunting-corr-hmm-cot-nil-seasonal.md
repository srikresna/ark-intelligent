# Research Report: Bug Hunting Siklus 5 Putaran 4
# Correlation Bounds, HMM Boundary, COT Broadcast Race, Nil Pointers
**Date:** 2026-04-02 10:00 WIB
**Siklus:** 5/5 (Bug Hunting) — Putaran 4
**Author:** Research Agent

## Ringkasan

Analisis mendalam menemukan 10 bug baru. 5 yang paling critical dijadikan task. Fokus: statistical edge cases, nil pointer risks, dan race conditions.

## Temuan 1: Correlation pearsonCorrelation() < 3 Points Silent Zero

**File:** `internal/service/price/correlation.go:322-354`

Function truncates kedua array ke min length, lalu checks `n < 3`. Tapi `<` bukan `<=`, jadi n=2 lolos. Dengan 2 data points, correlation mathematically undefined tapi return 0 — silently losing signal information. Denom check (line 349) handles division-by-zero tapi n=2 tetap produce unreliable output.

**Impact:** Correlation matrix entries bernilai 0 padahal seharusnya error/NaN.
**Severity:** HIGH

## Temuan 2: HMM Boundary at Exactly 40 Returns

**File:** `internal/service/price/hmm_regime.go:55-70`

Prices check: `len(prices) < 60`. Returns check: `len(returns) < 40`. Gap antara 60 prices dan 40 returns memungkinkan ~33% invalid prices lolos. Dengan hanya 40 returns untuk 3 states × 5 emissions = 18 parameters, system severely underparameterized. Baum-Welch converges to overfit.

**Impact:** Unreliable regime classification saat data marginal.
**Severity:** HIGH

## Temuan 3: COT Broadcast Duplicate Race Condition

**File:** `internal/scheduler/scheduler.go:305-318`

`broadcastCOTRelease()` checks oldLatest vs newLatest tanpa locking. Jika 2 concurrent triggers (scheduled + manual), keduanya bisa detect same new date dan broadcast duplicate. Tidak ada dedup mechanism.

**Impact:** Users terima COT alert duplikat.
**Severity:** MEDIUM

## Temuan 4: FRED Composites Nil Pointer in Regime Classification

**File:** `internal/service/fred/regime.go:254+, internal/service/cot/confluence_score.go:255+`

`ComputeComposites()` bisa return nil jika country data tidak lengkap. Downstream code checks nil tapi tidak semua paths protected. Line 256 di confluence_score.go checks `composites != nil` tapi tidak check individual composites[i] fields.

**Impact:** Panic saat regime classification dengan missing country data.
**Severity:** HIGH

## Temuan 5: Seasonal Analysis Nil Pointer on New Contracts

**File:** `internal/service/price/seasonal.go:~150-200`

`SeasonalPattern` struct fields (`RegimeStats`, `COTAlignment`, `EventDensity`) are pointers. Contracts with <1 year history produce nil fields. Calling code may access `.AvgReturn` tanpa nil check → panic.

**Impact:** Panic saat /seasonal pada instrumen baru.
**Severity:** HIGH

## Additional Findings (Not Tasked — Lower Priority)

- GARCH underparameterization at n=20-27 (MEDIUM)
- Telegram message >4096 not chunked in all paths (MEDIUM)
- FRED alert missed when prior data is 0 (MEDIUM)
- Backtest NaN confidence interval at 0 evaluated signals (MEDIUM)
- COT confluence score edge case with missing composites (MEDIUM)
