# Research — Siklus 3: Fitur Baru (ICT, SMC, Wyckoff, Elliott)
**Date:** 2026-04-01 15:00 WIB
**Focus:** Smart Money / Price Action concepts — ICT, SMC, Wyckoff, Elliott Wave

---

## 1. Latar Belakang

Siklus 3 fokus pada fitur baru bernilai tinggi. Codebase sudah punya:
- `internal/service/ta/` — RSI, MACD, Bollinger, Ichimoku, SuperTrend, Fibonacci, divergence, patterns, confluence
- `internal/service/price/levels.go` — S/R pivot levels, swing highs/lows
- `internal/service/price/hmm_regime.go` — HMM 3-state regime detection
- `internal/service/microstructure/engine.go` — orderbook imbalance, taker flow, OI

**Gap besar:** Tidak ada ICT/SMC concepts (FVG, Order Block, BOS/CHOCH), Wyckoff phase, atau Elliott Wave. Ini adalah konsep yang paling banyak dipakai oleh institutional forex traders modern.

---

## 2. Analisis Peluang per Topik

### 2a. ICT — Fair Value Gap (FVG) & Order Block

**Apa itu:**
- **Fair Value Gap (FVG)**: 3-candle pattern di mana candle tengah tidak terisi gap antara low candle 1 dan high candle 3 (bullish FVG) atau high candle 1 dan low candle 3 (bearish FVG). Ini adalah area di mana price "tidak diperdagangkan" — pasar akan sering kembali ke sini.
- **Order Block (OB)**: Candle terakhir sebelum impulse move besar. Ini adalah zona di mana institutional orders diasumsikan ditempatkan. Bullish OB = last bearish candle sebelum rally. Bearish OB = last bullish candle sebelum drop.
- **Breaker Block**: Order block yang sudah ditembus — menjadi area berlawanan (bullish OB menjadi resistance setelah tertembus).
- **Liquidity Sweep**: Pengambilan liquidity di atas swing high / di bawah swing low sebelum reversal.

**Integrasi ke codebase:**
- Buat `internal/service/ta/ict.go` — FVGDetector, OrderBlockDetector
- FVG hanya butuh OHLCV bars → pure computation, tidak ada deps baru
- OB butuh swing detection (sudah ada di levels.go sebagai `findSwingHighs/Lows`)
- Output bisa ditambahkan ke `IndicatorSnapshot` sebagai `ICT *ICTResult`
- Tampilkan di `/cta` command dan `/levels` command

**Kompleksitas:** Medium (2-4h) — pure algo, no external API

### 2b. SMC — Break of Structure (BOS) & Change of Character (CHOCH)

**Apa itu:**
- **Break of Structure (BOS)**: Konfirmasi trend continuation — price menembus swing high (bullish BOS) atau swing low (bearish BOS) sebelumnya.
- **Change of Character (CHOCH)**: Pertanda trend reversal — bullish CHOCH ketika bearish trend menembus swing high, bearish CHOCH ketika bullish trend menembus swing low.
- **Premium/Discount Zones**: Area di atas/bawah 50% dari trading range saat ini. ICT menggunakan Fibonacci 50% untuk membagi zona premium (beli di discount, jual di premium).

**Integrasi ke codebase:**
- Buat `internal/service/ta/smc.go`
- BOS/CHOCH butuh swing high/low history — bisa leverage `CalcATR` dan swing detection dari `fibonacci.go`
- Premium/Discount menggunakan range dari swing extreme terbaru
- Output masuk ke `ConfluenceResult` sebagai signal tambahan
- Tampilkan di `/cta` dan tingkatkan confluence score

**Kompleksitas:** Medium (2-4h)

### 2c. Wyckoff Phase Detection

**Apa itu:**
- Wyckoff Analysis memetakan siklus market ke 4 fasa: Accumulation, Markup, Distribution, Markdown
- Sub-events: PS (Preliminary Support/Supply), SC (Selling Climax), AR (Automatic Rally), ST (Secondary Test), Spring/Upthrust, Creek, SOS (Sign of Strength)
- Deteksi berbasis volume + price structure

