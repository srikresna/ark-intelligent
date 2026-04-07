# TASK-211: Multi-Timeframe ICT/SMC Confluence Engine

**Priority:** high
**Type:** feature
**Estimated:** M
**Area:** internal/service/ict/, internal/adapter/telegram/handler_ict.go

## Deskripsi

Tambahkan MTF (Multi-Timeframe) confluence analysis ke `/ict` command. Saat ini ICT dan SMC berjalan single-timeframe. MTF adalah inti dari metodologi ICT: HTF menentukan bias, LTF menentukan entry.

## Logic MTF

```
HTF (H4/D1) → Bias: Bullish / Bearish / Neutral
              Faktor: Phase (accumulation/distribution), CHoCH arah, FVG unfilled
MTF (H1)    → Structure: Apakah retracing ke discount/premium?
              Entry setup: OB/FVG yang belum dimitigasi di arah HTF
LTF (M15)   → Entry Confirmation: CHoCH konfirmasi, killzone aktif

Confluence Score = HTF(2pt) + MTF(1pt) + LTF(1pt) alignment
Score 4/4 = Strong setup
Score 3/4 = Moderate
Score 2/4 = Weak / skip
```

## Command Extension

```
/ict EURUSD MTF    → jalankan 3 TF, tampilkan confluence dashboard
/ict EURUSD H4     → single TF (existing behavior, tidak berubah)
```

## File Changes

- `internal/service/ict/mtf.go` — NEW: MTFEngine, confluenceScore(), MTFResult struct
- `internal/service/ict/types.go` — Add MTFResult, ConfluenceScore types
- `internal/adapter/telegram/handler_ict.go` — Parse "MTF" keyword, call MTFEngine
- `internal/adapter/telegram/formatter_ict.go` — MTF dashboard formatter

## MTF Dashboard Format

```
📊 MTF ICT Confluence — EURUSD
━━━━━━━━━━━━━━━━━━━━
H4  [BULLISH BIAS] CHoCH ↑ | FVG 1.0820-1.0835 unfilled
H1  [DISCOUNT ZONE] Price at 38% retrace | OB 1.0815
M15 [CONFIRMED] CHoCH bullish | Killzone: London open

⚡ Confluence: 4/4 — STRONG LONG SETUP
🎯 Entry Zone: 1.0815-1.0820 (H1 OB + H4 FVG overlap)
🛑 Invalidation: Below 1.0800 (HTF structure low)
```

## Acceptance Criteria

- [ ] Fetch H4, H1, M15 bars for same symbol in parallel (goroutines)
- [ ] HTF bias derived from CHoCH direction + FVG fill status
- [ ] MTF checks if price is in premium/discount relative to HTF range
- [ ] LTF confirms entry (CHoCH + killzone alignment)
- [ ] Confluence score 0-4 with label (STRONG / MODERATE / WEAK / NO SETUP)
- [ ] Entry zone = overlap of HTF FVG + MTF OB (if exists)
- [ ] go build ./... clean
