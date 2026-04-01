# TASK-187: Integrate Wyckoff + SMC into ta/engine.go FullResult

**Priority:** high
**Type:** refactor
**Estimated:** S
**Area:** internal/service/ta/

## Deskripsi

Quick win: add Wyckoff dan SMC results ke ta/engine.go FullResult struct dan compute di ComputeSnapshot(). Saat ini hanya ICT yang terintegrasi. Wyckoff dan SMC exist tapi tidak accessible via centralized pipeline.

## Current State

```go
// ta/engine.go line 11-18
type FullResult struct {
    // ...
    ICT *ICTResult  // ✅ exists
    // MISSING: SMC, Wyckoff
}
```

## Fix

```go
type FullResult struct {
    // ...
    ICT     *ICTResult
    SMC     *SMCResult           // ADD
    Wyckoff *wyckoff.WyckoffResult // ADD
}

func ComputeSnapshot(...) *FullResult {
    // ...
    result.ICT = CalcICT(bars, snap.ATR)
    result.SMC = CalcSMC(bars, snap.ATR)        // ADD
    result.Wyckoff = wyckoffEngine.Analyze(...)   // ADD
}
```

## File Changes

- `internal/service/ta/engine.go` — Add SMC + Wyckoff fields to FullResult, compute in ComputeSnapshot()
- `internal/service/ta/engine.go` — Import wyckoff package

## Acceptance Criteria

- [ ] FullResult includes SMC *SMCResult field
- [ ] FullResult includes Wyckoff *WyckoffResult field
- [ ] ComputeSnapshot computes both
- [ ] Existing /cta, /alpha, /report can access SMC + Wyckoff data
- [ ] No breaking changes — existing fields untouched
- [ ] go build ./... clean
