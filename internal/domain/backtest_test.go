package domain

import (
	"testing"
	"time"
)

func TestNeedsEvaluation_ZeroEntryPrice(t *testing.T) {
	s := PersistedSignal{
		EntryPrice: 0,
		ReportDate: time.Now().Add(-14 * 24 * time.Hour),
	}
	if s.NeedsEvaluation(time.Now()) {
		t.Error("signal with zero entry price should not need evaluation")
	}
}

func TestNeedsEvaluation_TooYoung(t *testing.T) {
	s := PersistedSignal{
		EntryPrice: 1.08,
		ReportDate: time.Now().Add(-3 * 24 * time.Hour), // 3 days old
	}
	if s.NeedsEvaluation(time.Now()) {
		t.Error("signal younger than 7 days should not need evaluation")
	}
}

func TestNeedsEvaluation_1WPending(t *testing.T) {
	s := PersistedSignal{
		EntryPrice: 1.08,
		ReportDate: time.Now().Add(-8 * 24 * time.Hour), // 8 days old
		Outcome1W:  "",                                  // not evaluated
	}
	if !s.NeedsEvaluation(time.Now()) {
		t.Error("signal with empty 1W outcome and age > 7 days should need evaluation")
	}
}

func TestNeedsEvaluation_1WDoneBut2WPending(t *testing.T) {
	s := PersistedSignal{
		EntryPrice: 1.08,
		ReportDate: time.Now().Add(-15 * 24 * time.Hour), // 15 days old
		Outcome1W:  OutcomeWin,
		Outcome2W:  "", // not evaluated
	}
	if !s.NeedsEvaluation(time.Now()) {
		t.Error("signal with 1W done but 2W pending at age > 14 days should need evaluation")
	}
}

func TestNeedsEvaluation_1W2WDoneBut4WPending(t *testing.T) {
	s := PersistedSignal{
		EntryPrice: 1.08,
		ReportDate: time.Now().Add(-30 * 24 * time.Hour), // 30 days old
		Outcome1W:  OutcomeWin,
		Outcome2W:  OutcomeLoss,
		Outcome4W:  "", // not evaluated
	}
	if !s.NeedsEvaluation(time.Now()) {
		t.Error("signal with 4W pending at age > 28 days should need evaluation")
	}
}

func TestNeedsEvaluation_FullyEvaluated(t *testing.T) {
	s := PersistedSignal{
		EntryPrice: 1.08,
		ReportDate: time.Now().Add(-60 * 24 * time.Hour),
		Outcome1W:  OutcomeWin,
		Outcome2W:  OutcomeLoss,
		Outcome4W:  OutcomeWin,
	}
	if s.NeedsEvaluation(time.Now()) {
		t.Error("fully evaluated signal should not need evaluation")
	}
}

func TestNeedsEvaluation_2WNotYetOldEnough(t *testing.T) {
	// 10 days old — 1W is done, but 2W is pending and age < 14 days
	s := PersistedSignal{
		EntryPrice: 1.08,
		ReportDate: time.Now().Add(-10 * 24 * time.Hour),
		Outcome1W:  OutcomeWin,
		Outcome2W:  "", // pending but too young
	}
	if s.NeedsEvaluation(time.Now()) {
		t.Error("signal with 2W pending but age < 14 days should NOT need evaluation")
	}
}

func TestIsFullyEvaluated(t *testing.T) {
	tests := []struct {
		name string
		sig  PersistedSignal
		want bool
	}{
		{
			"all done",
			PersistedSignal{Outcome1W: OutcomeWin, Outcome2W: OutcomeLoss, Outcome4W: OutcomeWin},
			true,
		},
		{
			"1W pending",
			PersistedSignal{Outcome1W: OutcomePending, Outcome2W: OutcomeLoss, Outcome4W: OutcomeWin},
			false,
		},
		{
			"4W empty",
			PersistedSignal{Outcome1W: OutcomeWin, Outcome2W: OutcomeLoss, Outcome4W: ""},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sig.IsFullyEvaluated(); got != tt.want {
				t.Errorf("IsFullyEvaluated() = %v, want %v", got, tt.want)
			}
		})
	}
}
