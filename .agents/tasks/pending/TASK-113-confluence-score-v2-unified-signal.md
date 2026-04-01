# TASK-113: Confluence Score V2 — Unified Directional Signal

**Priority:** high
**Type:** feature
**Estimated:** L
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-01 22:00 WIB
**Siklus:** Fitur

## Deskripsi
Buat unified directional signal per currency yang menjawab "should I be long or short EUR?" dengan satu score. Aggregate COT signal (30%) + CTA technical (30%) + Quant regime (20%) + Sentiment (15%) + Seasonal (5%) menjadi satu output.

Key innovation: conflict detection. Jika COT bilang long tapi CTA bilang short → confidence reduced. VotingMatrix menampilkan sub-system mana yang agree/disagree.

## Konteks
- Confluence v1 sudah ada di `service/ta/confluence.go` (12KB) — hanya technical indicators, score -100 to +100
- COT ConvictionScoreV3 sudah ada di `service/cot/`
- Quant models (HMM, GARCH, Hurst) sudah ada di `service/price/`
- Sentiment (VIX) sudah ada di `domain/risk.go`
- Seasonal sudah ada di `service/price/seasonal.go`
- Gap: semua terpisah, belum ada fusion engine
- Ref: `.agents/research/2026-04-01-22-fitur-regime-overlay-unified-signal.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat `internal/service/analysis/unified_signal_engine.go`
- [ ] Type `UnifiedSignalV2`: Currency, UnifiedScore (-100 to +100), Grade (A+ to F), component breakdown (COT, CTA, Quant, Sentiment, Seasonal), Confidence (0-100), ConflictCount, VotingMatrix, Recommendation (STRONG_LONG / LONG / NEUTRAL / SHORT / STRONG_SHORT)
- [ ] Normalize setiap signal ke -1 to +1 sebelum weighting
- [ ] Conflict detection: jika >2 sub-systems disagree → reduce confidence by 20% per conflict
- [ ] VIX multiplier: high VIX → reduce all scores magnitude by 15%
- [ ] Telegram command: `/signal [CURRENCY]` atau integrate ke `/bias`
- [ ] Format: clean output showing unified recommendation + breakdown per component
- [ ] Caching: hasil valid per timeframe (e.g., daily signal cached 4h)

## File yang Kemungkinan Diubah
- `internal/service/analysis/unified_signal_engine.go` (baru)
- `internal/service/analysis/types.go` (baru)
- `internal/adapter/telegram/handler.go` (new command routing)
- `internal/adapter/telegram/formatter.go` (format unified signal)
- `internal/adapter/telegram/keyboard.go` (currency selector)

## Referensi
- `.agents/research/2026-04-01-22-fitur-regime-overlay-unified-signal.md`
- `internal/service/ta/confluence.go`
- `internal/service/cot/`
- `internal/service/price/hmm_regime.go`
