package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ============================================================================
// Mock Implementations
// ============================================================================

type mockMessenger struct {
	mu       sync.Mutex
	sent     []string
	chatIDs  []string
	kbSent   []ports.InlineKeyboard
	failNext bool
}

func (m *mockMessenger) SendMessage(ctx context.Context, chatID string, text string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return 0, errors.New("send failed")
	}
	m.sent = append(m.sent, text)
	m.chatIDs = append(m.chatIDs, chatID)
	return 1, nil
}

func (m *mockMessenger) SendHTML(ctx context.Context, chatID string, html string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return 0, errors.New("send failed")
	}
	m.sent = append(m.sent, html)
	m.chatIDs = append(m.chatIDs, chatID)
	return 1, nil
}

func (m *mockMessenger) SendWithKeyboard(ctx context.Context, chatID string, text string, kb ports.InlineKeyboard) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return 0, errors.New("send failed")
	}
	m.sent = append(m.sent, text)
	m.chatIDs = append(m.chatIDs, chatID)
	m.kbSent = append(m.kbSent, kb)
	return 1, nil
}

func (m *mockMessenger) EditMessage(ctx context.Context, chatID string, msgID int, text string) error {
	return nil
}

func (m *mockMessenger) EditWithKeyboard(ctx context.Context, chatID string, msgID int, text string, kb ports.InlineKeyboard) error {
	return nil
}

func (m *mockMessenger) AnswerCallback(ctx context.Context, callbackID string, text string) error {
	return nil
}

func (m *mockMessenger) DeleteMessage(ctx context.Context, chatID string, msgID int) error {
	return nil
}

type mockPrefsRepository struct {
	mu           sync.Mutex
	users        map[int64]domain.UserPrefs
	getAllErr    error
}

func (m *mockPrefsRepository) Get(ctx context.Context, userID int64) (domain.UserPrefs, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if prefs, ok := m.users[userID]; ok {
		return prefs, nil
	}
	return domain.DefaultPrefs(), nil
}

func (m *mockPrefsRepository) Set(ctx context.Context, userID int64, prefs domain.UserPrefs) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[userID] = prefs
	return nil
}

func (m *mockPrefsRepository) GetAllActive(ctx context.Context) (map[int64]domain.UserPrefs, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getAllErr != nil {
		return nil, m.getAllErr
	}
	result := make(map[int64]domain.UserPrefs)
	for id, prefs := range m.users {
		if prefs.COTAlertsEnabled && prefs.ChatID != "" {
			result[id] = prefs
		}
	}
	return result, nil
}



// ============================================================================
// Test Setup Helpers
// ============================================================================

func newTestScheduler(t *testing.T) (*Scheduler, *mockMessenger, *mockPrefsRepository) {
	mockBot := &mockMessenger{}
	mockPrefs := &mockPrefsRepository{
		users: make(map[int64]domain.UserPrefs),
	}

	deps := &Deps{
		Bot:       mockBot,
		PrefsRepo: mockPrefs,
		ChatID:    "test-chat-123",
	}

	scheduler := New(deps)
	return scheduler, mockBot, mockPrefs
}

// ============================================================================
// Scheduler Lifecycle Tests
// ============================================================================

