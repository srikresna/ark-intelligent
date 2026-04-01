package ai

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
)

// ---------------------------------------------------------------------------
// Mock AIAnalyzer
// ---------------------------------------------------------------------------

type mockAIAnalyzer struct {
	available   bool
	callCount   int
	mu          sync.Mutex
	returnVal   string
	returnErr   error
}

func (m *mockAIAnalyzer) AnalyzeCOT(_ context.Context, _ []domain.COTAnalysis) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.returnVal, m.returnErr
}

func (m *mockAIAnalyzer) AnalyzeCOTWithPrice(_ context.Context, _ []domain.COTAnalysis, _ map[string]*domain.PriceContext) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.returnVal, m.returnErr
}

func (m *mockAIAnalyzer) GenerateWeeklyOutlook(_ context.Context, _ ports.WeeklyData) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.returnVal, m.returnErr
}

func (m *mockAIAnalyzer) AnalyzeCrossMarket(_ context.Context, _ map[string]*domain.COTAnalysis) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.returnVal, m.returnErr
}

func (m *mockAIAnalyzer) AnalyzeNewsOutlook(_ context.Context, _ []domain.NewsEvent, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.returnVal, m.returnErr
}

func (m *mockAIAnalyzer) AnalyzeCombinedOutlook(_ context.Context, _ ports.WeeklyData) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.returnVal, m.returnErr
}

func (m *mockAIAnalyzer) AnalyzeFREDOutlook(_ context.Context, _ *fred.MacroData, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.returnVal, m.returnErr
}

func (m *mockAIAnalyzer) AnalyzeActualRelease(_ context.Context, _ domain.NewsEvent, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.returnVal, m.returnErr
}

func (m *mockAIAnalyzer) IsAvailable() bool {
	return m.available
}

func (m *mockAIAnalyzer) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// ---------------------------------------------------------------------------
// Mock AICacheRepository
// ---------------------------------------------------------------------------

type mockCacheRepo struct {
	mu    sync.Mutex
	store map[string]string
}

func newMockCacheRepo() *mockCacheRepo {
	return &mockCacheRepo{store: make(map[string]string)}
}

func (m *mockCacheRepo) Get(_ context.Context, key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.store[key]
	return v, ok
}

func (m *mockCacheRepo) Set(_ context.Context, key, response, cacheType, dataVersion string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = response
	return nil
}

func (m *mockCacheRepo) InvalidateByPrefix(_ context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.store {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(m.store, k)
		}
	}
	return nil
}

func (m *mockCacheRepo) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.store)
}

// ---------------------------------------------------------------------------
// Tests: CachedInterpreter
// ---------------------------------------------------------------------------

func TestCachedInterpreter_IsAvailable(t *testing.T) {
	t.Run("available", func(t *testing.T) {
		inner := &mockAIAnalyzer{available: true}
		ci := NewCachedInterpreter(inner, newMockCacheRepo())
		if !ci.IsAvailable() {
			t.Error("expected IsAvailable() = true")
		}
	})

	t.Run("not available", func(t *testing.T) {
		inner := &mockAIAnalyzer{available: false}
		ci := NewCachedInterpreter(inner, newMockCacheRepo())
		if ci.IsAvailable() {
			t.Error("expected IsAvailable() = false")
		}
	})
}

func TestCachedInterpreter_CacheMiss(t *testing.T) {
	inner := &mockAIAnalyzer{available: true, returnVal: "COT analysis result"}
	cache := newMockCacheRepo()
	ci := NewCachedInterpreter(inner, cache)

	analyses := []domain.COTAnalysis{
		{ReportDate: time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)},
	}

	ctx := context.Background()
	result, err := ci.AnalyzeCOT(ctx, analyses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "COT analysis result" {
		t.Errorf("result = %q, want %q", result, "COT analysis result")
	}
	if inner.getCallCount() != 1 {
		t.Errorf("inner call count = %d, want 1", inner.getCallCount())
	}
	// Verify it was stored in cache
	if cache.count() != 1 {
		t.Errorf("cache should have 1 entry, got %d", cache.count())
	}
}

