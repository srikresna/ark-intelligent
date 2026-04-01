# TASK-138: Proactive Regime Change Alert System

**Priority:** high
**Type:** feature
**Estimated:** M
**Area:** internal/service/price + internal/scheduler
**Created by:** Research Agent
**Created at:** 2026-04-02 03:00 WIB
**Siklus:** Fitur

## Deskripsi
Leverage existing HMM TransitionWarning: buat proactive alert system yang fire saat regime transition detected. Track regime duration, implement alert tiers (AMBER early warning, RED confirmed), multi-asset regime sync.

## Konteks
- `service/price/hmm_regime.go` — 3-state HMM, TransitionWarning sudah compute P(regime_change)
- `detectTransitionWarning()` check if next-state probability >30%
- Missing: proactive push alerts, regime duration tracking, multi-asset sync
- Ref: `.agents/research/2026-04-02-03-fitur-volcom-carry-microstructure-regime-alert.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Track regime state history: `RegimeStartDate`, `DaysInCurrentRegime`, `MeanRegimeDuration`
- [ ] Alert tiers:
  - 🟡 AMBER: P(CRISIS) > 20% — early warning
  - 🔴 RED: P(CRISIS) > 50% atau regime flip confirmed via Viterbi
- [ ] Scheduler job: check regime probabilities setiap 4 jam
- [ ] Push alert ke subscribed users via Telegram saat tier changes
- [ ] Telegram command: `/regime` showing current state, duration, probabilities, alert tier
- [ ] Multi-asset: check HMM for BTC, ETH jika data available — flag divergence
- [ ] Cool-down: jangan spam alerts — max 1 alert per regime transition event

## File yang Kemungkinan Diubah
- `internal/service/price/regime_alert.go` (baru)
- `internal/scheduler/` (new regime check job)
- `internal/adapter/telegram/handler.go` (new /regime command)
- `internal/adapter/telegram/formatter.go` (regime alert formatter)

## Referensi
- `.agents/research/2026-04-02-03-fitur-volcom-carry-microstructure-regime-alert.md`
- `internal/service/price/hmm_regime.go`
