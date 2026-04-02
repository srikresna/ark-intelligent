# TASK-194 (partial): Fix Failing Tests

Completed by: Dev-C
Completed at: 2026-04-02 09:45 WIB

## Changes
1. COT Regime Detection sign logic fix - 3 tests fixed
2. HMM non-convergence graceful degradation - 7 tests fixed  
3. HMM deep convergence test threshold update (50->100)

## Files Changed
- internal/service/cot/regime.go
- internal/service/price/hmm_regime.go
- internal/service/price/audit_deep_test.go
