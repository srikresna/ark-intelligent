# TASK-096: Extract Magic Numbers to Constants (TECH-006)

**Priority:** medium
**Estimated:** S
**Area:** internal/config | internal/adapter | internal/service
**Created by:** Dev-A Agent
**Created at:** 2026-04-01 22:45 WIB

## Deskripsi
Extract remaining magic numbers and hardcoded values to named constants in `internal/config/constants.go`.

## Konteks
Refactor plan TECH-006. Magic numbers scattered across codebase make it hard to tune thresholds and understand intent.

## Targets
1. `internal/service/backtest/stats.go:152` — `Strength >= 4` → use `config.SignalStrengthAlert`
2. `internal/adapter/telegram/api.go` — `35*time.Millisecond` rate limit → new constant
3. `internal/adapter/telegram/api.go` — `4096` Telegram message limit → new constant
4. `internal/service/price/fetcher.go` — `300*time.Millisecond` rate limit → new constant
5. `internal/service/cot/fetcher.go` — `200*time.Millisecond` rate limit → new constant
6. `internal/service/ai/claude_analyzer.go` — `MaxTokens: 4096` → new constant
7. `internal/service/ai/gemini.go` — `SetMaxOutputTokens(4096)` → new constant
8. `internal/service/microstructure/engine.go` — `Strength >= 0.50` → new constant

## Acceptance Criteria
- [ ] All magic numbers replaced with named constants
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] No behavior change
