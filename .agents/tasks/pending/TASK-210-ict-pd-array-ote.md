# TASK-210: ICT PD Array Hierarchy + Optimal Trade Entry (OTE)

**Priority:** high
**Type:** feature
**Estimated:** M
**Area:** internal/service/ict/

## Deskripsi

Extend ICT engine dengan dua konsep lanjutan: PD Array hierarchy dan Optimal Trade Entry (OTE). Keduanya adalah fondasi ICT yang digunakan untuk filter entry presisi.

## PD Array Hierarchy

PD Array = price delivery array, urutan "kualitas" level untuk entry:
```
1. SIBI/BISI (Single/Double Wick imbalance) — highest quality
2. Fair Value Gap (FVG) — sudah ada
3. Order Block (OB) — sudah ada
4. Breaker Block (failed OB yang flipped)
5. Mitigation Block
6. Rejection Block
7. EQ (50% equilibrium of range)
```

Tambahkan scoring: setiap level ICT mendapat `PDScore int (1-7)` berdasarkan tipe.

## Optimal Trade Entry (OTE)

OTE = Fibonacci retracement 62%–79% dari impulse swing.
- Setelah CHoCH/BOS dikonfirmasi → hitung OTE level dari swing low ke swing high (bullish) atau sebaliknya
- OTE zone: entry dianggap "optimal" bila harga retrace ke 62-79%
- Tampilkan OTE zone di output `/ict`

## New Day/Week Opening Gap

- NDOG = gap antara Friday close dan Monday open
- NWOG = gap antara previous session close dan current session open
- ICT mengajarkan price sering "fill the gap" sebelum trending
- Simpan di `ICTResult.OpeningGaps []GapZone`

## File Changes

- `internal/service/ict/fvg.go` — Add SIBI/BISI detection
- `internal/service/ict/orderblock.go` — Add Breaker + Mitigation Block detection
- `internal/service/ict/ote.go` — NEW: OTE Fibonacci zone calculator
- `internal/service/ict/gaps.go` — NEW: NDOG/NWOG gap detection
- `internal/service/ict/types.go` — Add PDScore, OTEZone, GapZone types
- `internal/service/ict/engine.go` — Wire new steps into Analyze()
- `internal/adapter/telegram/formatter_ict.go` — Display PD score, OTE zone, gaps

## Acceptance Criteria

- [ ] SIBI/BISI imbalance wicks detected (single candle rejection wick)
- [ ] Breaker Block = failed OB that price broke through, now acting as resistance/support
- [ ] OTE zone calculated after each CHoCH/BOS (62-79% fib of last impulse)
- [ ] NDOG/NWOG detected when session opens with a gap > 0.05% from prev close
- [ ] PD Array score (1-7) shown per zone in /ict output
- [ ] go build ./... clean, go vet ./... clean
