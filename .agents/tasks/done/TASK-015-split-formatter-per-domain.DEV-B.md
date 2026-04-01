# TASK-015: Split formatter.go per Domain

**Status:** pending
**Priority:** HIGH
**Effort:** L (Large — estimasi 2-3 jam)
**Cycle:** Siklus 4 — Technical Refactor
**Ref:** TECH-001 in TECH_REFACTOR_PLAN.md

---

## Problem

`internal/adapter/telegram/formatter.go` saat ini **4,489 LOC** — god class yang menangani formatting untuk COT, Calendar, FRED, Price, Backtest, Sentiment, AI, Signal, dan domain lainnya dalam satu file.

**Dampak:**
- Setiap PR yang tambah feature baru ke formatter → conflict
- File impossible to navigate, sulit onboard dev baru
- Build/review time lebih lama karena file besar

---

## Solution

Split menjadi per-domain files di direktori baru:

```
internal/adapter/telegram/format/
├── cot.go          ← FormatCOT*, FormatBias*, FormatRank*
├── calendar.go     ← FormatCalendar*, FormatEvent*
├── macro.go        ← FormatMacro*, FormatFRED*, FormatRegime*
├── price.go        ← FormatPrice*, FormatLevel*, FormatSeasonal*
├── backtest.go     ← FormatBacktest*, FormatAccuracy*
├── sentiment.go    ← FormatSentiment*, FormatFlow*
├── ai.go           ← FormatOutlook*, FormatChat*
├── signal.go       ← FormatSignal*, FormatAlert*
└── common.go       ← shared helpers (directionArrow, numberFormat, dll)
```

---

## Acceptance Criteria

- [ ] `go build ./...` clean setelah refactor
- [ ] `go vet ./...` zero warnings
- [ ] Semua existing tests masih pass: `go test ./...`
- [ ] TIDAK ada behavior change — output Telegram identik
- [ ] Tidak ada file baru >800 LOC
- [ ] Import paths di semua caller (handler.go, handler_cta.go, dll) sudah diupdate

---

## Implementation Notes

1. Buat direktori `internal/adapter/telegram/format/`
2. Mulai dari domain terkecil dulu (sentimen/signal ~300 LOC) untuk warmup
3. Extract `common.go` paling akhir — butuh lihat apa yang truly shared
4. Update semua import di `handler.go`, `handler_cta.go`, `bot.go`, `scheduler.go`
5. **Jangan merge kalau ada test yang fail**

---

## Warning

Ini adalah task yang **BESAR** dan berisiko conflict tinggi dengan task lain.
Dev agent yang mengerjakan ini HARUS mengklaim task dulu (pindah ke `claimed/`) sebelum mulai.
Branch name: `refactor/split-formatter`

---

## Assigned To

(unassigned — tersedia untuk Dev-A, Dev-B, atau Dev-C)
