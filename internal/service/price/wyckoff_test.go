package price

import (
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// buildAccumulationBars creates synthetic daily bars simulating an Accumulation scenario.
// Phase A: SC at bar 10 (heavy vol, down bar near support), AR at bar 15.
// Phase B: range-bound chop, bars 15-40.
// Phase C: Spring at bar 42 (brief dip below support + recovery).
// Phase D: SOS rally at bar 50 (strong up bar, high vol).
// Returns bars newest-first (as expected by AnalyzeWyckoff).
func buildAccumulationBars(n int) []domain.DailyPrice {
	base := 100.0
	bars := make([]domain.DailyPrice, n)
	t := time.Now().AddDate(0, 0, -n)

	for i := 0; i < n; i++ {
		b := domain.DailyPrice{
			Date:   t.AddDate(0, 0, i),
			Open:   base,
			High:   base + 0.5,
			Low:    base - 0.5,
			Close:  base,
			Volume: 1000,
		}

		// SC at i=10: large down bar, high vol, near support (97).
		if i == 10 {
			b.Open = base + 1
			b.High = base + 1.2
			b.Low = 96.8
			b.Close = 97.2
			b.Volume = 5000
			base = 97.2
		}

		// AR at i=15: rally after SC.
		if i == 15 {
			b.Open = base
			b.High = base + 2.5
			b.Low = base - 0.2
			b.Close = base + 2.2
			b.Volume = 2500
			base = b.Close
		}

		// Range chop 16-41: oscillate between ~97 and ~99.5 on moderate volume.
		if i > 15 && i <= 41 {
			cycle := float64(i-15) / 4.0
			if int(cycle)%2 == 0 {
				base = 97.5 + (cycle-float64(int(cycle)))*1.5
			} else {
				base = 99.0 - (cycle-float64(int(cycle)))*1.5
			}
			b.Open = base
			b.High = base + 0.4
			b.Low = base - 0.4
			b.Close = base
			b.Volume = 900
		}

		// Spring at i=42: brief dip below support (low ~96.5), close above support.
		if i == 42 {
			b.Open = 97.4
			b.High = 97.6
			b.Low = 96.3
			b.Close = 97.5
			b.Volume = 3000
			base = 97.5
		}

		// Recovery i=43-44.
		if i == 43 || i == 44 {
			base += 0.5
			b.Open = base - 0.3
			b.High = base + 0.4
			b.Low = base - 0.4
			b.Close = base
			b.Volume = 1800
		}

		// SOS at i=50: strong rally, high vol.
		if i == 50 {
			b.Open = base
			b.High = base + 3.0
			b.Low = base - 0.1
			b.Close = base + 2.8
			b.Volume = 4000
			base = b.Close
		}

		bars[i] = b
	}

	// Reverse to newest-first.
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		bars[i], bars[j] = bars[j], bars[i]
	}
	return bars
}

func TestAnalyzeWyckoff_MinBars(t *testing.T) {
	// Less than 60 bars should return nil.
	bars := buildAccumulationBars(59)
	result := AnalyzeWyckoff(bars)
	if result != nil {
		t.Errorf("expected nil for <60 bars, got %+v", result)
	}
}

func TestAnalyzeWyckoff_Accumulation(t *testing.T) {
	bars := buildAccumulationBars(90)
	result := AnalyzeWyckoff(bars)
	if result == nil {
		t.Fatal("expected non-nil WyckoffResult for accumulation scenario")
	}

	// Phase should be ACCUMULATION or MARKUP (Spring+SOS combo may push to MARKUP).
	if result.Phase != "ACCUMULATION" && result.Phase != "MARKUP" && result.Phase != "UNCERTAIN" {
		t.Errorf("unexpected phase %q — expected ACCUMULATION, MARKUP, or UNCERTAIN", result.Phase)
	}

	if result.Confidence < 0 || result.Confidence > 100 {
		t.Errorf("confidence out of range: %d", result.Confidence)
	}

	if result.SupportZone <= 0 {
		t.Errorf("support zone should be positive, got %f", result.SupportZone)
	}
	if result.ResistanceZone <= result.SupportZone {
		t.Errorf("resistance %.4f should be > support %.4f", result.ResistanceZone, result.SupportZone)
	}
	if result.Interpretation == "" {
		t.Error("interpretation should not be empty")
	}

	t.Logf("Phase: %s %s | Confidence: %d%% | Events: %d | %s",
		result.Phase, result.SubPhase, result.Confidence, len(result.KeyEvents), result.Interpretation)
}

func TestAnalyzeWyckoff_Fields(t *testing.T) {
	bars := buildAccumulationBars(70)
	result := AnalyzeWyckoff(bars)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	validPhases := map[string]bool{
		"ACCUMULATION": true, "MARKUP": true,
		"DISTRIBUTION": true, "MARKDOWN": true, "UNCERTAIN": true,
	}
	if !validPhases[result.Phase] {
		t.Errorf("invalid phase: %q", result.Phase)
	}

	validSubPhases := map[string]bool{
		"PHASE_A": true, "PHASE_B": true, "PHASE_C": true, "PHASE_D": true, "PHASE_E": true,
	}
	if !validSubPhases[result.SubPhase] {
		t.Errorf("invalid sub_phase: %q", result.SubPhase)
	}

	for _, ev := range result.KeyEvents {
		validTypes := map[string]bool{
			"PS": true, "SC": true, "AR": true, "ST": true,
			"SPRING": true, "UPTHRUST": true,
			"SOS": true, "SOW": true,
			"LPS": true, "LPSY": true,
		}
		if !validTypes[ev.Type] {
			t.Errorf("invalid event type: %q", ev.Type)
		}
		if ev.Price <= 0 {
			t.Errorf("event %s has invalid price %f", ev.Type, ev.Price)
		}
	}
}