func TestScheduler_New(t *testing.T) {
	deps := &Deps{
		Bot:    &mockMessenger{},
		ChatID: "test",
	}

	s := New(deps)
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.deps != deps {
		t.Error("New() did not set deps correctly")
	}
	if s.stopCh == nil {
		t.Error("New() did not initialize stopCh")
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s, _, _ := newTestScheduler(t)
	ctx := context.Background()
	intervals := &Intervals{
		COTFetch: 100 * time.Millisecond,
	}

	// Start should not block
	s.Start(ctx, intervals)

	if !s.running {
		t.Error("Start() did not set running to true")
	}

	// Stop should complete cleanly
	s.Stop()

	if s.running {
		t.Error("Stop() did not set running to false")
	}
}

func TestScheduler_Start_DoubleStart(t *testing.T) {
	s, _, _ := newTestScheduler(t)
	ctx := context.Background()
	intervals := &Intervals{COTFetch: 100 * time.Millisecond}

	s.Start(ctx, intervals)
	defer s.Stop()

	// Second start should be no-op and not panic
	s.Start(ctx, intervals)
}

func TestScheduler_Stop_DoubleStop(t *testing.T) {
	s, _, _ := newTestScheduler(t)
	ctx := context.Background()
	intervals := &Intervals{COTFetch: 100 * time.Millisecond}

	s.Start(ctx, intervals)
	s.Stop()

	// Second stop should be no-op and not panic
	s.Stop()
}

func TestScheduler_Stop_NotRunning(t *testing.T) {
	s, _, _ := newTestScheduler(t)

	// Stop on not-running scheduler should not panic
	s.Stop()
}

// ============================================================================
// Job Execution Tests
// ============================================================================

func TestScheduler_runJob_CompletesSuccessfully(t *testing.T) {
	s, _, _ := newTestScheduler(t)

	var executed bool
	job := func(ctx context.Context) error {
		executed = true
		return nil
	}

	s.runJob(context.Background(), "test-job", job)

	if !executed {
		t.Error("runJob() did not execute the job function")
	}
}

func TestScheduler_runJob_ReturnsError(t *testing.T) {
	s, _, _ := newTestScheduler(t)

	expectedErr := errors.New("job failed")
	job := func(ctx context.Context) error {
		return expectedErr
	}

	// Should not panic, should log error (we can't capture logs easily here)
	s.runJob(context.Background(), "failing-job", job)
	// Test passes if no panic
}

func TestScheduler_runJob_PanicRecovery(t *testing.T) {
	s, _, _ := newTestScheduler(t)

	var executed atomic.Bool
	job := func(ctx context.Context) error {
		executed.Store(true)
		panic("intentional panic for test")
	}

	// Should recover from panic and not crash
	s.runJob(context.Background(), "panic-job", job)

	if !executed.Load() {
		t.Error("runJob() did not execute the job before panic")
	}
	// Test passes if no panic propagated
}

func TestScheduler_runJob_ContextTimeout(t *testing.T) {
	s, _, _ := newTestScheduler(t)

	var executed atomic.Bool
	job := func(ctx context.Context) error {
		executed.Store(true)
		// Try to exceed the 5-minute timeout
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	s.runJob(ctx, "timeout-job", job)

	if !executed.Load() {
		t.Error("runJob() did not execute the job")
	}
}

func TestScheduler_runJob_RespectsCancellation(t *testing.T) {
	s, _, _ := newTestScheduler(t)

	ctx, cancel := context.WithCancel(context.Background())

	var executed atomic.Bool
	job := func(ctx context.Context) error {
		executed.Store(true)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	cancel() // Cancel before running
	s.runJob(ctx, "cancelled-job", job)

	// Job should still be called, but context should be cancelled
	if !executed.Load() {
		t.Error("runJob() did not execute the job")
	}
}

// ============================================================================
// Alert Broadcasting Tests
// ============================================================================

func TestScheduler_broadcastCOTRelease_SkipsBannedUsers(t *testing.T) {
	s, mockBot, mockPrefs := newTestScheduler(t)

	// Setup users
	mockPrefs.users[1] = domain.UserPrefs{
		COTAlertsEnabled: true,
		ChatID:           "chat-1",
	}
	mockPrefs.users[2] = domain.UserPrefs{
		COTAlertsEnabled: true,
		ChatID:           "chat-2",
	}

	// Mark user 2 as banned
	banCheck := func(ctx context.Context, userID int64) bool {
		return userID == 2
	}
	s.deps.IsBanned = banCheck

	ctx := context.Background()
	date := time.Now()
	analyses := []domain.COTAnalysis{}

	s.broadcastCOTRelease(ctx, date, analyses)

	mockBot.mu.Lock()
	defer mockBot.mu.Unlock()

	// Should only send to user 1 (not banned)
	if len(mockBot.chatIDs) != 1 {
		t.Errorf("Expected 1 message, got %d", len(mockBot.chatIDs))
	}
	if len(mockBot.chatIDs) > 0 && mockBot.chatIDs[0] != "chat-1" {
		t.Errorf("Expected message to chat-1, got %s", mockBot.chatIDs[0])
	}
}

func TestScheduler_broadcastCOTRelease_RespectsAlertPrefs(t *testing.T) {
	s, mockBot, mockPrefs := newTestScheduler(t)

	// Setup users with different alert preferences
	mockPrefs.users[1] = domain.UserPrefs{
		COTAlertsEnabled: true,
		ChatID:           "chat-1",
	}
	mockPrefs.users[2] = domain.UserPrefs{
		COTAlertsEnabled: false, // Disabled
		ChatID:           "chat-2",
	}
	mockPrefs.users[3] = domain.UserPrefs{
		COTAlertsEnabled: true,
		ChatID:           "", // No chat ID
	}

	ctx := context.Background()
	date := time.Now()
	analyses := []domain.COTAnalysis{}

	s.broadcastCOTRelease(ctx, date, analyses)

	mockBot.mu.Lock()
	defer mockBot.mu.Unlock()

	// Should only send to user 1
	if len(mockBot.chatIDs) != 1 {
		t.Errorf("Expected 1 message, got %d", len(mockBot.chatIDs))
	}
}

func TestScheduler_jobFREDAlerts_NoCrashWithoutFREDData(t *testing.T) {
	s, _, _ := newTestScheduler(t)

	// Setup a user
	ctx := context.Background()

	// This test just verifies the job doesn't crash when FRED data isn't available
	// The actual FRED alert filtering tests would need mocked FRED data
	err := s.jobFREDAlerts(ctx)

	// May return error or nil depending on cache state, but should not panic
	// If error, it should be about FRED fetch, not nil pointer
	if err != nil {
		// Expected - FRED API not available in tests
		t.Logf("jobFREDAlerts returned error (expected): %v", err)
	}
}

// ============================================================================
// Alert Gate Tests
// ============================================================================

func TestScheduler_ShouldDeliverAlert_NoGate(t *testing.T) {
	s, _, _ := newTestScheduler(t)

	prefs := domain.UserPrefs{
		COTAlertsEnabled: true,
		ChatID:           "test-chat",
	}

	// When alertGate is nil, should always return true
	ok, reason := s.ShouldDeliverAlert(prefs, domain.AlertTypeCOTRelease)
	if !ok {
		t.Errorf("Expected ok=true when alertGate is nil, got ok=%v, reason=%s", ok, reason)
	}
}

func TestScheduler_RecordAlertDelivery_NoGate(t *testing.T) {
	s, _, _ := newTestScheduler(t)

	// Should not panic when alertGate is nil
	ctx := context.Background()
	s.RecordAlertDelivery(ctx, "test-chat")
}

// ============================================================================
// Concurrent Safety Tests
// ============================================================================

func TestScheduler_ConcurrentStartStop(t *testing.T) {
	s, _, _ := newTestScheduler(t)
	ctx := context.Background()
	intervals := &Intervals{COTFetch: 50 * time.Millisecond}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Start(ctx, intervals)
		}()
	}

	wg.Wait()
	s.Stop()

	// Should not have panicked or deadlocked
}

func TestScheduler_ConcurrentJobExecution(t *testing.T) {
	s, _, _ := newTestScheduler(t)

	var counter atomic.Int32
	job := func(ctx context.Context) error {
		counter.Add(1)
		return nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.runJob(context.Background(), "concurrent-job", job)
		}()
	}

	wg.Wait()

	if counter.Load() != 100 {
		t.Errorf("Expected 100 job executions, got %d", counter.Load())
	}
}

