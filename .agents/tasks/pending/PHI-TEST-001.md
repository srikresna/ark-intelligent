# PHI-TEST-001: Add Unit Tests for Telegram Handlers

## Problem Statement
The telegram adapter has severe test coverage gap:
- 29 source files, only 1 test file (3% coverage)
- Critical 2,909-line `handler.go` has no tests
- 4,539-line `formatter.go` has no tests
- Business logic in handlers is untested, high regression risk

## Files Requiring Tests (Priority Order)

### P0 — Critical Business Logic
1. `internal/adapter/telegram/handler.go`
   - Command handlers (cmdCOT, cmdOutlook, cmdRank, etc.)
   - Error handling paths
   - Rate limiting integration

2. `internal/adapter/telegram/formatter.go`
   - Message formatting functions
   - HTML escaping (security)
   - Numeric parsing

3. `internal/adapter/telegram/api.go`
   - Telegram API calls
   - Retry logic
   - Rate limiting

### P1 — Supporting Components
4. `internal/adapter/telegram/bot.go`
   - Update handling loop
   - Command routing
   - Graceful shutdown

5. `internal/adapter/telegram/keyboard.go`
   - Callback parsing
   - Button generation

6. `internal/adapter/telegram/errors.go`
   - Error message mapping
   - User-friendly translations

## Acceptance Criteria
- [ ] Create `handler_test.go` with tests for top 5 commands
- [ ] Create `formatter_test.go` with HTML escape tests
- [ ] Create `api_test.go` with mock Telegram API server
- [ ] Achieve minimum 60% coverage for handler functions
- [ ] Add table-driven tests for error handling paths
- [ ] Mock external dependencies (repositories, services)
- [ ] Test both success and failure scenarios
- [ ] Run tests in CI pipeline

## Testing Strategy

### Mock Pattern
```go
type mockCOTRepo struct {
    signals []domain.Signal
    err     error
}

func (m *mockCOTRepo) GetLatest(ctx context.Context) ([]domain.Signal, error) {
    return m.signals, m.err
}
```

### Table-Driven Tests
```go
func TestCmdCOT(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        repoMock mockCOTRepo
        wantErr  bool
        wantMsg  string
    }{
        {
            name:     "success with data",
            input:    "/cot EUR",
            repoMock: mockCOTRepo{signals: testSignals},
            wantErr:  false,
            wantMsg:  "COT Positioning",
        },
        {
            name:     "repository error",
            input:    "/cot USD",
            repoMock: mockCOTRepo{err: errors.New("db error")},
            wantErr:  true,
        },
    }
    // ...
}
```

## Files to Create/Modify
- `internal/adapter/telegram/handler_test.go` (new)
- `internal/adapter/telegram/formatter_test.go` (new)
- `internal/adapter/telegram/api_test.go` (new)
- `internal/adapter/telegram/bot_test.go` (new)
- `internal/adapter/telegram/keyboard_test.go` (new)

## Risk Assessment
**Impact**: HIGH — Regression risk, untested business logic  
**Effort**: HIGH — 16-24 hours (recommend splitting)  
**Priority**: P1 (High)

## Suggested Split
This task is large. Suggest splitting into:
- PHI-TEST-001a: handler_test.go (core commands)
- PHI-TEST-001b: formatter_test.go + keyboard_test.go
- PHI-TEST-001c: api_test.go + bot_test.go

## Related
- MED-001 in research audit
