# TASK-096: Fix Fibonacci Flat-Market Edge Case

**Priority:** MEDIUM
**Siklus:** 5 (Bug Hunting)
**Estimasi:** 1 jam

## Problem

`CalcFibonacci()` di `internal/service/ta/fibonacci.go` tidak handle kasus `swingHigh == swingLow` (market choppy/flat, atau synthetic data dengan semua OHLC sama):

```go
diff := swingHigh - swingLow
// Jika diff == 0:
// - Semua level identik (swingHigh - 0 = swingHigh)
// - nearestLevel tetap "" (semua dist = 0, tapi minDist tidak update)
// - Output FibResult misleading: levels ada tapi semua sama
```

`NearestLevel` juga berpotensi return empty string karena `minDist` tetap `MaxFloat64` ketika semua distances = 0.

## Solution

```go
diff := swingHigh - swingLow
// Early return jika swing range terlalu kecil (< 0.001% dari price)
minDiff := swingHigh * 0.00001
if diff < minDiff {
    return nil // tidak ada swing yang meaningful
}
```

## Acceptance Criteria
- [ ] `CalcFibonacci` return nil jika swingHigh == swingLow atau diff sangat kecil
- [ ] Unit test: input dengan semua bar OHLC identik → return nil
- [ ] Unit test: input choppy market dengan minimal swing → handled gracefully
- [ ] Existing tests tetap pass
- [ ] `go build ./...` clean

## Files to Modify
- `internal/service/ta/fibonacci.go`
- `internal/service/ta/ta_test.go` (add test cases)
