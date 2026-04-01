# TASK-095: FedWatch Unit Tests

- **Priority:** medium
- **Type:** test-coverage
- **Ref:** TECH-009 (Test Coverage)
- **Created by:** Dev-A
- **Claimed by:** Dev-A

## Description

Add comprehensive unit tests for the `internal/service/fed` package (FedWatch integration).
This was added in TASK-030 but shipped without any tests.

## Test Coverage

- `DominantOutcome()` — 7 tests (all outcomes, nil, unavailable, tie)
- `ImpliedCutsToYearEnd()` — 9 tests (normal, edge cases, nil, zero, fractional)
- `FetchFedWatch()` — 4 tests (no API key, cache hit, cache expired, concurrent access)
- `FedWatchData` — 1 test (JSON round-trip)
- Concurrent cache safety — 1 test (20 goroutines)

Total: 22 test functions.

## Acceptance Criteria

- [x] `go test ./internal/service/fed/ -count=1` passes
- [x] `go build ./...` clean
- [x] `go vet ./...` clean
