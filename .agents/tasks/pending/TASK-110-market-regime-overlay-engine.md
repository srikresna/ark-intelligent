# TASK-110: Market Regime Overlay Engine

**Priority:** high
**Type:** feature
**Estimated:** L
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-01 22:00 WIB
**Siklus:** Fitur

## Deskripsi
Buat unified regime engine yang menggabungkan HMM state (3 states), GARCH volatility regime, ADX trend strength, dan COT sentiment menjadi SATU "market health score" (-100 to +100). Score ini di-overlay sebagai header di semua analysis output.

## Konteks
- HMM sudah ada di `service/price/hmm_regime.go` (RISK_ON/RISK_OFF/CRISIS)
- GARCH sudah ada di `service/price/garch.go` (EXPANDING/NORMAL/CONTRACTING)
- ADX sudah ada di `service/ta/indicators.go`
- COT sentiment sudah ada di `service/cot/`
- Semua terpisah — belum ada orkestrasi terpadu
- Ref: `.agents/research/2026-04-01-22-fitur-regime-overlay-unified-signal.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat `internal/service/regime/overlay_engine.go`
- [ ] Type `RegimeOverlay` dengan fields: HMMState, HMMConfidence, GARCHVolRegime, VolRatio, ADXStrength, COTSentiment, UnifiedScore (-100 to +100), OverlayColor (🟢/🟡/🔴), Description
- [ ] Weights: HMM 30%, GARCH 25%, ADX 25%, COT 20%
- [ ] Method `ComputeOverlay(ctx, symbol, timeframe)` yang orchestrate semua sub-models
- [ ] Expose sebagai header di output `/cta`, `/quant`, `/cot` (e.g., "📊 Regime: 🟢 BULLISH (+67) | Trending, Low Vol, COT Long")
- [ ] Cache overlay results (TTL sesuai timeframe: 1h untuk intraday, 4h untuk daily)
- [ ] Graceful degradation: jika satu sub-model gagal, tetap hitung dengan yang lain (adjust weights)

## File yang Kemungkinan Diubah
- `internal/service/regime/overlay_engine.go` (baru)
- `internal/service/regime/types.go` (baru)
- `internal/adapter/telegram/formatter.go` (tambah overlay header)
- `internal/adapter/telegram/handler_cta.go` (inject overlay)
- `internal/adapter/telegram/handler_quant.go` (inject overlay)

## Referensi
- `.agents/research/2026-04-01-22-fitur-regime-overlay-unified-signal.md`
- `internal/service/price/hmm_regime.go`
- `internal/service/price/garch.go`
- `internal/service/ta/indicators.go` (ADX)
