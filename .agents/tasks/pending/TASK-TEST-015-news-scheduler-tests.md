# TASK-TEST-015: Unit Tests for news/scheduler.go

**Task ID:** TASK-TEST-015  
**Type:** Test Coverage  
**Priority:** High  
**Effort Estimate:** 6-8 hours  
**Created:** 2026-04-05  
**Author:** Research Agent

---

## Objective

Create comprehensive unit tests for `internal/service/news/scheduler.go` — the news alert scheduling and dispatching system.

---

## Background

The news scheduler is **critical infrastructure** that manages:
- Background pulling of economic calendar data
- Pre-event reminder alerts (5min, 15min, 60min)
- Post-event impact analysis scheduling
- FRED data alert dispatching
- Cross-reference with COT data for confluence alerts
- Alert deduplication and rate limiting
- Tier-based alert filtering (Free vs Paid users)

**Current State:** 1,134 lines, **0 unit tests**, only 1 integration test file exists for the entire news package.

---

## Acceptance Criteria

### Coverage Targets
- [ ] Minimum 60% code coverage (aim for 70%+)
- [ ] All exported methods have test cases
- [ ] Alert scheduling logic fully tested
- [ ] Error handling paths tested

### Specific Test Cases Required

#### 1. Scheduler Initialization
- [ ] Test `NewScheduler()` with valid dependencies
- [ ] Test `NewScheduler()` with nil dependencies
- [ ] Test scheduler configuration loading

#### 2. Alert Scheduling Core
- [ ] Test `ScheduleAlert()` for economic events
- [ ] Test `ScheduleFREDAlert()` for FRED data releases
- [ ] Test reminder scheduling (5min, 15min, 60min before events)
- [ ] Test alert deduplication logic (`sentReminders` map)
- [ ] Test midnight reset of deduplication state

#### 3. Alert Dispatching
- [ ] Test `DispatchAlert()` with valid event data
- [ ] Test `DispatchAlert()` with nil messenger (graceful skip)
- [ ] Test alert filtering by currency (e.g., USD only)
- [ ] Test alert filtering by impact (High/Medium/Low)
- [ ] Test tier-based filtering (Free tier exclusions)

#### 4. Background Processing
- [ ] Test `Start()` begins background polling
- [ ] Test `Stop()` gracefully stops polling
- [ ] Test context cancellation propagation
- [ ] Test ticker-based polling intervals

#### 5. COT Cross-Reference (Confluence)
- [ ] Test COT data integration for confluence alerts
- [ ] Test `ConfluenceScore()` calculation
- [ ] Test confluence alert gating logic

#### 6. Concurrency Safety
- [ ] Test concurrent `ScheduleAlert()` calls
- [ ] Test concurrent access to `sentReminders` map
- [ ] Test thread-safe alert deduplication
- [ ] Test `sync.Mutex` usage for state protection

#### 7. Edge Cases
- [ ] Test scheduling with past events (should skip)
- [ ] Test with empty event list
- [ ] Test with missing/invalid COT data
- [ ] Test alert when user has no preferences set
- [ ] Test recovery from partial failures

---

## Technical Notes

### Dependencies to Mock

```go
type Scheduler struct {
    repo       ports.NewsRepository
    fetcher    ports.NewsFetcher
    aiAnalyzer ports.AIAnalyzer
    messenger  ports.Messenger
    prefsRepo  ports.PrefsRepository
    cotRepo    ports.COTRepository
    // ... callbacks
}
```

### Key Challenges
1. Time-based testing — use `github.com/jonboulle/clockwork` or manual ticker injection
2. Testing background goroutines — ensure clean start/stop
3. Mocking external service dependencies (news fetcher, AI analyzer)
4. Testing concurrent access to shared state (`sentReminders`)

### Critical Code Patterns to Test

```go
// From scheduler.go - these patterns need coverage:

// sentReminders map access (lines 50-58)
sentMu.Lock()
delete(s.sentReminders, key) // midnight cleanup
sentMu.Unlock()

// Background context usage (lines 715, 721)
recordCtx, recordCancel := context.WithTimeout(context.Background(), 5*time.Minute)

// Alert deduplication
if _, exists := s.sentReminders[reminderKey]; exists {
    return // skip duplicate
}
```

### Suggested Test Structure

```go
func TestScheduler_New(t *testing.T) { }
func TestScheduler_StartStop(t *testing.T) { }
func TestScheduler_ScheduleAlert(t *testing.T) { }
func TestScheduler_DispatchAlert(t *testing.T) { }
func TestScheduler_ReminderDeduplication(t *testing.T) { }
func TestScheduler_ConfluenceScoring(t *testing.T) { }
func TestScheduler_Concurrency(t *testing.T) { }
```

---

## Implementation Guidelines

1. **Create** `internal/service/news/scheduler_test.go`
2. **Use** `testify/mock` or manual mocks for dependency interfaces
3. **Use** `testify/assert` for assertions
4. **Follow** existing test patterns from `pkg/retry/retry_test.go`
5. **Ensure** tests run quickly (< 5s total)
6. **Avoid** actual network calls or external dependencies
7. **Use** `t.Parallel()` where safe
8. **Test** both success and error paths

---

## Implementation

**Branch:** `feat/TASK-TEST-015-news-scheduler-tests`  
**PR:** #363 - https://github.com/arkcode369/ark-intelligent/pull/363  
**Status:** In Review (pending QA approval)

---

## Definition of Done

- [x] Test file created with comprehensive coverage
- [x] All tests passing (`go test ./internal/service/news/...`)
- [ ] Coverage report shows 60%+ for news package (currently 14.4%, further tests needed)
- [x] No race conditions detected (`go test -race`)
- [ ] Code review approved by QA Agent
- [ ] Merged to main branch

---

## Related Files

- `internal/service/news/scheduler.go` (1,134 lines) — primary target
- `internal/service/news/fetcher.go` (373 lines)
- `internal/service/news/analyzer.go`
- `internal/service/news/impact_recorder.go`
- `internal/service/news/surprise.go`
- `internal/service/cot/` — for confluence integration

---

## Context

This file is critical for the bot's core functionality — economic calendar alerts. It:
- Runs background polling to detect new events
- Schedules pre-event reminders for subscribed users
- Cross-references with COT positioning for smart alerts
- Handles tier-based filtering (Free users get fewer alerts)

**Related Issues:**
- Shares context.Background() concerns with TASK-CODEQUALITY-002 (lines 715, 721)
- Uses sync.Mutex for `sentReminders` protection — verify race-free

---

## References

- Previous audit: `.agents/research/2026-04-05-scheduled-audit-17.md`
- Related task: TASK-CODEQUALITY-002 (context.Background() in production)
- Test examples: `pkg/retry/retry_test.go`, `pkg/mathutil/stats_test.go`
