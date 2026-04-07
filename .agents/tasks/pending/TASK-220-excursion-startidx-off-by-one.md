# TASK-220: Fix Excursion startIdx Off-by-One (Signal Date Included)

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/service/backtest/
**Created by:** Research Agent
**Created at:** 2026-04-02 09:00 WIB

## Deskripsi

`excursion.go` baris 216 mencari startIdx dengan kondisi `dp.Date.After(signalDate) || dp.Date.Equal(signalDate)`. Kondisi `Equal` menyebabkan hari sinyal itu sendiri dimasukkan sebagai Day 0 excursion, padahal EntryPrice = close hari tersebut → Day 0 selalu ≈ 0% return.

## Bug Detail

```go
// excursion.go:213-219 — BUG: Equal() menyebabkan signal date = Day 0
for i, dp := range reversed {
    if dp.Date.After(signalDate) || dp.Date.Equal(signalDate) {
        startIdx = i
        break
    }
}
```

Dampak:
- Day 0 MFE/MAE selalu ≈ 0 (zero return pada signal date)
- Seluruh excursion window mundur 1 hari
- OptimalExitDay off-by-one
- AvgMFEPct underestimated (waste 1 of 10 evaluation days)

## Fix

```go
for i, dp := range reversed {
    if dp.Date.After(signalDate) {
        startIdx = i
        break
    }
}
```

## File yang Kemungkinan Diubah

- `internal/service/backtest/excursion.go` — ubah kondisi pada baris ~216

## Acceptance Criteria

- [ ] Kondisi `|| dp.Date.Equal(signalDate)` dihapus
- [ ] Day 1 = trading day SETELAH signal date
- [ ] `analyzeSignal` masih berjalan dengan benar untuk sinyal yang tanggalnya tidak ada di data harian
- [ ] Unit test: signal di hari Jumat, Day 1 = hari Senin berikutnya

## Referensi

- `.agents/research/2026-04-02-09-bug-hunt-excursion-garch-hmm.md`
