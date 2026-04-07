# TASK-221: Fix Excursion Boundary Check — Allow Partial Windows for Recent Signals

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/service/backtest/
**Created by:** Research Agent
**Created at:** 2026-04-02 09:00 WIB

## Deskripsi

`excursion.go` baris 222 membuang semua sinyal yang tidak memiliki `evalDays` (default 10) hari data setelah signal date. Ini menyebabkan semua sinyal yang berumur kurang dari 2 minggu tidak pernah dimasukkan ke excursion analysis. Data MFE/MAE menjadi biased ke sinyal lama.

## Bug Detail

```go
// excursion.go:222 — BUG: terlalu strict, recent signals dibuang
if startIdx < 0 || startIdx+evalDays > len(reversed) {
    return nil, fmt.Errorf("insufficient daily data after signal date")
}
```

Dampak: Signal diterbitkan 5 hari lalu dengan evalDays=10 → dibuang. Padahal MFE/MAE untuk 5 hari sudah valuable.

## Fix

```go
effectiveEvalDays := evalDays
if startIdx >= 0 && startIdx+evalDays > len(reversed) {
    effectiveEvalDays = len(reversed) - startIdx
}
if startIdx < 0 || effectiveEvalDays <= 0 {
    return nil, fmt.Errorf("insufficient daily data after signal date")
}
// Ganti evalDays → effectiveEvalDays di seluruh loop analyzeSignal:
// for d := 0; d < effectiveEvalDays && startIdx+d < len(reversed); d++
```

## File yang Kemungkinan Diubah

- `internal/service/backtest/excursion.go` — ubah boundary check dan loop baris ~222-290

## Acceptance Criteria

- [ ] Sinyal berumur < evalDays tetap dianalisis dengan partial window
- [ ] `result.OptimalExitDay` tidak melebihi jumlah hari data yang tersedia
- [ ] Sinyal tanpa data sama sekali setelah signal date masih di-reject
- [ ] ExcursionSummary.TotalSignals meningkat (sebelumnya membuang recent signals)

## Referensi

- `.agents/research/2026-04-02-09-bug-hunt-excursion-garch-hmm.md`
