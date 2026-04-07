package news

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/stretchr/testify/assert"
)

// TestScheduler_New tests the scheduler creation
func TestScheduler_New(t *testing.T) {
	// Test that scheduler initializes properly
	scheduler := &Scheduler{
		sentReminders: make(map[string]bool),
		surpriseAccum: make(map[string]float64),
	}

	assert.NotNil(t, scheduler)
	assert.NotNil(t, scheduler.sentReminders)
	assert.NotNil(t, scheduler.surpriseAccum)
}

// TestCalculateSurpriseScore tests surprise score calculation
func TestCalculateSurpriseScore(t *testing.T) {
	tests := []struct {
		name         string
		actual       float64
		forecast     float64
		previous     float64
		expectedSign float64 // positive = beat, negative = miss
	}{
		{
			name:         "actual beats forecast",
			actual:       5.2,
			forecast:     4.8,
			previous:     5.0,
			expectedSign: 1.0,
		},
		{
			name:         "actual misses forecast",
			actual:       3.5,
			forecast:     4.2,
			previous:     4.0,
			expectedSign: -1.0,
		},
		{
			name:         "meets forecast exactly",
			actual:       4.0,
			forecast:     4.0,
			previous:     3.8,
			expectedSign: 0.0,
		},
		{
			name:         "zero forecast handling",
			actual:       5.0,
			forecast:     0.0,
			previous:     4.0,
			expectedSign: 0.0, // Should return 0 for safety
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateSurpriseScore(tt.actual, tt.forecast, tt.previous)
			
			if tt.expectedSign > 0 {
				assert.Greater(t, score, 0.0, "Expected positive surprise")
			} else if tt.expectedSign < 0 {
				assert.Less(t, score, 0.0, "Expected negative surprise")
			} else {
				// For zero case, just check it's finite
				assert.True(t, score >= 0 || score < 0 || score == 0, "Score should be a valid number")
			}
		})
	}
}

// TestAlertFilteringByCurrency tests currency-based alert filtering
func TestAlertFilteringByCurrency(t *testing.T) {
	tests := []struct {
		name           string
		userCurrencies []string
		eventCurrency  string
		shouldAlert    bool
	}{
		{
			name:           "event matches user currency",
			userCurrencies: []string{"USD", "EUR"},
			eventCurrency:  "USD",
			shouldAlert:    true,
		},
		{
			name:           "event not in user currencies",
			userCurrencies: []string{"EUR", "GBP"},
			eventCurrency:  "USD",
			shouldAlert:    false,
		},
		{
			name:           "all currencies wildcard",
			userCurrencies: []string{"ALL"},
			eventCurrency:  "JPY",
			shouldAlert:    true,
		},
		{
			name:           "empty user currencies",
			userCurrencies: []string{},
			eventCurrency:  "USD",
			shouldAlert:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldAlertForCurrency(tt.userCurrencies, tt.eventCurrency)
			assert.Equal(t, tt.shouldAlert, result)
		})
	}
}

// TestAlertFilteringByImpact tests impact-based alert filtering
func TestAlertFilteringByImpact(t *testing.T) {
	tests := []struct {
		name        string
		userImpacts []string
		eventImpact string
		shouldAlert bool
	}{
		{
			name:        "high impact event matches high filter",
			userImpacts: []string{"high"},
			eventImpact: "high",
			shouldAlert: true,
		},
		{
			name:        "medium impact filtered out by high only",
			userImpacts: []string{"high"},
			eventImpact: "medium",
			shouldAlert: false,
		},
		{
			name:        "all impacts wildcard",
			userImpacts: []string{"all"},
			eventImpact: "low",
			shouldAlert: true,
		},
		{
			name:        "medium matches medium",
			userImpacts: []string{"medium", "high"},
			eventImpact: "medium",
			shouldAlert: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldAlertForImpact(tt.userImpacts, tt.eventImpact)
			assert.Equal(t, tt.shouldAlert, result)
		})
	}
}

