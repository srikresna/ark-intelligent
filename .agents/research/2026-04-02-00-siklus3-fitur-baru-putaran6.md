# Research Siklus 3 — Fitur Baru (Putaran 6)
**Date:** 2026-04-02 00:00 WIB
**Focus:** Fitur baru — ICT PD Array/OTE, MTF SMC, Options Flow, Elliott Wave, Footprint

---

## Ringkasan Temuan

### ✅ Sudah Implemented
| Fitur | Lokasi |
|---|---|
| ICT (FVG, OrderBlock, CHoCH/BOS, LiqSweep, Killzone) | `internal/service/ict/` |
| SMC (BOS/CHoCH, premium/discount) | `internal/service/ta/smc.go` |
| Wyckoff (phases, events, accumulation/dist) | `internal/service/wyckoff/` |
| GEX (Gamma Exposure via Deribit) | `internal/service/gex/` |

### ❌ Gap yang Ditemukan

#### 1. ICT PD Array + OTE (Optimal Trade Entry)
- ICT engine sudah punya FVG, OB, CHoCH — tapi BELUM ada:
  - PD Array hierarchy (SIBI > FVG > OB > Breaker > Mitigation > RB > EQ)
  - OTE = Fibonacci 62%–79% dari swing retracement (entry presisi)
  - New Day Opening Gap (NDOG) / New Week Opening Gap (NWOG)
  - Liquidity run probability score
- Easy to add: extend `internal/service/ict/engine.go` + `fvg.go`

#### 2. Multi-Timeframe ICT/SMC Confluence
- Semua engine berjalan single-timeframe
- HTF bias + MTF confirmation missing
- Dua engine sudah ada → tinggal orchestrate 3 TF (H4 → H1 → M15)
- Confluence score sebagai composite signal

#### 3. Options Flow via Deribit
- GEX sudah ada, tapi kurang:
  - PCR dari Deribit (hanya CBOE PCR di sentiment)
  - Large trade scanner (whale flow)
  - IV term structure per expiry
  - Options sentiment score (composite)
- Deribit client sudah ada di `internal/service/marketdata/deribit/`

#### 4. Elliott Wave Engine
- Zero implementasi — tidak ada file sama sekali
- Bisa reuse swing detection dari `ict/swing.go`
- Rule-based: W1 impulse → W2 retrace → W3 extension → W4 → W5
- ABC correction detection
- New package: `internal/service/elliott/`

#### 5. Footprint / Delta Volume Approximation
- True footprint butuh tick data
- OHLCV approximation: BullDelta = (C-L)/(H-L)*Vol, BearDelta = (H-C)/(H-L)*Vol
- Delta bar chart via text/emoji → cocok untuk Telegram
- Crypto: Deribit trades endpoint bisa beri granularity lebih baik

---

## Prioritas

| Priority | Task | Effort |
|---|---|---|
| HIGH | ICT PD Array + OTE | M |
| HIGH | MTF ICT/SMC Confluence | M |
| HIGH | Options Flow via Deribit | M |
| MEDIUM | Elliott Wave Engine | L |
| MEDIUM | Footprint Delta Volume | M |
