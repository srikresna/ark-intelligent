// +build integration

package news

import (
	"context"
	"testing"
	"time"
)

// TestMQL5FetchAndImpactDirection verifies that MQL5 API returns events with
// ImpactDirection populated, and that OldPreviousValue (revision tracking) works.
// Run: go test -tags=integration -run TestMQL5FetchAndImpactDirection -v ./internal/service/news/
func TestMQL5FetchAndImpactDirection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fetcher := NewMQL5Fetcher()

	t.Run("ScrapeCalendar_ThisWeek", func(t *testing.T) {
		events, err := fetcher.ScrapeCalendar(ctx, "this")
		if err != nil {
			t.Skipf("ScrapeCalendar failed (MQL5 may block this environment): %v", err)
		}

		t.Logf("Fetched %d events for this week", len(events))
		if len(events) == 0 {
			t.Fatal("No events returned — MQL5 API may be down")
		}

		// HIGH PRIORITY 3: ImpactDirection must be captured
		impactDirCount := 0
		oldPrevCount := 0
		releasedCount := 0

		for _, e := range events {
			if e.ImpactDirection != 0 {
				impactDirCount++
			}
			if e.OldPrevious != "" {
				oldPrevCount++
			}
			if e.Actual != "" {
				releasedCount++
			}
		}

		t.Logf("Events with ImpactDirection != 0: %d / %d", impactDirCount, len(events))
		t.Logf("Events with OldPrevious (revision): %d / %d", oldPrevCount, len(events))
		t.Logf("Released events (has Actual): %d / %d", releasedCount, len(events))

		// ImpactDirection should be present on released events
		if releasedCount > 0 && impactDirCount == 0 {
			t.Error("No events have ImpactDirection set — MQL5 field mapping may be broken")
		}

		// Show sample events with ImpactDirection
		shown := 0
		for _, e := range events {
			if e.Actual != "" && shown < 5 {
				surprise := "N/A"
				if e.Forecast != "" && e.Actual != "" {
					aVal, aOk := ParseNumericValue(e.Actual)
					fVal, fOk := ParseNumericValue(e.Forecast)
					if aOk && fOk {
						sigma := ComputeSurpriseWithDirection(aVal, fVal, nil, e.ImpactDirection)
						label := ClassifySurpriseWithDirection(sigma, e.ImpactDirection)
						surprise = label
					}
				}
				t.Logf("  %s %s: Actual=%s Fcast=%s ImpDir=%d → %s",
					e.Currency, e.Event, e.Actual, e.Forecast, e.ImpactDirection, surprise)
				shown++
			}
		}

		// Show revision samples
		shown = 0
		for _, e := range events {
			if e.OldPrevious != "" && shown < 3 {
				t.Logf("  REVISION: %s %s: Prev=%s OldPrev=%s",
					e.Currency, e.Event, e.Previous, e.OldPrevious)
				shown++
			}
		}
	})

	t.Run("SurpriseWithDirection_InvertedIndicator", func(t *testing.T) {
		// Test: Unemployment Claims up = BAD for currency (ImpactDirection=2)
		// actual=250K, forecast=220K → raw diff is positive but bearish
		sigma := ComputeSurpriseWithDirection(250000, 220000, nil, 2)
		if sigma >= 0 {
			t.Errorf("Expected negative sigma for bearish ImpactDirection=2 with actual>forecast, got %.2f", sigma)
		}
		t.Logf("Unemployment Claims: actual=250K fcast=220K impDir=2 → sigma=%.4f (%s)",
			sigma, ClassifySurpriseWithDirection(sigma, 2))

		// Test: NFP up = GOOD for currency (ImpactDirection=1)
		// actual=200K, forecast=180K → positive and bullish
		sigma2 := ComputeSurpriseWithDirection(200000, 180000, nil, 1)
		if sigma2 <= 0 {
			t.Errorf("Expected positive sigma for bullish ImpactDirection=1 with actual>forecast, got %.2f", sigma2)
		}
		t.Logf("NFP: actual=200K fcast=180K impDir=1 → sigma=%.4f (%s)",
			sigma2, ClassifySurpriseWithDirection(sigma2, 1))

		// Test: neutral direction (0) falls back to raw diff
		sigma3 := ComputeSurpriseWithDirection(200000, 180000, nil, 0)
		if sigma3 <= 0 {
			t.Errorf("Expected positive sigma for neutral ImpactDirection=0 with actual>forecast, got %.2f", sigma3)
		}
		t.Logf("Neutral: actual=200K fcast=180K impDir=0 → sigma=%.4f (%s)",
			sigma3, ClassifySurpriseWithDirection(sigma3, 0))
	})
}
