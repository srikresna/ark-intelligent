# TASK-089: Elliott Wave Swing Labeler (Fase 1 — Swing Detection & Basic Rule Validation)

**Priority:** medium
**Type:** feature
**Estimated:** L (4h+)
**Area:** internal/service/ta
**Created by:** Research Agent
**Created at:** 2026-04-01 15:00 WIB
**Siklus:** Fitur (Siklus 3)

## Deskripsi
Implementasi fase 1 Elliott Wave analysis di `internal/service/ta/elliott.go`: automated swing labeling + rule-based wave count validation. Output berupa possible wave position dengan confidence score.

## Konteks
Elliott Wave adalah salah satu framework paling komprehensif untuk price cycle analysis. Automated wave counting sulit dan subjektif, sehingga fase 1 fokus pada swing detection + validasi aturan dasar Elliott (tidak terlanggar) tanpa klaim wave count definitif. Fase 2 (struktur nested, degree labeling) bisa dikerjakan siklus berikutnya.

Ref: `.agents/research/2026-04-01-15-fitur-baru-ict-smc-wyckoff.md#2d`

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Buat `internal/service/ta/elliott.go` dengan:
  - Struct `SwingPoint` berisi: Type ("HIGH"/"LOW"), Price float64, BarIndex int
  - Struct `WaveCandidate` berisi:
    - WaveNumber int (1–5 atau "A"/"B"/"C")
    - StartPrice, EndPrice float64
    - StartBar, EndBar int
    - FibRatio float64 (rasio terhadap wave sebelumnya)
  - Struct `ElliottResult` berisi:
    - Swings []SwingPoint (swing points terdeteksi, max 20 terbaru)
    - PossibleWavePosition string ("W1","W2","W3","W4","W5","WA","WB","WC","UNCLEAR")
    - WaveCandidates []WaveCandidate (kandidat wave dari swing terbaru)
    - RulesViolated []string (aturan Elliott yang dilanggar jika ada)
    - Confidence int (0-100)
    - Bias string ("BULLISH"/"BEARISH"/"NEUTRAL")
  - `DetectSwings(bars []OHLCV, sensitivity int) []SwingPoint`:
    - sensitivity: jumlah bars kanan/kiri untuk konfirmasi swing (default 3)
    - Swing High: bars[i].High > bars[i-sensitivity..i+sensitivity].High semua
    - Swing Low: bars[i].Low < bars[i-sensitivity..i+sensitivity].Low semua
  - `AnalyzeElliott(bars []OHLCV) *ElliottResult`:
    - Panggil DetectSwings dengan sensitivity=3
    - Dari swing terbaru, coba fitting ke impulse pattern (5 swings)
    - Validasi aturan Elliott:
      - Rule 1: Wave 2 tidak boleh retrace lebih dari 100% Wave 1
      - Rule 2: Wave 3 tidak boleh menjadi wave terpendek di antara W1, W3, W5
      - Rule 3: Wave 4 tidak boleh overlap territory Wave 1
    - Fibonacci guideline check: W3 ≥ 1.618×W1, W2 retrace 50-61.8% W1
    - Assign confidence: 100% jika semua rules passed + Fib ratios on target
- [ ] Tambahkan field `Elliott *ElliottResult` ke `IndicatorSnapshot` di `types.go`
- [ ] Panggil dari `engine.go` hanya untuk daily/weekly timeframe (skip untuk < 4h untuk performance)
- [ ] Display sederhana di /cta: "Elliott: Kemungkinan Wave 3 (confidence 68%) — Bullish bias"
- [ ] Unit test: synthetic 5-wave impulse yang valid + 1 kasus violation

## File yang Kemungkinan Diubah
- `internal/service/ta/elliott.go` (baru)
- `internal/service/ta/types.go`
- `internal/service/ta/engine.go`
- Formatter /cta output

## Referensi
- `.agents/research/2026-04-01-15-fitur-baru-ict-smc-wyckoff.md`
- `internal/service/ta/fibonacci.go` (leverage swing detection pattern)
