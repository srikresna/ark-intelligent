# TASK-088: Wyckoff Phase Detector (Basic)

**Priority:** medium
**Type:** feature
**Estimated:** L (4h+)
**Area:** internal/service/price
**Created by:** Research Agent
**Created at:** 2026-04-01 15:00 WIB
**Siklus:** Fitur (Siklus 3)

## Deskripsi
Implementasi Wyckoff phase classifier di `internal/service/price/wyckoff.go` — deteksi fase Accumulation, Markup, Distribution, Markdown dari daily bars + volume, dengan identifikasi key events (Spring, Upthrust, SOS/SOW). Integrasikan ke output `/quant`.

## Konteks
Wyckoff Analysis adalah metodologi klasik untuk membaca siklus market berdasarkan price-volume relationship. HMM yang sudah ada (`hmm_regime.go`) deteksi bull/bear/sideways, tapi Wyckoff memberikan konteks lebih kaya: sideways bisa Accumulation (mau naik) atau Distribution (mau turun). Ini relevan terutama untuk weekly/daily timeframe.

Ref: `.agents/research/2026-04-01-15-fitur-baru-ict-smc-wyckoff.md#2c`

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Buat `internal/service/price/wyckoff.go` dengan:
  - Struct `WyckoffEvent` berisi: Type string, Price float64, BarIndex int, Description string
    - Types: "PS" (Preliminary Support/Supply), "SC" (Selling/Buying Climax), "AR" (Automatic Rally/Reaction), "ST" (Secondary Test), "SPRING", "UPTHRUST", "SOS" (Sign of Strength), "SOW" (Sign of Weakness), "LPS" (Last Point of Support), "LPSY"
  - Struct `WyckoffResult` berisi:
    - Phase string ("ACCUMULATION" / "MARKUP" / "DISTRIBUTION" / "MARKDOWN" / "UNCERTAIN")
    - SubPhase string ("PHASE_A" / "PHASE_B" / "PHASE_C" / "PHASE_D" / "PHASE_E")
    - Confidence int (0-100)
    - KeyEvents []WyckoffEvent (terdeteksi dalam 60 bars terakhir)
    - SupportZone float64 (approximate trading range low)
    - ResistanceZone float64 (approximate trading range high)
    - Interpretation string (human-readable summary)
  - `AnalyzeWyckoff(bars []domain.DailyPrice) *WyckoffResult`:
    - Butuh min 60 bars
    - Phase detection logic:
      - Cek volatility mengecil (ATR trend) → menandakan ranging/consolidation
      - Volume pada swing lows vs swing highs (high vol di low = SC, menandakan Accumulation)
      - Spring: swing low brief di bawah support dengan volume tinggi + recover
      - Upthrust: swing high brief di atas resistance + reject + volume rendah
      - SOS: strong rally dengan expanding volume setelah Spring/LPS
    - Confidence score: jumlah karakteristik yang match / total karakteristik × 100
- [ ] Integrasikan ke quant service output (`/quant` command):
  - Tambahkan Wyckoff section setelah HMM output
  - Format: "Wyckoff: ACCUMULATION Phase C (Spring terdeteksi, SOS dikonfirmasi) — Confidence: 72%"
- [ ] Unit test dengan synthetic daily bars (accumulation scenario)

## File yang Kemungkinan Diubah
- `internal/service/price/wyckoff.go` (baru)
- `internal/service/price/types.go` atau `context.go` (tambah WyckoffResult)
- Quant handler atau formatter `/quant`

## Referensi
- `.agents/research/2026-04-01-15-fitur-baru-ict-smc-wyckoff.md`
- `internal/service/price/hmm_regime.go` (pola integrasi)
