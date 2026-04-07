# Research Siklus 3: Feature Deep Dive — ICT/SMC/Wyckoff/Elliott/GEX/OrderFlow

**Date:** 2026-04-06
**Cycle:** 3
**Focus:** Fitur aktif yang sudah ada — gap dan enhancement peluang

---

## Ringkasan Eksekutif

Semua fitur utama (ICT, SMC, Wyckoff, Elliott, GEX, OrderFlow) sudah terimplementasi
dengan service layer penuh dan Telegram commands terdaftar. Namun terdapat 4 gap
signifikan yang ditemukan melalui analisis mendalam:

1. **COT Seasonal — service built, TIDAK ada command** (dead code)
2. **ICT IPDA — tidak diimplementasikan** (konsep ICT fundamental)
3. **ICT Macro Windows — tidak diimplementasikan** (intraday timing precision)
4. **Elliott Wave ABC Corrective — Phase 2 belum ada** (setengah teori Elliott)

---

## Status Per Fitur

### ICT (Inner Circle Trader) — `/ict`
**Status:** Implemented ✅ dengan gaps

**Yang sudah ada:**
- FVG (Fair Value Gap) detection — bullish + bearish
- Order Block detection (Bullish + Bearish OB)
- Breaker Block (OB yang dibreakthrough menjadi resistance/support baru)
- BOS (Break of Structure) + CHoCH (Change of Character)
- Liquidity Sweep detection (wick melalui swing high/low)
- Killzone detection (Asian/London Open/NY AM/London Close)
- Inline keyboard navigasi symbol + timeframe

**Yang BELUM ada (genuine gaps):**
1. **IPDA (Interbank Price Delivery Algorithm)** — 20/40/60-day delivery range
   - IPDA menentukan apakah harga dalam "premium zone" (upper 50%) atau "discount zone" (lower 50%)
   - ICT traders menggunakan IPDA untuk filter entry: buy di discount, sell di premium
   - File `service/ict/engine.go` dan `ta/ict.go` tidak ada IPDA sama sekali

2. **ICT Macro Windows** — intraday timing windows berbeda dari Killzone
   - Killzone = broad session windows (07:00-10:00 London)
   - ICT Macros = precise algorithmic delivery windows (8:50-9:10 AM ET, etc.)
   - 6 Macro windows per trading day, masing-masing 20-30 menit
   - Di dalam Macro, FVG lebih sering terisi karena algoritma bank aktif

3. **ICT Silver Bullet** — specific setup dalam 10:00-11:00 AM NY window
   - FVG yang terbentuk + terisi dalam window ini = high-probability entry
   - Belum ada detection pattern ini

### SMC (Smart Money Concepts) — `/smc`
**Status:** Implemented ✅, quality good

SMC handler mengkombinasikan `ta.SMCResult` (BOS/CHOCH/zones) dengan ICT analysis
(FVG + Order Blocks). Coverage sudah cukup lengkap untuk SMC dasar.

**Gap minor:** Tidak ada premium/discount zone visualization berdasarkan HTF range.

### Wyckoff — `/wyckoff`
**Status:** Implemented ✅, comprehensive

Phase detection (Accumulation A-E, Distribution A-E), spring/upthrust events,
cause/effect projection. Quality implementation.

**Tidak ada gap signifikan.**

### Elliott Wave — `/elliott`
**Status:** Partial ⚠️ — Phase 1 MVP only

Package header eksplisit menyatakan: *"Only Phase 1 (MVP) is implemented"*

**Yang sudah ada (Phase 1):**
- ZigZag swing detection
- 5-wave impulse identification (waves 1-2-3-4-5)
- Rule validation (Wave 3 tidak paling pendek, Wave 2 tidak melewati Wave 1, etc.)
- Fibonacci target projection (T1, T2)
- Alternate count
- Confidence scoring

**Yang BELUM ada (Phase 2 — genuine gap):**
- **ABC corrective wave count** — ini setengah dari Elliott Wave theory
  - ZigZag ABC (5-3-5 structure)
  - Flat ABC (3-3-5 structure)
  - Triangle (A-B-C-D-E)
- Tanpa corrective count, `/elliott` hanya bisa menghitung trend, bukan koreksi
- Ini membatasi utility signifikan bagi trader yang ingin tahu "Wave 2 atau Wave 4 sudah selesai?"

### GEX (Gamma Exposure) — `/gex`, `/ivol`, `/skew`
**Status:** Implemented ✅, robust

Deribit options data, GEX flip level, put/call walls, Max Pain, IV Surface, skew.

**Gap minor (tidak immediate priority):**
- Vanna/Charm exposure (2nd-order Greeks dealer hedging flow) — advanced feature
- Tidak ada fitur gratis untuk equity GEX (hanya crypto via Deribit)

### Order Flow — `/orderflow`
**Status:** Implemented ✅

Estimated delta (tick rule), cumulative delta divergence, absorption patterns,
point of control. Coverage cukup untuk forex (tanpa tick data).

**Gap:** Footprint chart visualization (per-bar bid/ask volume by price level)
tidak ada — ini membutuhkan tick data yang tidak gratis untuk forex.

### COT Seasonal — NO COMMAND ❌
**Status:** Service built, ZERO user-facing exposure

`internal/service/cot/seasonal.go` sudah memiliki:
- `SeasonalEngine` dengan `Analyze()` dan `AnalyzeAll()`
- COT positioning seasonal patterns per currency per ISO week
- Deviation scoring, Z-score, seasonal bias classification
- 5+ tahun historical data support

**TIDAK ADA** command handler, formatter, atau RegisterCommand apapun.
Ini adalah dead code — functionality lengkap yang tidak pernah bisa diakses user.

---

## Rekomendasi Task (Prioritas)

| Task | Gap | Effort | Priority |
|---|---|---|---|
| TASK-010 | Expose COT Seasonal via `/cotseasonal` | S | HIGH |
| TASK-011 | ICT IPDA Range Detection | M | HIGH |
| TASK-012 | ICT Macro Windows Detection | S | MEDIUM |
| TASK-013 | Elliott Wave ABC Corrective Count | L | MEDIUM |

---

## Catatan Teknis

- `service/ict/engine.go` (182 lines) — engine tipis, mudah di-extend
- `service/ta/ict.go` (585 lines) — ini canonical impl untuk FVG/OB; IPDA harus ditambah di sini
- `service/elliott/engine.go` (277 lines) — `fitImpulse()` harus dilengkapi dengan `fitCorrective()`
- `service/cot/seasonal.go` fully built — hanya butuh handler + formatter
- Semua fitur menggunakan pola `With*` injection + `RegisterCommand` yang konsisten

