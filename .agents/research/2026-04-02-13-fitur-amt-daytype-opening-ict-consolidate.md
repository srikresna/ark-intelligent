# Research Report: Fitur Baru Siklus 3 Putaran 5
# AMT Implementation, ICT Consolidation, TA Engine Integration
**Date:** 2026-04-02 13:00 WIB
**Siklus:** 3/5 (Fitur Baru) — Putaran 5
**Author:** Research Agent

## Ringkasan

Deep analysis menemukan gap besar: AMT (Auction Market Theory) 100% documented tapi 0% code. ICT punya duplicate implementation. Wyckoff + SMC tidak terintegrasi ke ta/engine.go. 5 tasks fokus mengatasi gap ini.

## Temuan 1: AMT Day Type Classification — 0% Code

docs/AMT_UPGRADE_PLAN.md mendokumentasikan 5 modul AMT secara detail. Module 1 (Day Type Classification) adalah fondasi yang harus diimplementasi duluan.

**Dalton's 6 Day Types:**
- Normal: 85% range dalam Initial Balance (IB)
- Normal Variation: 100-115% IB extension
- Trend: >115% IB, one-directional
- Double Distribution: Two separate value areas
- P-shape: High volume di atas (selling into rally)
- b-shape: High volume di bawah (buying the dip)

**Data requirement:** 30m bars × 52 days (sudah tersedia di price service)
**No new data sources needed.**

## Temuan 2: AMT Opening Type Analysis — 0% Code

Module 2 sama pentingnya — Opening Type memberi edge di awal sesi:
- Open Drive (OD): Strong directional open, follow momentum
- Open Test Drive (OTD): Test back then drive, follow after test
- Open Rejection Reverse (ORR): Failed breakout, fade
- Open Auction (OA): Balance, wait for breakout

Requires yesterday's Value Area (VA) from existing Volume Profile engine (scripts/vp_engine.py sudah compute VA).

## Temuan 3: Wyckoff + SMC Not in ta/engine.go FullResult

**ta/engine.go line 17:** `ICT *ICTResult` exists, tapi MISSING:
- `SMC *SMCResult` — CalcSMC() exists tapi tidak dipanggil
- `Wyckoff *WyckoffResult` — Engine.Analyze() exists tapi tidak di FullResult

**Impact:** Centralized analysis pipeline (`ComputeSnapshot()`) tidak include Wyckoff/SMC data. Commands tetap bisa access via separate handler, tapi unified /report dan /alpha tidak bisa leverage data ini.

**Fix:** 2-line addition ke FullResult struct + 2 compute calls. Quick win.

## Temuan 4: Duplicate ICT Implementations

Dua implementasi ICT yang overlap:
1. `internal/service/ict/` — 6 files, own engine, own types (FVGZone, OrderBlock)
2. `internal/service/ta/ict.go` — 585 LOC, different types (FVG, OrderBlock)

Keduanya detect FVG, Order Blocks, Liquidity, Killzone. Type naming berbeda. Maintenance burden double.

**Recommendation:** Consolidate ke `ta/ict.go` (sudah integrated ke ta/engine.go). Make `service/ict/` wrapper atau remove.

## Temuan 5: AMT Modules 3-5 (Rotation, Close, Migration)

Sisa 3 AMT modules yang melengkapi:
- Module 3: Rotation Factor (VA crossing counter, balance score)
- Module 4: Close Location tracking + follow-through stats
- Module 5: Multi-Day Migration + MGI (Market Generated Information)

Combined ini memberikan institutional-grade Auction Market Theory analysis. Semua documented, semua feasible, semua dari existing data.