func TestCachedInterpreter_CacheHit(t *testing.T) {
	inner := &mockAIAnalyzer{available: true, returnVal: "fresh result"}
	cache := newMockCacheRepo()
	ci := NewCachedInterpreter(inner, cache)

	// Pre-seed cache with expected key
	reportDate := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	version := reportDate.Format("20060102")
	key := fmt.Sprintf("aicache:cot:%s:np", version)
	cache.Set(context.Background(), key, "cached result", "cot", version)

	analyses := []domain.COTAnalysis{
		{ReportDate: reportDate},
	}

	ctx := context.Background()
	result, err := ci.AnalyzeCOT(ctx, analyses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "cached result" {
		t.Errorf("result = %q, want cached result", result)
	}
	// Inner should NOT have been called
	if inner.getCallCount() != 0 {
		t.Errorf("inner call count = %d, want 0 (cache hit)", inner.getCallCount())
	}
}

func TestCachedInterpreter_NilCache_Passthrough(t *testing.T) {
	inner := &mockAIAnalyzer{available: true, returnVal: "direct result"}
	ci := NewCachedInterpreter(inner, nil)

	analyses := []domain.COTAnalysis{
		{ReportDate: time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)},
	}

	ctx := context.Background()
	result, err := ci.AnalyzeCOT(ctx, analyses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "direct result" {
		t.Errorf("result = %q, want direct result", result)
	}
	if inner.getCallCount() != 1 {
		t.Errorf("inner call count = %d, want 1", inner.getCallCount())
	}
}

func TestCachedInterpreter_EmptyAnalyses_Passthrough(t *testing.T) {
	inner := &mockAIAnalyzer{available: true, returnVal: "empty result"}
	cache := newMockCacheRepo()
	ci := NewCachedInterpreter(inner, cache)

	ctx := context.Background()
	result, err := ci.AnalyzeCOT(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "empty result" {
		t.Errorf("result = %q, want empty result", result)
	}
}

func TestCachedInterpreter_AnalyzeActualRelease_NeverCached(t *testing.T) {
	inner := &mockAIAnalyzer{available: true, returnVal: "release analysis"}
	cache := newMockCacheRepo()
	ci := NewCachedInterpreter(inner, cache)

	ctx := context.Background()
	result, err := ci.AnalyzeActualRelease(ctx, domain.NewsEvent{}, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "release analysis" {
		t.Errorf("result = %q, want release analysis", result)
	}
	// Cache should remain empty — AnalyzeActualRelease is never cached
	if cache.count() != 0 {
		t.Errorf("cache should be empty for AnalyzeActualRelease, got %d entries", cache.count())
	}
}

func TestCachedInterpreter_InvalidateOnCOTUpdate(t *testing.T) {
	inner := &mockAIAnalyzer{available: true, returnVal: "result"}
	cache := newMockCacheRepo()
	ci := NewCachedInterpreter(inner, cache)

	ctx := context.Background()

	// Seed some cache entries
	cache.Set(ctx, "aicache:cot:20260325:np", "cot cached", "cot", "20260325")
	cache.Set(ctx, "aicache:weekly:20260324:20260325:id", "weekly cached", "weekly", "20260324")
	cache.Set(ctx, "aicache:cross:20260325", "cross cached", "cross", "20260325")
	cache.Set(ctx, "aicache:combined:20260324:id:nf", "combined cached", "combined", "20260324")
	cache.Set(ctx, "aicache:fred:20260325:id", "fred cached", "fred", "20260325")
	cache.Set(ctx, "aicache:news:20260324:id", "news cached", "news", "20260324")

	if cache.count() != 6 {
		t.Fatalf("expected 6 cache entries, got %d", cache.count())
	}

	ci.InvalidateOnCOTUpdate(ctx)

	// cot, weekly, cross, combined should be invalidated (4 entries)
	// fred and news should remain (2 entries)
	if cache.count() != 2 {
		t.Errorf("expected 2 remaining cache entries after COT invalidation, got %d", cache.count())
	}

	// Verify fred and news survived
	if _, ok := cache.Get(ctx, "aicache:fred:20260325:id"); !ok {
		t.Error("fred cache entry should survive COT invalidation")
	}
	if _, ok := cache.Get(ctx, "aicache:news:20260324:id"); !ok {
		t.Error("news cache entry should survive COT invalidation")
	}
}

func TestCachedInterpreter_InvalidateOnNewsUpdate(t *testing.T) {
	inner := &mockAIAnalyzer{available: true}
	cache := newMockCacheRepo()
	ci := NewCachedInterpreter(inner, cache)

	ctx := context.Background()

	cache.Set(ctx, "aicache:news:20260324:id", "news cached", "news", "20260324")
	cache.Set(ctx, "aicache:combined:20260324:id:nf", "combined cached", "combined", "20260324")
	cache.Set(ctx, "aicache:cot:20260325:np", "cot cached", "cot", "20260325")

	ci.InvalidateOnNewsUpdate(ctx)

	// news and combined should be gone, cot should remain
	if cache.count() != 1 {
		t.Errorf("expected 1 remaining cache entry after news invalidation, got %d", cache.count())
	}
	if _, ok := cache.Get(ctx, "aicache:cot:20260325:np"); !ok {
		t.Error("cot cache entry should survive news invalidation")
	}
}

func TestCachedInterpreter_InvalidateAll(t *testing.T) {
	inner := &mockAIAnalyzer{available: true}
	cache := newMockCacheRepo()
	ci := NewCachedInterpreter(inner, cache)

	ctx := context.Background()
	cache.Set(ctx, "aicache:cot:20260325:np", "cot", "cot", "20260325")
	cache.Set(ctx, "aicache:fred:20260325:id", "fred", "fred", "20260325")
	cache.Set(ctx, "aicache:news:20260324:id", "news", "news", "20260324")

	ci.InvalidateAll(ctx)

	if cache.count() != 0 {
		t.Errorf("expected 0 cache entries after InvalidateAll, got %d", cache.count())
	}
}

func TestCachedInterpreter_InvalidateOnFREDUpdate(t *testing.T) {
	inner := &mockAIAnalyzer{available: true}
	cache := newMockCacheRepo()
	ci := NewCachedInterpreter(inner, cache)

	ctx := context.Background()
	cache.Set(ctx, "aicache:fred:20260325:id", "fred", "fred", "20260325")
	cache.Set(ctx, "aicache:weekly:20260324:20260325:id", "weekly", "weekly", "20260324")
	cache.Set(ctx, "aicache:news:20260324:id", "news", "news", "20260324")
	cache.Set(ctx, "aicache:combined:20260324:id:wf", "combined", "combined", "20260324")
	cache.Set(ctx, "aicache:cot:20260325:np", "cot", "cot", "20260325")

	ci.InvalidateOnFREDUpdate(ctx)

	// fred, weekly, news, combined should be gone. Only cot remains.
	if cache.count() != 1 {
		t.Errorf("expected 1 remaining entry after FRED invalidation, got %d", cache.count())
	}
	if _, ok := cache.Get(ctx, "aicache:cot:20260325:np"); !ok {
		t.Error("cot cache entry should survive FRED invalidation")
	}
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func TestLatestReportDate(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		got := latestReportDate(nil)
		if got != "unknown" {
			t.Errorf("latestReportDate(nil) = %q, want unknown", got)
		}
	})

	t.Run("single entry", func(t *testing.T) {
		analyses := []domain.COTAnalysis{
			{ReportDate: time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)},
		}
		got := latestReportDate(analyses)
		if got != "20260325" {
			t.Errorf("got %q, want 20260325", got)
		}
	})

	t.Run("multiple entries", func(t *testing.T) {
		analyses := []domain.COTAnalysis{
			{ReportDate: time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)},
			{ReportDate: time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)},
			{ReportDate: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)},
		}
		got := latestReportDate(analyses)
		if got != "20260325" {
			t.Errorf("got %q, want 20260325", got)
		}
	})
}

func TestLatestReportDateFromMap(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		got := latestReportDateFromMap(nil)
		if got != "unknown" {
			t.Errorf("got %q, want unknown", got)
		}
	})

	t.Run("with entries", func(t *testing.T) {
		data := map[string]*domain.COTAnalysis{
			"EUR": {ReportDate: time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)},
			"GBP": {ReportDate: time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)},
			"JPY": nil,
		}
		got := latestReportDateFromMap(data)
		if got != "20260325" {
			t.Errorf("got %q, want 20260325", got)
		}
	})
}

func TestCurrentWeekStart(t *testing.T) {
	result := currentWeekStart()
	if len(result) != 8 {
		t.Errorf("currentWeekStart() = %q, expected YYYYMMDD format", result)
	}
	// Parse and verify it's a Monday
	parsed, err := time.Parse("20060102", result)
	if err != nil {
		t.Fatalf("failed to parse %q: %v", result, err)
	}
	if parsed.Weekday() != time.Monday {
		t.Errorf("currentWeekStart() = %s which is %s, want Monday", result, parsed.Weekday())
	}
}
