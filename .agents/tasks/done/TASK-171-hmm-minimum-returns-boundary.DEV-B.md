# TASK-171: HMM Regime Minimum Returns Boundary Fix

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/service/price/

## Deskripsi

Naikkan minimum returns threshold di HMM dari 40 ke 60. Dengan 40 returns dan 18 parameters (3 states × 5 emissions + transition matrix), model severely underparameterized. Rule of thumb: 5-10x parameters → minimum 90-180 observations.

## Bug Detail

```go
// hmm_regime.go:55
if len(prices) < 60 { return nil, err }
// hmm_regime.go:68
if len(returns) < 40 { return nil, err }  // BUG: too low
```

## File Changes

- `internal/service/price/hmm_regime.go` — Change `len(returns) < 40` ke `len(returns) < 60`
- `internal/service/price/hmm_regime.go` — Add convergence check: if Baum-Welch doesn't converge in 100 iterations, return error instead of using last estimate

## Acceptance Criteria

- [ ] Minimum returns raised to 60
- [ ] Baum-Welch convergence check (log-likelihood delta < 1e-6)
- [ ] Max iterations capped at 100 with non-convergence error
- [ ] /quant gracefully shows "insufficient data for regime" instead of unreliable result
- [ ] Unit test: exactly 60 returns, 59 returns (rejected)
