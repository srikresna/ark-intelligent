# TASK-SECURITY-001: Fix http.DefaultClient timeout in tradingeconomics_client.go

**Priority:** High  
**Estimated Effort:** 1-2 hours  
**Status:** Completed ✅
**Completed By:** Dev-A
**Completed At:** 2026-04-06
**Branch:** feat/TASK-SECURITY-001-http-client-timeout
**PR:** https://github.com/arkcode369/ark-intelligent/pull/354 → agents/main

## Issue Description

The `tradingeconomics_client.go` file uses `http.DefaultClient.Do(req)` at line 246 without configuring a timeout. This can cause goroutine leaks if the HTTP request hangs indefinitely.

## Location

- File: `internal/service/macro/tradingeconomics_client.go`
- Line: 246
- Current code:
```go
resp, err := http.DefaultClient.Do(req)
```

## Expected Behavior

Use an HTTP client with explicit timeout configuration:
```go
client := &http.Client{
    Timeout: 30 * time.Second,
}
resp, err := client.Do(req)
```

## Acceptance Criteria (Dev MUST validate before PR)

- [ ] Replace `http.DefaultClient` with a custom `http.Client` with timeout
- [ ] Timeout should be reasonable (suggested: 30 seconds)
- [ ] Add unit test to verify timeout behavior (optional but recommended)
- [ ] Verify no other occurrences of `http.DefaultClient` in the codebase
- [ ] **VALIDATION: `go build ./...` passes**
- [ ] **VALIDATION: `go vet ./...` passes**
- [ ] **VALIDATION: No new test failures**

## Context

The Trading Economics client fetches macroeconomic data. Without a timeout, a slow or unresponsive API could cause goroutine accumulation and memory leaks in long-running bot processes.

## Related

- Pattern established in `internal/service/vix/fetcher.go` which correctly uses `&http.Client{Timeout: 15 * time.Second}`
