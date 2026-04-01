# TASK-208: SKEW/VIX Ratio — Tail Risk Alert System

**Priority:** high
**Type:** feature
**Estimated:** S
**Area:** internal/service/vix/

## Deskripsi

Build specific alert system for SKEW/VIX ratio. High SKEW + low VIX = "complacent market with hidden tail risk" — one of the strongest crash warning signals. Requires TASK-205 for SKEW data.

## Signal Logic

```
SKEW > 140 AND VIX < 15 → EXTREME TAIL RISK WARNING
SKEW > 130 AND VIX < 18 → ELEVATED TAIL RISK
SKEW/VIX ratio > 8.0    → HISTORICALLY DANGEROUS
```

Historical: SKEW/VIX ratio >8 preceded Feb 2018 volmageddon, March 2020 crash, Aug 2024 yen carry unwind.

## File Changes

- `internal/service/vix/analysis.go` — Add SKEW/VIX ratio computation + alert thresholds
- `internal/service/fred/alerts.go` — Add tail risk alert to regime alert system
- `internal/adapter/telegram/formatter.go` — Add tail risk section to /vix output

## Acceptance Criteria

- [ ] SKEW/VIX ratio computed from TASK-205 data
- [ ] 3-tier alert: normal / elevated / extreme
- [ ] Historical percentile for current ratio
- [ ] Alert broadcast when ratio crosses extreme threshold
- [ ] Display in /vix output with historical context
- [ ] Depends on TASK-205 completion