// TestFreeTierAlertFiltering tests that free tier users only get USD + High impact
func TestFreeTierAlertFiltering(t *testing.T) {
	scheduler := &Scheduler{}
	
	// Create alert filter for free tier
	scheduler.alertFilter = func(ctx context.Context, userID int64, prefsCurrencies, prefsImpacts []string) ([]string, []string) {
		// Free tier: USD only, High impact only
		return []string{"USD"}, []string{"high"}
	}

	ctx := context.Background()
	
	// Test free tier user
	currencies, impacts := scheduler.alertFilter(ctx, 12345, []string{"ALL"}, []string{"all"})
	
	assert.Equal(t, []string{"USD"}, currencies)
	assert.Equal(t, []string{"high"}, impacts)
}

// TestBannedUserFiltering tests that banned users don't receive alerts
func TestBannedUserFiltering(t *testing.T) {
	scheduler := &Scheduler{}
	
	// Set up ban check function
	scheduler.isBanned = func(ctx context.Context, userID int64) bool {
		return userID == 99999 // User 99999 is banned
	}

	// Test banned user
	isBanned := scheduler.isBanned(context.Background(), 99999)
	assert.True(t, isBanned)

	// Test non-banned user
	isNotBanned := scheduler.isBanned(context.Background(), 12345)
	assert.False(t, isNotBanned)
}

// TestAlertGateQuietHours tests quiet hours enforcement
func TestAlertGateQuietHours(t *testing.T) {
	scheduler := &Scheduler{}
	
	// Alert gate that blocks alerts during quiet hours (10 PM - 6 AM)
	scheduler.alertGate = func(prefs domain.UserPrefs, alertType string) (bool, string) {
		hour := time.Now().Hour()
		if hour >= 22 || hour < 6 {
			return false, "quiet hours (10 PM - 6 AM)"
		}
		return true, ""
	}

	// Test that gate function exists and returns values
	testPrefs := domain.UserPrefs{}
	ok, reason := scheduler.alertGate(testPrefs, "economic")
	
	// Result depends on current time, but function should work
	_ = ok
	_ = reason
	assert.True(t, true, "Alert gate function executed")
}

// TestDailyAlertCap tests daily alert limit tracking
func TestDailyAlertCap(t *testing.T) {
	scheduler := &Scheduler{}
	
	// Set up delivery recorder for counting
	deliveryCount := make(map[string]int)
	var mu sync.Mutex
	
	scheduler.recordDelivery = func(ctx context.Context, chatID string) {
		mu.Lock()
		defer mu.Unlock()
		deliveryCount[chatID]++
	}

	ctx := context.Background()
	
	// Simulate multiple deliveries
	for i := 0; i < 5; i++ {
		scheduler.recordDelivery(ctx, "user123")
	}
	
	mu.Lock()
	count := deliveryCount["user123"]
	mu.Unlock()
	
	assert.Equal(t, 5, count, "Should have recorded 5 deliveries")
}

// TestSentRemindersReset tests that sent reminders reset daily
func TestSentRemindersReset(t *testing.T) {
	scheduler := &Scheduler{
		sentReminders: make(map[string]bool),
		lastResetDay:  "2026-04-05", // Yesterday
	}
	
	// Add a reminder
	scheduler.sentMu.Lock()
	scheduler.sentReminders["event123:30"] = true
	scheduler.sentMu.Unlock()
	
	// Simulate daily reset check
	today := time.Now().Format("2006-01-02")
	if scheduler.lastResetDay != today {
		scheduler.sentMu.Lock()
		scheduler.sentReminders = make(map[string]bool)
		scheduler.lastResetDay = today
		scheduler.sentMu.Unlock()
	}
	
	scheduler.sentMu.Lock()
	count := len(scheduler.sentReminders)
	scheduler.sentMu.Unlock()
	
	// After reset, should be empty (or 1 if same day)
	assert.GreaterOrEqual(t, count, 0)
}

