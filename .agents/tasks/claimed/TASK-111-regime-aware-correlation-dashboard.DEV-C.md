# TASK-111: Regime-Aware Correlation Dashboard V2

**Priority:** medium
**Type:** feature
**Estimated:** M
**Area:** internal/service/price
**Created by:** Research Agent
**Created at:** 2026-04-01 22:00 WIB
**Siklus:** Fitur

## Deskripsi
Extend correlation matrix engine agar di-split per regime. Tampilkan "correlation saat trending" vs "correlation saat crisis" vs "correlation saat ranging". Tag setiap historical bar dengan regime state dari HMM + ADX, lalu recalculate correlation matrix per bucket.

## Konteks
- Correlation matrix sudah ada di `service/price/correlation.go` (9KB) — Pearson + breakdown detection + clustering
- HMM regime sudah ada di `service/price/hmm_regime.go`
- ADX trend classification sudah ada di `service/ta/`
- Gap: correlasi statis, tidak membedakan "correlasi normal" vs "correlasi saat krisis"
- Institutional desks selalu melihat regime-conditional correlation
- Ref: `.agents/research/2026-04-01-22-fitur-regime-overlay-unified-signal.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Extend `correlation.go` atau buat file baru `regime_correlation.go`
- [ ] Type `RegimeCorrelationMatrix` dengan: CurrentRegime, MatrixPerRegime (map), BreakdownsByRegime
- [ ] Tag historical bars dengan regime state (HMM + ADX composite)
- [ ] Calculate separate correlation matrices per regime bucket (min 3: trending, ranging, crisis)
- [ ] Compare current correlation vs regime-baseline → flag divergences
- [ ] Expose via `/quant correlation` command dengan regime toggle keyboard
- [ ] Formatter output: tabel correlation per regime, highlight significant deviations

## File yang Kemungkinan Diubah
- `internal/service/price/correlation.go` (extend) atau `regime_correlation.go` (baru)
- `internal/domain/correlation.go` (extend types)
- `internal/adapter/telegram/formatter.go` (format regime correlation)
- `internal/adapter/telegram/handler_quant.go` (add regime toggle)

## Referensi
- `.agents/research/2026-04-01-22-fitur-regime-overlay-unified-signal.md`
- `internal/service/price/correlation.go`
- `internal/service/price/hmm_regime.go`
