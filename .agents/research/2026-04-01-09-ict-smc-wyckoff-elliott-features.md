# Research: ICT, SMC, Wyckoff, Elliott Wave & Advanced Features
**Tanggal:** 2026-04-01  
**Siklus:** 3/5 — Fitur Baru  
**Fokus:** ICT, Smart Money Concepts, Wyckoff, Elliott Wave, Footprint, GEX

---

## Ringkasan Eksekutif

Riset siklus 3 mengidentifikasi peluang implementasi 5 blok fitur besar yang belum ada di codebase: ICT/SMC Price Action, Wyckoff Structure, Elliott Wave, Footprint/Delta Analysis, dan Gamma Exposure (GEX). Tidak ada satupun dari fitur ini yang sudah diimplementasi — semua masih di FEATURE_INDEX.md sebagai "planned". Seluruh fitur bisa diimplementasi menggunakan data OHLCV yang sudah ada (TwelveData/Polygon) tanpa sumber data berbayar baru.

---

## Analisis Codebase

### Foundation yang Sudah Ada
- `internal/service/ta/` — engine TA lengkap dengan OHLCV type, patterns, divergence, zones
- `internal/service/ta/patterns.go` — candlestick detection dengan helpers (bodySize, upperShadow, dll)
- `internal/service/ta/types.go` — IndicatorSnapshot, OHLCV, CandlePattern structs
- `internal/service/ta/engine.go` — ComputeSnapshot() + ComputeFull() orchestrator
- `internal/adapter/telegram/handler_alpha.go` — pattern untuk registrasi command baru (RegisterCommand)
- `internal/service/microstructure/engine.go` — contoh engine baru yang standalone

### Gap yang Ditemukan
1. **Tidak ada ICT/SMC engine** — Fair Value Gap, Order Block, Breaker Block, Killzone belum exist
2. **Tidak ada Wyckoff engine** — Accumulation/Distribution schematics belum exist
3. **Tidak ada Elliott Wave engine** — wave counting otomatis belum exist  
4. **Tidak ada Footprint/Delta engine** — tick data analysis belum exist
5. **Tidak ada GEX engine** — Gamma Exposure dari options market belum exist
6. **Command `/ict`, `/smc`, `/wyckoff`, `/elliott` belum ada** di handler.go

### Arsitektur yang Direkomendasikan
Setiap fitur baru harus ikuti pola yang sama dengan `service/microstructure/`:
```
internal/service/ict/       ← ICT + SMC engine
internal/service/wyckoff/   ← Wyckoff structure engine
internal/service/elliott/   ← Elliott Wave counter
```
Dan untuk yang butuh lebih banyak data:
```
internal/service/orderflow/ ← Delta + Footprint analysis
```

---

## Detail Temuan Per Fitur

### 1. ICT / Smart Money Concepts (SMC)

**Fair Value Gap (FVG):**
- Deteksi: candle[i-2].High < candle[i].Low (bullish FVG) atau sebaliknya
- Data: OHLCV sudah ada, bisa detect pada semua timeframe
- Output: list FVG zones dengan koordinat harga, filled/unfilled status

**Order Block:**
- Deteksi: last bearish candle sebelum impulsive bullish move (bullish OB) atau sebaliknya
- Kriteria: candle dengan body terbesar di swing, diikuti 3+ candles yang bergerak terus
- Data: OHLCV bars sudah cukup

**Breaker Block:**
- Order Block yang sudah dibreak price (struktur terbalik)
- Derived dari Order Block yang sudah teridentifikasi

**Killzone Timing:**
- Asian: 00:00-04:00 UTC
- London: 07:00-10:00 UTC  
- New York: 12:00-16:00 UTC
- Implementasi: pure time logic, bisa detect dari timestamp bar

**Change of Character (CHOCH) / Break of Structure (BOS):**
- CHOCH: swing high/low yang terbreak pertama kali di trend baru
- BOS: continuation break di trend yang sudah ada
- Data: perlu swing point detection (sudah ada dasar di Fibonacci swing detection)

**Liquidity Sweep:**
- Price briefly breaks swing high/low lalu reversal
- Deteksi: price goes above previous high tapi close di bawahnya

### 2. Wyckoff Structure Analysis

**Accumulation Schematic:**
- Phase A: PS (Preliminary Support) → SC (Selling Climax) → AR (Automatic Rally) → ST (Secondary Test)
- Phase B: Building Cause
- Phase C: Spring → Test of Spring
- Phase D: SOS (Sign of Strength) → LPS (Last Point of Support)
- Phase E: Markup

