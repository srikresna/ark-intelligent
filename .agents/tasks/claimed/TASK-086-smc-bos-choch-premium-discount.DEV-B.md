# TASK-086: SMC Break of Structure (BOS) & Change of Character (CHOCH)

**Priority:** high
**Type:** feature
**Estimated:** M (2-4h)
**Area:** internal/service/ta
**Created by:** Research Agent
**Created at:** 2026-04-01 15:00 WIB
**Siklus:** Fitur (Siklus 3)

## Deskripsi
Implementasi Smart Money Concepts (SMC): Break of Structure (BOS), Change of Character (CHOCH), dan Premium/Discount zone classifier di `internal/service/ta/smc.go`. Integrasikan ke confluence scoring.

## Konteks
BOS dan CHOCH adalah konsep kunci SMC yang dipakai institutional traders untuk menentukan trend continuation vs reversal. Premium/Discount zone (berdasarkan 50% Fibonacci dari current range) menentukan apakah price saat ini dalam zona ideal untuk entry. Sangat relevan untuk trader yang sudah familiar dengan ICT/SMC.

Ref: `.agents/research/2026-04-01-15-fitur-baru-ict-smc-wyckoff.md#2b`

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Buat `internal/service/ta/smc.go` dengan:
  - Struct `StructurePoint` berisi: Type (SWING_HIGH/SWING_LOW), Price float64, BarIndex int
  - Struct `StructureBreak` berisi: Type (BOS/CHOCH), Direction (BULLISH/BEARISH), Level float64, ConfirmedAt int, PriorTrend string
  - Struct `PremiumDiscountResult` berisi: Zone (PREMIUM/DISCOUNT/EQUILIBRIUM), RangeHigh, RangeLow, MidPoint, CurrentPosition float64
  - Struct `SMCResult` berisi: Breaks []StructureBreak (max 3 terbaru), PremiumDiscount PremiumDiscountResult, LastStructureTrend string
  - `DetectStructureBreaks(bars []OHLCV) SMCResult`:
    - Identifikasi swing highs/lows dari bars (min 5 bars lookback per swing)
    - BOS bullish: price close di atas swing high sebelumnya, trend sebelumnya sudah bullish
    - BOS bearish: price close di bawah swing low sebelumnya, trend sebelumnya bearish
    - CHOCH bullish: price close di atas swing high, tapi trend sebelumnya BEARISH (reversal signal)
    - CHOCH bearish: price close di bawah swing low, tapi trend sebelumnya BULLISH (reversal signal)
  - `ClassifyPremiumDiscount(bars []OHLCV, lookback int) PremiumDiscountResult`:
    - Gunakan recent range high/low (lookback bars) sebagai batas
    - Midpoint = (High + Low) / 2
    - PREMIUM jika current price > midpoint + 10% range, DISCOUNT jika < midpoint - 10%, else EQUILIBRIUM
- [ ] Tambahkan field `SMC *SMCResult` ke `IndicatorSnapshot` di `types.go`
- [ ] Panggil dari `engine.go` dalam `ComputeSnapshot()`
- [ ] Tambahkan SMC signal ke `confluence.go` sebagai TASignal dengan weight 0.15
  - BOS = signal continuation (+0.5 atau -0.5)
  - CHOCH = signal reversal (berlawanan dengan prior trend)
- [ ] Display di /cta output: tampilkan CHOCH/BOS terakhir + Premium/Discount zone

## File yang Kemungkinan Diubah
- `internal/service/ta/smc.go` (baru)
- `internal/service/ta/types.go` (tambah SMCResult)
- `internal/service/ta/engine.go` (call DetectStructureBreaks)
- `internal/service/ta/confluence.go` (tambah SMC signal)
- Formatter /cta output

## Referensi
- `.agents/research/2026-04-01-15-fitur-baru-ict-smc-wyckoff.md`
