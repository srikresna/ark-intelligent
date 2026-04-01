# TASK-170: Correlation pearsonCorrelation() Minimum Data Points Guard

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/service/price/

## Deskripsi

Fix pearsonCorrelation() yang loloskan n=2 data points (check `n < 3` seharusnya `n <= 2` atau naikkan minimum ke 5 untuk statistical validity). Dengan 2 points, correlation undefined.

## Bug Detail

```go
// correlation.go:322-326
n := len(x)
if len(y) < n { n = len(y) }
if n < 3 { return 0 }  // BUG: n=2 could slip through in edge cases
```

Seharusnya: `if n < 5 { return 0 }` — 5 minimum untuk Pearson correlation yang bermakna.

## File Changes

- `internal/service/price/correlation.go` — Change minimum n check dari 3 ke 5, return math.NaN() bukan 0 untuk insufficient data
- `internal/service/price/correlation.go` — Add NaN check di callers of pearsonCorrelation()

## Acceptance Criteria

- [ ] pearsonCorrelation() returns NaN untuk n < 5
- [ ] All callers handle NaN (isNaN check before using value)
- [ ] Correlation matrix cells marked "N/A" instead of 0.00 when insufficient
- [ ] Unit test: n=2, n=4, n=5, n=100 test cases
- [ ] No behavior change for n >= 5
