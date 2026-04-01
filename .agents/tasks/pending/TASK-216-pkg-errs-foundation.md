# TASK-216: pkg/errs — Error Package Foundation

**Priority:** MEDIUM
**Type:** Tech Refactor
**Estimated:** S
**Area:** pkg/errs/ (new package)
**Ref:** TECH-007 in TECH_REFACTOR_PLAN.md
**Created by:** Research Agent
**Created at:** 2026-04-02 08:00 WIB
**Siklus:** 4 — Technical Refactor

## Problem

718 pola error handling campuran di codebase:
- `log.Error().Err(err).Msg(...)` — zerolog style
- `fmt.Errorf("wrap: %w", err)` — stdlib
- `return nil, err` — bare return tanpa context

Tidak ada sentinel errors yang bisa di-`errors.Is()`. Sulit debug dari mana asalnya error.

## Approach

Buat `pkg/errs/` package minimal yang tidak breaking — tidak perlu langsung migrasi semua kode:

```go
package errs

import (
    "errors"
    "fmt"
)

// Sentinel errors — dapat di-errors.Is() oleh caller
var (
    ErrNoData       = errors.New("no data available")
    ErrRateLimited  = errors.New("rate limited")
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrTimeout      = errors.New("timeout")
    ErrInvalidInput = errors.New("invalid input")
)

// Wrap menambahkan context ke error tanpa kehilangan wrapped error.
// Usage: return errs.Wrap(err, "cot fetch")
func Wrap(err error, context string) error {
    if err == nil {
        return nil
    }
    return fmt.Errorf("%s: %w", context, err)
}

// Is adalah alias errors.Is untuk convenience.
func Is(err, target error) bool {
    return errors.Is(err, target)
}
```

Setelah package ada, dev bisa mulai adopt secara incremental.

## File Changes

- `pkg/errs/errs.go` — NEW: sentinel errors + Wrap
- `pkg/errs/errs_test.go` — NEW: unit tests (Wrap, sentinel matching)

## Acceptance Criteria

- [ ] `pkg/errs/errs.go` dengan 6 sentinel errors + Wrap + Is
- [ ] Unit tests cover: Wrap nil, Wrap non-nil, errors.Is sentinel, chain wrap
- [ ] `go build ./... && go test ./pkg/errs/...` clean
- [ ] Tidak mengubah kode existing — pure addition
- [ ] Branch: `refactor/pkg-errs-foundation`
