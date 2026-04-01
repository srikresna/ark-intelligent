# TASK-013: Elliott Wave Automated Counter

**Priority:** MEDIUM  
**Cycle:** Siklus 3 — Fitur Baru  
**Estimated Complexity:** VERY HIGH  
**Research Ref:** `.agents/research/2026-04-01-09-ict-smc-wyckoff-elliott-features.md`

---

## Deskripsi

Implementasi algoritma otomatis untuk menghitung Elliott Wave dari data OHLCV. Engine harus mengidentifikasi current wave position, invalidation level, dan projected target. Ini adalah fitur paling kompleks di siklus ini — implementasikan secara bertahap.

## Konteks Teknis

### Foundation yang Sudah Ada
- `internal/service/ta/fibonacci.go` — CalcFibonacci() sudah ada, detect swing points
- `internal/service/ta/types.go` — FibonacciResult dengan swing high/low sudah ada
- `internal/service/ta/zones.go` — ZoneResult untuk entry zones

### File yang Perlu Dibuat
```
internal/service/elliott/
├── types.go      ← Wave, WaveCount, WaveCountResult structs
├── engine.go     ← Engine.Analyze(bars []ta.OHLCV) *WaveCountResult
├── zigzag.go     ← ZigZag algorithm untuk swing point detection
├── validator.go  ← Elliott Wave rules validator
└── projector.go  ← Wave projection calculator (Fibonacci targets)
```

### File yang Perlu Dimodifikasi
- `internal/adapter/telegram/handler_alpha.go` — tambah `/elliott` command
- `internal/adapter/telegram/formatter.go` — FormatElliottResult()

## Spesifikasi

### Types
```go
type WaveType string
const (
    Impulse     WaveType = "IMPULSE"
    Corrective  WaveType = "CORRECTIVE"
    Diagonal    WaveType = "DIAGONAL"
    ZigZag      WaveType = "ZIGZAG"
    Flat        WaveType = "FLAT"
    Triangle    WaveType = "TRIANGLE"
)

type Wave struct {
    Number     string   // "1", "2", "3", "4", "5", "A", "B", "C"
    Kind       WaveType
    Start      float64  // price level start
    End        float64  // price level end (0 = ongoing/projected)
    StartBar   int      // bar index
    EndBar     int      // bar index (-1 = current/ongoing)
    Direction  string   // "UP" | "DOWN"
    Retracement float64 // % retracement dari wave sebelumnya (untuk validation)
    FibRatio   float64  // rasio terhadap previous impulse wave
    Valid      bool     // false = rule violation terdeteksi
    Violation  string   // deskripsi violation jika ada
}

type WaveCountResult struct {
    Symbol          string
    Timeframe       string
    Degree          string   // "GRAND_SUPERCYCLE", "SUPERCYCLE", "CYCLE", "PRIMARY", "INTERMEDIATE", "MINOR", "MINUTE", "MINUETTE"
    Waves           []Wave
    CurrentWave     string   // "1", "2", "3", "4", "5", "A", "B", "C"
    WaveProgress    float64  // 0-100% progress dalam wave saat ini
    InvalidationLevel float64 // level yang membatalkan wave count ini
    Target1         float64  // konservatif target
    Target2         float64  // agresif target
    AlternateCount  *WaveCountResult // alternate scenario
    Confidence      string   // "HIGH", "MEDIUM", "LOW"
    Summary         string
    AnalyzedAt      time.Time
}
```

### Elliott Wave Rules (WAJIB divalidasi)
```
Rule 1: Wave 2 TIDAK BOLEH retracement > 100% dari Wave 1
Rule 2: Wave 3 TIDAK BOLEH menjadi wave terpendek (dibanding W1 dan W5)
Rule 3: Wave 4 TIDAK BOLEH overlap dengan territory Wave 1
         (kecuali untuk leading/ending diagonal)

Guidelines (tidak wajib tapi penting untuk confidence):
- Wave 2 biasa retrace 50% atau 61.8% dari Wave 1
- Wave 3 biasa 1.618x Wave 1 (paling sering)  
- Wave 4 biasa retrace 38.2% dari Wave 3
- Wave 5 biasa equal Wave 1, atau 0.618x Wave 1 jika Wave 3 extended
```

### ZigZag Algorithm
```go
// Deteksi swing point dengan minimum retracement threshold
// Parameter: bars []ta.OHLCV, minRetracement float64 (biasa 0.05 = 5%)
func detectZigZag(bars []ta.OHLCV, minRetracement float64) []SwingPoint

type SwingPoint struct {
    Index     int
    Price     float64
    IsHigh    bool // true = swing high, false = swing low
}
```

### Wave Projection
```go
// Berdasarkan Fibonacci extensions:
// Wave 3 target = Wave1Start + (Wave1Length × 1.618)
// Wave 5 target = Wave1Start + (Wave1Length × 1.000) atau × 1.618
// Wave C target = WaveAStart + WaveALength (equality)
```

## Telegram Command `/elliott`

```
〽️ ELLIOTT WAVE — EURUSD H4

📊 CURRENT COUNT: Wave 4 of 5 (PRIMARY degree)
📍 Progress: ~60% complete

〽️ WAVE STRUCTURE:
  ① 1.0640 → 1.0890 (+250 pips) ✅
  ② 1.0890 → 1.0770 (-120p, 48% ret) ✅
  ③ 1.0770 → 1.1020 (+250p, 1.00×W1) ✅
  ④ 1.1020 → 1.0895 (-125p, 50% ret) ← CURRENT

🎯 PROJECTIONS:
  Wave 4 end: 1.0860-1.0875 (38.2% ret zone)
  Wave 5 target: 1.1150-1.1210

🚫 INVALIDATION: Below 1.0770 (Wave 1 high)

⚠️  ALTERNATE: Could still be Wave B if count adjusted

💡 SUMMARY: Classic Wave 4 pullback setelah extended Wave 3.
   Entry opportunity di 1.0860-1.0875 untuk Wave 5 target 1.1150+.
```

## Implementasi Bertahap

**Phase 1 (MVP):**
- ZigZag swing detection
- Basic 5-wave impulse identification
- Rule validation (3 rules utama)
- Simple Fibonacci targets

**Phase 2:**
- Corrective patterns (A-B-C, ZigZag, Flat, Triangle)
- Multiple degrees analysis
- Alternate count generation
- Confidence scoring

## Acceptance Criteria

- [ ] Compile tanpa error
- [ ] ZigZag detection bekerja dengan synthetic data
- [ ] Semua 3 Elliott Wave rules divalidasi
- [ ] Output readable di mobile
- [ ] Confidence "LOW" jika bars < 50
- [ ] Alternate count ditampilkan saat confidence < 70%
- [ ] Min 5 unit tests (rules validation)

