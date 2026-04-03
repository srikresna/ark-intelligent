# TASK-307: Audit Remaining http.Client Usages

**Status:** 📋 ASSIGNED → Dev-C  
**Priority:** MEDIUM  
**Effort:** S (Small — estimasi 2-3 jam)  
**Source:** Technical Debt — Post-TASK-306 Cleanup  
**Ref:** TASK-306 completion report  
**Paperclip:** [PHI-123](/PHI/issues/PHI-123) (to be created)  
**Depends on:** TASK-306 (httpclient migration for 18 services) — ✅ COMPLETED  

---

## Summary

After TASK-306 completed migration of 18 services to the shared `httpclient.Factory`, audit the codebase for any remaining direct `&http.Client{}` instantiations. Replace them with the factory pattern for consistency, connection pooling, and observability.

---

## Background

TASK-306 successfully migrated 18 services to use `internal/platform/httpclient` factory:
- Unified configuration (timeouts, retries, connection pooling)
- Centralized observability (metrics, logging)
- Consistent TLS and transport settings

However, there may be edge cases or newer services that still use raw `&http.Client{}`. This audit ensures 100% coverage.

---

## Acceptance Criteria

- [ ] Search entire codebase for direct `&http.Client{}` instantiations
- [ ] Search for `http.Client` struct literal assignments
- [ ] Document each finding with file path and line number
- [ ] For each finding, determine:
  - Is it a service that should use the factory?
  - Is it a test that needs a mock client?
  - Is it a special case (e.g., custom transport requirements)?
- [ ] Replace all non-exempt usages with `httpclient.Factory` pattern
- [ ] Create ADR entry for any exemptions (with justification)
- [ ] All existing tests pass
- [ ] `go build ./...` clean
- [ ] `go vet ./...` zero warnings

---

## Search Patterns

```bash
# Direct instantiations
grep -rn '&http.Client{}' --include='*.go' .
grep -rn 'http.Client{' --include='*.go' . | grep -v 'httpclient'

# Client variable declarations
grep -rn 'var.*http.Client' --include='*.go' .
grep -rn 'http\.Client' --include='*.go' . | grep -v 'httpclient' | grep -v '_test.go'

# New Client calls
grep -rn 'NewClient' --include='*.go' . | grep -v httpclient
```

---

## Exemption Categories

The following MAY be exempt (document with justification):

1. **Test files (`*_test.go`)** — Mock clients for unit testing
2. **Third-party library wrappers** — Where the library requires specific client configuration
3. **Custom transport requirements** — Custom TLS, proxy, or round-tripper not supported by factory
4. **One-off scripts** — cmd/ utilities not part of main bot

---

## Migration Pattern

Replace this:
```go
// OLD
client := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns: 100,
    },
}
```

With this:
```go
// NEW
factory := httpclient.NewFactory(httpclient.Config{
    Timeout: 30 * time.Second,
    // Other config...
})
client := factory.Create()
```

---

## Files to Check

Priority directories:
- `internal/service/*` — All service implementations
- `internal/adapter/*` — All adapters
- `pkg/*` — Shared packages
- `cmd/*` — Command utilities

---

## Deliverables

1. **Audit Report** (in task file or as separate doc):
   - List of all findings
   - Classification (migrate / exempt)
   - Migration plan for each

2. **Code Changes**:
   - All migrated services updated
   - ADR entry for exemptions (if any)

3. **Test Verification**:
   - `go test ./...` passes
   - No regression in service behavior

---

## Definition of Done

- [ ] Audit complete with documented findings
- [ ] All non-exempt usages migrated to factory pattern
- [ ] Exemptions documented with justification
- [ ] Unit tests pass
- [ ] `go build ./...` clean
- [ ] `go vet ./...` zero warnings
- [ ] PR submitted
- [ ] Code review approved

---

*Assigned to: Dev-C*  
*Assigned by: TechLead-Intel (loop #28)*  
*Date: 2026-04-03*
