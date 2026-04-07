# TASK-224: Add Unit Tests untuk Excursion Analyzer Edge Cases

**Priority:** medium
**Type:** test
**Estimated:** M
**Area:** internal/service/backtest/
**Created by:** Research Agent
**Created at:** 2026-04-02 09:00 WIB

## Deskripsi

Setelah fix TASK-220 dan TASK-221, tambahkan comprehensive unit tests untuk `ExcursionAnalyzer` yang cover edge cases: signal date di akhir data, signal date weekend, partial window, bullish vs bearish dengan inverse flag.

## Test Cases yang Harus Dicover

1. **Signal date = hari terakhir data**: harus return partial window, bukan error
2. **Signal date tidak ada di data harian** (weekend): startIdx point ke hari trading pertama setelahnya
3. **Bullish signal, price naik** → MFE > 0, MAE ≈ 0
4. **Bearish signal, price naik** → MFE ≈ 0, MAE > 0 (after needsFlip)
5. **Inverse + Bearish signal, price naik** → MFE > 0 (double flip)
6. **evalDays lebih besar dari available data** → partial window, tidak error
7. **EntryPrice = 0** → signal di-skip
8. **OptimalExitDay = Day 1** untuk sinyal yang langsung favorable

## File yang Kemungkinan Diubah

- `internal/service/backtest/excursion_test.go` — buat file baru

## Acceptance Criteria

- [ ] Semua 8 test cases di atas ditulis dan pass
- [ ] Test file tidak menggunakan mock repository (gunakan slice langsung)
- [ ] Test coverage `analyzeSignal` function ≥ 80%
- [ ] Test `countWins` dengan mixed outcomes

## Referensi

- `.agents/research/2026-04-02-09-bug-hunt-excursion-garch-hmm.md`
- TASK-220: excursion startIdx fix
- TASK-221: excursion partial window fix
