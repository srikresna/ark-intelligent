# Research Report — Siklus 4: Tech Refactor Plan (Putaran 15)
**Date:** 2026-04-02 12:00 WIB
**Focus:** TECH-001, TECH-002, TECH-011 — Splitting god classes di adapter/telegram/

---

## Ringkasan Eksekutif

Audit ulang terhadap tiga item refactor tertinggi prioritas dari TECH_REFACTOR_PLAN.md.
formatter.go tumbuh dari 4,489 → **4,539 LOC** dan handler.go tumbuh dari 2,381 → **2,909 LOC** sejak
dokumen plan dibuat. Keduanya belum direfactor walaupun sudah ada partial split (formatter_ict, formatter_gex,
formatter_wyckoff, formatter_quant, dan 10+ handler_*.go files).

---

## Temuan per File

### 1. formatter.go (4,539 LOC) — TECH-001 masih open

**Sudah di-split:** formatter_ict.go, formatter_gex.go, formatter_wyckoff.go, formatter_quant.go, formatter_compact.go

**Masih dalam formatter.go — 50 fungsi, bisa dibagi jadi domain berikut:**

| Domain | Fungsi | LOC Estimasi |
|--------|--------|--------------|
| Calendar | FormatCalendarDay/Week/Month, FormatUpcomingCatalysts | ~230 |
| COT+Bias | FormatCOTOverview, FormatCOTDetail*, FormatCOTRaw, FormatRanking*, FormatConvictionBlock, FormatBias* | ~1,343 |
| Macro/FRED | FormatMacroRegime, FormatFREDContext, FormatMacroComposites, FormatMacro{Global,Labor,Inflation,Summary,Explain}, FormatRegimeLabel, FormatRegimePerformance | ~1,003 |
| Backtest | FormatBacktestStats, FormatBacktestSummary, FormatSignalTiming, FormatWalkForward, FormatWeightOptimization, FormatSmartMoneyAccuracy, FormatExcursion*, FormatTrendFilter* | ~511 |
| Price/Seasonal | FormatPriceContext, FormatPriceCOT*, FormatStrengthRanking, FormatDailyPrice*, FormatSeasonal*, FormatLevels | ~803 |

**Quick wins (2 tasks baru):**
- `formatter_macro.go` — ambil 12 fungsi FormatMacro*/FormatFRED*/FormatRegime* (~1,003 LOC)
- `formatter_cot.go` — ambil 9 fungsi FormatCOT*/FormatRanking*/FormatBias* (~1,343 LOC)

Setelah kedua split ini, formatter.go tinggal ~2,193 LOC (turun 52%).

### 2. handler.go (2,909 LOC) — TECH-002 masih open

**Sudah di-split:** handler_alpha.go, handler_backtest.go, handler_ctabt.go, handler_cta.go,
handler_gex.go, handler_ict.go, handler_levels.go, handler_price.go, handler_quant.go,
handler_seasonal.go, handler_smc.go, handler_vp.go, handler_wyckoff.go

**Masih dalam handler.go — 33+ fungsi, paling logis dipecah ke:**

| Split Target | Fungsi | LOC Estimasi |
|---|---|---|
| handler_macro.go | cmdMacro, macroSendSummary, macroSendDetail, cbMacro, buildRegimeAssetInsight, macroRegimePerformance, currentMacroRegimeName | ~253 |
| handler_admin.go | cmdMembership, requireAdmin, cmdUsers, cmdSetRole, cmdBan, cmdUnban | ~254 |

Setelah kedua split ini, handler.go turun ~507 LOC dari 2,909 → ~2,402 LOC.

### 3. TECH-011: Hardcoded Contract Codes — masih ada di 3 tempat

`domain/contracts.go` sudah ada konstanta (ContractEUR = "099741", dsb.) **tapi belum digunakan** di:

1. `handler.go:1368` — fungsi `currencyToContractCode()` menggunakan string literals lengkap
2. `formatter.go:267` — array `Codes` dengan 8 hardcoded strings
3. `formatter.go:1042` — reverse map code→currency dengan 8 hardcoded strings

Ini TECH-011 yang masih open meski constants sudah tersedia. Quick win.

### 4. Progress TECH-006 (Magic Numbers)

- `internal/config/constants.go` sudah ada dengan `SignalStrengthAlert`, `LongPollTimeout`, dll.
- `api.go:512` masih ada `35*time.Millisecond` — belum jadi konstanta
- `bot.go:202` ada `5 * time.Second` tapi sudah ada `config.PollRetryDelay` — perlu dicek apakah sudah pakai konstanta

### 5. Progress TECH-005 (Dual Scheduler)

- `scheduler/scheduler.go` = 1,125 LOC
- `service/news/scheduler.go` = 1,101 LOC
- Keduanya masih terpisah — belum ada unifikasi

---

## Rekomendasi Task Baru (TASK-265 s/d TASK-269)

1. **TASK-265**: Split formatter.go → formatter_macro.go (12 fungsi Macro/FRED/Regime)
2. **TASK-266**: Split formatter.go → formatter_cot.go (9 fungsi COT/Ranking/Bias)
3. **TASK-267**: Split handler.go → handler_macro.go (7 fungsi macro)
4. **TASK-268**: Split handler.go → handler_admin.go (6 fungsi admin/membership)
5. **TASK-269**: Fix TECH-011 — ganti hardcoded contract code literals dengan domain.ContractXXX

---

## Catatan Eksekusi

- Semua task ini adalah **pure refactor** — zero behavior change, hanya pindah fungsi ke file baru
- Pola split yang benar: file baru di package `telegram`, gunakan `package telegram` header
- Harus `go build ./... && go vet ./...` clean sebelum PR
- Lihat `formatter_ict.go` dan `formatter_compact.go` sebagai contoh file split yang benar
