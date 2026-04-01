# TASK-196: Chart Generation — Zero-Byte Output + Python Dependency Validation

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/telegram/, scripts/

## Deskripsi

Two fixes:
1. Add file size validation after Python subprocess: reject 0-byte chart files
2. Add startup check for Python dependencies (mplfinance, matplotlib, numpy, pandas)

## Fix 1: Zero-Byte Check

```go
pngData, err := os.ReadFile(chartPath)
if err != nil {
    return nil, fmt.Errorf("read chart: %w", err)
}
if len(pngData) == 0 {
    return nil, fmt.Errorf("chart renderer produced empty output")
}
```

## Fix 2: Startup Dependency Check

```go
// In bot initialization or health check
func checkPythonDeps() error {
    cmd := exec.Command("python3", "-c", "import mplfinance; import matplotlib; import numpy; import pandas")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("Python dependencies missing: %w", err)
    }
    return nil
}
```

## File Changes

- `internal/adapter/telegram/handler_cta.go` — Add size check after ReadFile
- `internal/adapter/telegram/handler_vp.go` — Same
- `internal/adapter/telegram/handler_quant.go` — Same
- `internal/health/health.go` — Add Python dependency check

## Acceptance Criteria

- [ ] 0-byte chart files rejected with clear error message
- [ ] User sees "Chart generation failed. Text analysis below:" fallback
- [ ] Python dependencies checked at startup
- [ ] Missing dependency logged as ERROR with package name
- [ ] Chart commands gracefully degrade to text-only