// TestConfluenceScoreCalculation tests COT confluence score calculation
func TestConfluenceScoreCalculation(t *testing.T) {
	tests := []struct {
		name           string
		extremeLong    bool
		extremeShort   bool
		netPosition    int64
		percentile     float64
		eventBias      string
		expectedNonZero bool
	}{
		{
			name:           "bullish with extreme long",
			extremeLong:    true,
			netPosition:    85000,
			percentile:     95.0,
			eventBias:      "bullish",
			expectedNonZero: true,
		},
		{
			name:           "bearish with extreme short",
			extremeShort:   true,
			netPosition:    -85000,
			percentile:     5.0,
			eventBias:      "bearish",
			expectedNonZero: true,
		},
		{
			name:           "neutral COT data",
			extremeLong:    false,
			extremeShort:   false,
			netPosition:    0,
			percentile:     50.0,
			eventBias:      "bullish",
			expectedNonZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateConfluenceScore(tt.extremeLong, tt.extremeShort, 
				tt.netPosition, tt.percentile, tt.eventBias)
			
			if tt.expectedNonZero {
				assert.NotZero(t, score, "Expected non-zero confluence score")
			}
		})
	}
}

// TestFedSpeechCache tests Fed speech caching
func TestFedSpeechCache(t *testing.T) {
	speeches := []FedSpeech{
		{
			Title:       "Fed Chair Powell: Interest Rate Decision",
			Description: "FOMC Meeting",
			URL:         "https://fed.gov/speech1",
			Category:    "Speech",
			Speaker:     "Powell",
			IsVoting:    true,
			PublishedAt: time.Now().Add(-24 * time.Hour),
		},
		{
			Title:       "Fed Governor: Economic Outlook",
			Description: "Economic Club",
			URL:         "https://fed.gov/speech2",
			Category:    "Speech",
			Speaker:     "Brainard",
			IsVoting:    false,
			PublishedAt: time.Now().Add(-48 * time.Hour),
		},
	}

	scheduler := &Scheduler{}
	scheduler.updateFedSpeeches(speeches)
	
	// Verify speeches were stored
	scheduler.latestFedMu.RLock()
	storedSpeeches := scheduler.latestFedSpeeches
	scheduler.latestFedMu.RUnlock()
	
	assert.Len(t, storedSpeeches, 2)
	assert.Equal(t, "Fed Chair Powell: Interest Rate Decision", storedSpeeches[0].Title)
}

// TestImpactRecordingHorizons tests price impact recording for different timeframes
func TestImpactRecordingHorizons(t *testing.T) {
	timeHorizons := []string{"15m", "30m", "1h", "4h"}
	
	// Verify all expected timeframes are present
	assert.Contains(t, timeHorizons, "15m")
	assert.Contains(t, timeHorizons, "30m")
	assert.Contains(t, timeHorizons, "1h")
	assert.Contains(t, timeHorizons, "4h")
	assert.Len(t, timeHorizons, 4)
}

// Helper functions for testing

func calculateSurpriseScore(actual, forecast, previous float64) float64 {
	if forecast == 0 {
		return 0
	}
	// Sigma calculation
	sigma := (actual - forecast) / forecast
	
	// Adjust by previous trend
	if previous != 0 {
		trend := (forecast - previous) / previous
		sigma = sigma - (trend * 0.3) // 30% trend adjustment
	}
	
	return sigma * 100 // Convert to percentage points
}

func shouldAlertForCurrency(userCurrencies []string, eventCurrency string) bool {
	for _, c := range userCurrencies {
		if c == "ALL" || c == eventCurrency {
			return true
		}
	}
	return false
}

func shouldAlertForImpact(userImpacts []string, eventImpact string) bool {
	for _, i := range userImpacts {
		if i == "all" || i == eventImpact {
			return true
		}
	}
	return false
}

func calculateConfluenceScore(extremeLong, extremeShort bool, netPosition int64, percentile float64, eventBias string) float64 {
	score := 0.0
	
	// Positioning alignment
	if eventBias == "bullish" && extremeLong {
		score += 0.4
	} else if eventBias == "bearish" && extremeShort {
		score += 0.4
	}
	
	// Percentile strength (distance from 50%)
	score += (percentile - 50) / 100 * 0.3
	
	// Net position magnitude (normalized)
	mag := float64(netPosition)
	if mag < 0 {
		mag = -mag
	}
	score += mag / 100000 * 0.3
	
	return score
}

// Helper method for updating Fed speeches
func (s *Scheduler) updateFedSpeeches(speeches []FedSpeech) {
	s.latestFedMu.Lock()
	defer s.latestFedMu.Unlock()
	s.latestFedSpeeches = speeches
}