**Gap vs Codebase Existing:**
- HMM (`hmm_regime.go`) sudah deteksi 3-state: bull, bear, sideways
- HMM state "sideways" bisa menjadi basis untuk Wyckoff Accumulation/Distribution
- Wyckoff lebih explicit: butuh volume profile (sudah ada VP via Python) + swing structure

**Strategi integrasi:**
- Buat `internal/service/price/wyckoff.go` — phase classifier
- Input: daily bars (OHLCV), volume (sudah ada di `domain.DailyPrice`)
- Deteksi Spring/Upthrust: swing low yang briefly breaches support lalu recover
- Phase scoring 0-100 berdasarkan karakteristik volume + price action
- Integrasikan ke `/quant` command sebagai tambahan HMM output

**Kompleksitas:** Large (4h+) — tapi bisa start dengan phase detection sederhana

### 2d. Elliott Wave Basic Automated Counter

**Apa itu:**
- Teori Elliott: pasar bergerak dalam pola 5 impuls wave + 3 koreksi wave (ABC)
- Wave 3 biasanya terpanjang, Wave 4 tidak boleh overlap Wave 1
- Fibonacci relationships: Wave 3 = 1.618x Wave 1, Wave 5 = 0.618x Wave 1, dll.

**Tantangan:**
- Subjektif — banyak cara counting berbeda
- Automated counting sulit dan error-prone
- Lebih baik: detect possible wave counts dan beri confidence score

**Integrasi:**
- Buat `internal/service/ta/elliott.go` — WaveCounter struct
- Leverage existing `fibonacci.go` yang sudah ada di ta service
- Focus pada swing detection → cari pola ABCDE dari swing highs/lows
- Output sebagai optional field di `IndicatorSnapshot`

**Kompleksitas:** Large (4h+) — pecah jadi 2 task (swing labeling + wave validation)

### 2e. ICT Killzone — Session-Based Analysis

**Apa itu:**
- ICT Killzones = windows of time with highest institutional activity:
  - London Open: 08:00–10:00 UTC
  - New York Open: 13:00–15:00 UTC (overlap terkuat)
  - New York Close: 20:00–22:00 UTC
  - London Close: 15:00–16:00 UTC
  - Asia Session: 00:00–04:00 UTC
- Pair KZ dengan FVG/OB untuk high-probability entries

**Integrasi:**
- Buat `internal/service/ta/killzone.go` — KillzoneClassifier
- Pure time-based, tidak butuh data baru
- Integrasikan ke `/cta` output: "Saat ini dalam London Kill Zone — perhatikan FVG di H1"
- Tambah context ke ATR interpretation (volatility lebih tinggi saat KZ)

**Kompleksitas:** Small (<2h)

---

## 3. Prioritisasi Task

| # | Fitur | Impact | Complexity | Priority |
|---|-------|--------|------------|---------|
| 1 | ICT Fair Value Gap + Order Block | Sangat tinggi | M | **HIGH** |
| 2 | SMC BOS/CHOCH + Premium-Discount | Tinggi | M | **HIGH** |
| 3 | ICT Killzone Classifier | Tinggi | S | **HIGH** |
| 4 | Wyckoff Phase Detection (basic) | Sedang | L | **MEDIUM** |
| 5 | Elliott Wave Swing Labeler (fase 1) | Sedang | L | **MEDIUM** |

---

## 4. Referensi Teknis

- ICT FVG: 3-candle pattern — `bars[i-2].Low > bars[i].High` (bearish FVG) atau `bars[i-2].High < bars[i].Low` (bullish FVG) dari perspective bars newest-first
- OB: last candle opposite direction sebelum strong move (≥1.5x ATR range)
- BOS: swing high/low sudah ada di `levels.go` via `findSwingHighs/Lows`
- Wyckoff Spring: swing low menembus support < 0.5x ATR, close kembali di atas support
- Elliott rules: Wave 2 < 100% Wave 1, Wave 4 tidak overlap Wave 1

---

## 5. Kesimpulan

Siklus 3 mengidentifikasi 5 task baru bernilai tinggi dalam area ICT/SMC/Wyckoff. 3 pertama bisa dimulai segera karena pure algorithmic computation dengan data yang sudah ada. FVG + OB adalah yang paling dibutuhkan komunitas forex modern dan akan meningkatkan nilai /cta command secara signifikan. Semua implementasi berbasis OHLCV existing tanpa kebutuhan API baru.
