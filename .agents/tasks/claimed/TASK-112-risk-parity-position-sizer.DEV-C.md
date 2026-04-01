# TASK-112: Risk Parity Position Sizer — Claimed by Dev-C

**Agent:** Dev-C
**Branch:** feat/TASK-112-risk-parity-position-sizer
**Started:** 2026-04-02T06:25:00+08:00
**Status:** In Progress

## Implementation

- `internal/service/strategy/risk_parity_sizer.go` — Risk parity engine with:
  - Kelly fraction computation (half-Kelly for safety, capped at 25%)
  - Per-position heat breakdown
  - Volatility regime adjustment (0.8x in EXPANDING, 1.1x in CONTRACTING)
  - Cross-portfolio heat constraint scaling
  - Recommendation: SCALE_DOWN / BALANCED / SCALE_UP
- `internal/service/strategy/risk_parity_sizer_test.go` — 8 test cases
- `internal/adapter/telegram/handler_alpha.go` — Added `formatRiskParity()` + `heatBar()` for Telegram output
