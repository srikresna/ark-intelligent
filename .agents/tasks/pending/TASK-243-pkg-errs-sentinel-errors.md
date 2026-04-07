# TASK-243: TECH-007 — Buat pkg/errs: Sentinel Errors + Wrap Helper

**Priority:** medium
**Type:** refactor
**Estimated:** S
**Area:** pkg/errs/ (new package)
**Created by:** Research Agent
**Created at:** 2026-04-02 22:00 WIB

## Deskripsi

`pkg/errs` package belum ada. Saat ini error handling di codebase sangat tidak konsisten — mix antara bare `return nil, err`, `fmt.Errorf("wrap: %w", err)`, dan zerolog tanpa wrapping.

Task ini membuat **foundation** untuk standardized error handling (TECH-007 Phase 1):
1. Sentinel errors yang bisa di-check dengan `errors.Is()`
2. `Wrap()` helper yang add context tanpa losing the chain

**CATATAN:** Task ini hanya membuat package baru — TIDAK mengubah existing code. Migration ke pkg/errs di existing files adalah task terpisah di masa depan.

## Implementasi

### pkg/errs/errs.go

```go
// Package errs provides sentinel errors and wrapping utilities
// for consistent error handling across ark-intelligent services.
package errs

import "fmt"

// ---------------------------------------------------------------------------
// Sentinel Errors
// ---------------------------------------------------------------------------

// ErrNoData indicates that no data is available for the requested resource.
// Use errors.Is(err, errs.ErrNoData) to check.
var ErrNoData = fmt.Errorf("no data available")

// ErrRateLimited indicates the upstream API has rate-limited this client.
var ErrRateLimited = fmt.Errorf("rate limited by upstream")

// ErrNotFound indicates the requested resource does not exist.
var ErrNotFound = fmt.Errorf("not found")

// ErrStale indicates cached data is too old to be reliable.
var ErrStale = fmt.Errorf("data is stale")

// ErrUnavailable indicates the upstream service is temporarily unavailable.
var ErrUnavailable = fmt.Errorf("service unavailable")

// ---------------------------------------------------------------------------
// Wrap Helpers
// ---------------------------------------------------------------------------

// Wrap adds context to an error while preserving the error chain.
// Returns nil if err is nil.
// Usage: return errs.Wrap(err, "cot fetch")
func Wrap(err error, context string) error {
    if err == nil {
        return nil
    }
    return fmt.Errorf("%s: %w", context, err)
}

// Wrapf adds context with formatting to an error.
// Returns nil if err is nil.
// Usage: return errs.Wrapf(err, "cot fetch %s", contractCode)
func Wrapf(err error, format string, args ...any) error {
    if err == nil {
        return nil
    }
    return fmt.Errorf(format+": %w", append(args, err)...)
}
```

### pkg/errs/errs_test.go

```go
package errs_test

import (
    "errors"
    "testing"
    "github.com/arkcode369/ark-intelligent/pkg/errs"
)

func TestWrapNil(t *testing.T) {
    if errs.Wrap(nil, "context") != nil {
        t.Error("Wrap(nil) should return nil")
    }
}

func TestWrapPreservesChain(t *testing.T) {
    err := errs.Wrap(errs.ErrNoData, "cot fetch")
    if !errors.Is(err, errs.ErrNoData) {
        t.Error("Wrap should preserve error chain via errors.Is")
    }
}

func TestSentinelErrors(t *testing.T) {
    sentinels := []error{
        errs.ErrNoData,
        errs.ErrRateLimited,
        errs.ErrNotFound,
        errs.ErrStale,
        errs.ErrUnavailable,
    }
    for _, e := range sentinels {
        if e == nil {
            t.Errorf("sentinel error must not be nil")
        }
    }
}
```

## File yang Harus Dibuat

- `pkg/errs/errs.go`
- `pkg/errs/errs_test.go`

## Aturan

- **Hanya buat package baru** — jangan ubah existing files
- Package harus zero external dependencies (hanya stdlib)
- Test harus pass: `go test ./pkg/errs/...`
- `go vet ./pkg/errs/...` harus bersih

## Acceptance Criteria

- [ ] `pkg/errs/errs.go` dibuat dengan 5 sentinel errors + `Wrap()` + `Wrapf()`
- [ ] `pkg/errs/errs_test.go` dibuat dengan minimum 3 test cases
- [ ] `go test ./pkg/errs/...` → PASS
- [ ] `go build ./...` sukses (package baru, tidak ada breaking change)
- [ ] `errors.Is(errs.Wrap(errs.ErrNoData, "ctx"), errs.ErrNoData)` return true

## Referensi

- `.agents/research/2026-04-02-22-tech-refactor-plan-putaran10.md` — Temuan 3
- `TECH_REFACTOR_PLAN.md#TECH-007` — Error Handling Policy
- `pkg/fmtutil/format.go` — contoh struktur pkg package yang sudah ada