// ============================================================================
// Helper Tests
// ============================================================================

func TestCotReleaseDate(t *testing.T) {
	tests := []struct {
		name       string
		reportDate time.Time
		expected   time.Weekday
	}{
		{
			name:       "Tuesday report -> Friday release",
			reportDate: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), // Tuesday
			expected:   time.Friday,
		},
		{
			name:       "Friday report -> next Friday release",
			reportDate: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), // Friday
			expected:   time.Friday,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cotReleaseDate(tt.reportDate)
			if result.Weekday() != tt.expected {
				t.Errorf("cotReleaseDate(%v) = %v (weekday: %v), expected weekday %v",
					tt.reportDate, result, result.Weekday(), tt.expected)
			}
		})
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestScheduler_FullLifecycle(t *testing.T) {
	s, mockBot, mockPrefs := newTestScheduler(t)

	// Setup test user
	mockPrefs.users[1] = domain.UserPrefs{
		COTAlertsEnabled: true,
		ChatID:           "test-chat-456",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	intervals := &Intervals{
		COTFetch: 200 * time.Millisecond,
	}

	// Start scheduler
	s.Start(ctx, intervals)

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop scheduler
	s.Stop()

	// Verify it started and stopped cleanly
	if s.running {
		t.Error("Scheduler should not be running after Stop()")
	}

	// Bot and Prefs may have been called depending on job timing
	_ = mockBot
	_ = mockPrefs
}
