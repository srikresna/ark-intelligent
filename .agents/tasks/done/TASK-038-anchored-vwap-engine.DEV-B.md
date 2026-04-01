# TASK-038: Anchored VWAP Engine — DONE by DEV-B
Completed: 2026-04-01
Branch: feat/TASK-038-anchored-vwap
Files:
- internal/service/ta/vwap.go (NEW — CalcVWAP, CalcVWAPAnchored, CalcVWAPSet, VWAPResult, VWAPSet)
- internal/service/ta/vwap_test.go (NEW — 6 tests)
- internal/service/ta/types.go (added VWAP field to IndicatorSnapshot)
- internal/service/ta/engine.go (integrated VWAP into ComputeSnapshot)