**Deteksi Algoritmik:**
- Volume analysis pada swing lows (high volume = SC, low volume = ST)
- Price range contraction (building cause = narrowing range)
- Spring: brief break below support with fast recovery
- Sign of Strength: breakout dengan volume tinggi

**Distribution Schematic:** Mirror dari Accumulation

**Data Requirements:** OHLCV + Volume sudah cukup untuk deteksi dasar

### 3. Elliott Wave Counter

**Rules Ketat:**
- Wave 2 tidak boleh retracement > 100% Wave 1
- Wave 3 tidak boleh yang terpendek
- Wave 4 tidak boleh overlap territory Wave 1 (kecuali diagonal)
- Wave 5 sering diverge dengan momentum

**Fibonacci Relationships:**
- Wave 2 biasa retrace 61.8% atau 50% dari Wave 1
- Wave 3 biasa 1.618x Wave 1
- Wave 4 biasa retrace 38.2% dari Wave 3
- Wave 5 biasa equal Wave 1 atau 0.618x Wave 1

**Deteksi Algoritmik:**
- Perlu swing point identification (ZigZag-like algorithm)
- Sudah ada Fibonacci engine di `ta/fibonacci.go` — bisa jadi dasar
- Validasi rules Wave 1-5 setelah swing teridentifikasi

**Output:** Current wave count, invalidation level, projected target

### 4. Footprint / Delta Analysis

**Delta:** Aggressive buys - Aggressive sells per candle
- Positive delta = buyer controlled bar
- Negative delta = seller controlled bar
- Delta divergence dengan price = potential reversal

**Volume Profile Delta:**
- Per price level: berapa buy vs sell terjadi
- Point of Control (POC): level dengan volume tertinggi

**Data Requirement:** 
- Butuh tick data atau at-minute trade data untuk real footprint
- Alternatif: estimasi dari OHLCV + tick rule (jika close > open → buy volume estimasi)
- Bybit sudah punya GetRecentTrades() di marketdata/bybit — tapi hanya beberapa ratus trades terakhir
- Untuk forex: tick data lebih sulit didapat (DTCC/Dukascopy data bisa dikonsider)

**Rekomendasi:** Implementasi "estimated delta" dari OHLCV dulu (approximation)

### 5. Gamma Exposure (GEX)

**Konsep:**
- Gamma = rate of change of delta option terhadap perubahan underlying
- Dealer GEX = total gamma exposure market maker (dari options positions)
- Positive GEX zone: market maker jual saat naik, beli saat turun → volatility damping
- Negative GEX zone: market maker jual saat turun, beli saat naik → volatility amplifying

**Data Sources (Gratis):**
- Crypto: Deribit options data via API (gratis untuk BTC/ETH)
- Forex: Tidak ada GEX standar untuk forex (konsep equity-native)
- CME FX Options: data terbatas gratis

**Rekomendasi:** Implement untuk crypto pairs dulu via Deribit public API
- `GET https://www.deribit.com/api/v2/public/get_book_summary_by_currency`
- `GET https://www.deribit.com/api/v2/public/get_instruments` (untuk daftar strikes)
- Perhitungan GEX: Σ(gamma × OI × contract_size × spot_price²)

---

## Gap & Prioritas Implementasi

| Fitur | Complexity | Data Availability | Impact | Priority |
|-------|-----------|-------------------|--------|----------|
| ICT Fair Value Gap | LOW | ✅ Existing OHLCV | HIGH | **P1** |
| ICT Order Block | MEDIUM | ✅ Existing OHLCV | HIGH | **P1** |
| SMC CHOCH/BOS | MEDIUM | ✅ Existing OHLCV | HIGH | **P1** |
| Wyckoff Structure | HIGH | ✅ OHLCV + Volume | MEDIUM | **P2** |
| Elliott Wave | VERY HIGH | ✅ OHLCV | MEDIUM | **P2** |
| Footprint Delta | MEDIUM | ⚠️ OHLCV estimate | MEDIUM | **P2** |
| GEX (Crypto) | MEDIUM | ✅ Deribit free API | HIGH | **P1** |

---

## Kesimpulan

Tiga task prioritas tinggi yang harus dibuat:
1. **ICT/SMC Engine** — FVG + Order Block + CHOCH/BOS (P1, data sudah ada)
2. **GEX Engine untuk Crypto** (P1, Deribit API gratis)
3. **Wyckoff Detection Engine** (P2, data sudah ada)
4. **Elliott Wave Counter** (P2, complex tapi high value)
5. **Estimated Delta dari OHLCV** (P2, tanpa tick data)

