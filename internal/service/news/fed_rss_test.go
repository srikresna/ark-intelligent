package news

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// parseSpeaker tests
// ---------------------------------------------------------------------------

func TestParseSpeaker_CommaSeparated(t *testing.T) {
	cases := []struct {
		title    string
		expected string
	}{
		{"Powell, Remarks on Economic Outlook", "Powell"},
		{"Barr, Brief Remarks on the Economic Outlook and Monetary Policy", "Barr"},
		{"Jefferson, Opening Statement", "Jefferson"},
		{"Waller, Comments on Inflation", "Waller"},
	}
	for _, c := range cases {
		got := parseSpeaker(c.title)
		if got != c.expected {
			t.Errorf("parseSpeaker(%q) = %q; want %q", c.title, got, c.expected)
		}
	}
}

func TestParseSpeaker_EmptyTitle(t *testing.T) {
	got := parseSpeaker("")
	if got != "" {
		t.Errorf("parseSpeaker(\"\") = %q; want \"\"", got)
	}
}

func TestParseSpeaker_NoComma(t *testing.T) {
	// Titles without commas — fallback to first word
	got := parseSpeaker("FOMC Statement Released")
	if got != "FOMC" {
		t.Errorf("parseSpeaker no-comma = %q; want \"FOMC\"", got)
	}
}

// ---------------------------------------------------------------------------
// votingMembers tests
// ---------------------------------------------------------------------------

func TestVotingMembers(t *testing.T) {
	votingExpected := []string{"powell", "jefferson", "waller", "cook", "barr"}
	for _, name := range votingExpected {
		if !votingMembers[name] {
			t.Errorf("expected %q to be a voting member", name)
		}
	}
}

func TestNonVotingMember(t *testing.T) {
	nonVoting := "kashkari" // typical non-voting
	if votingMembers[nonVoting] {
		t.Errorf("did not expect %q to be a voting member", nonVoting)
	}
}

// ---------------------------------------------------------------------------
// FedSpeech.AlertLevel tests
// ---------------------------------------------------------------------------

func TestAlertLevel_FOMCStatement(t *testing.T) {
	s := FedSpeech{Title: "FOMC Statement Released March 2026", Category: "Press Release"}
	if s.AlertLevel() != FedAlertCritical {
		t.Errorf("FOMC statement should be CRITICAL, got %s", s.AlertLevel())
	}
}

func TestAlertLevel_FOMCMinutes(t *testing.T) {
	s := FedSpeech{Title: "Minutes of the Federal Open Market Committee", Category: "Minutes"}
	if s.AlertLevel() != FedAlertHigh {
		t.Errorf("FOMC minutes should be HIGH, got %s", s.AlertLevel())
	}
}

func TestAlertLevel_VotingMemberSpeech(t *testing.T) {
	s := FedSpeech{
		Title:    "Powell, Remarks on the Economy",
		Speaker:  "Powell",
		IsVoting: true,
		Category: "Speech",
	}
	if s.AlertLevel() != FedAlertHigh {
		t.Errorf("voting member speech should be HIGH, got %s", s.AlertLevel())
	}
}

func TestAlertLevel_NonVotingMemberSpeech(t *testing.T) {
	s := FedSpeech{
		Title:    "Kashkari, Comments on Inflation",
		Speaker:  "Kashkari",
		IsVoting: false,
		Category: "Speech",
	}
	if s.AlertLevel() != FedAlertMedium {
		t.Errorf("non-voting member speech should be MEDIUM, got %s", s.AlertLevel())
	}
}

// ---------------------------------------------------------------------------
// FormatFedAlert tests
// ---------------------------------------------------------------------------

func TestFormatFedAlert_Critical(t *testing.T) {
	s := FedSpeech{
		Title:       "FOMC Statement Released",
		Category:    "Press Release",
		URL:         "https://www.federalreserve.gov/",
		PublishedAt: time.Date(2026, 3, 18, 18, 0, 0, 0, time.UTC),
	}
	html := FormatFedAlert(s, FedAlertCritical)
	if !strings.Contains(html, "🚨") {
		t.Error("CRITICAL alert should contain 🚨 icon")
	}
	if !strings.Contains(html, "FOMC Statement Released") {
		t.Error("alert should contain header")
	}
	if !strings.Contains(html, "https://www.federalreserve.gov/") {
		t.Error("alert should contain URL")
	}
}

func TestFormatFedAlert_High_Minutes(t *testing.T) {
	s := FedSpeech{
		Title:    "Minutes of the Federal Open Market Committee",
		Category: "Minutes",
		URL:      "https://www.federalreserve.gov/minutes",
	}
	html := FormatFedAlert(s, FedAlertHigh)
	if !strings.Contains(html, "📋") {
		t.Error("FOMC minutes alert should contain 📋 icon")
	}
}

func TestFormatFedAlert_High_VotingMember(t *testing.T) {
	s := FedSpeech{
		Title:    "Powell, Remarks on Economic Outlook",
		Speaker:  "Powell",
		IsVoting: true,
		Category: "Speech",
		URL:      "https://www.federalreserve.gov/speech1",
	}
	html := FormatFedAlert(s, FedAlertHigh)
	if !strings.Contains(html, "Powell") {
		t.Error("alert should include speaker name")
	}
	if !strings.Contains(html, "⭐") {
		t.Error("voting member alert should contain ⭐")
	}
}

func TestFormatFedAlert_Medium_NonVoting(t *testing.T) {
	s := FedSpeech{
		Title:       "Daly, Views on Labor Markets",
		Speaker:     "Daly",
		IsVoting:    false,
		Category:    "Speech",
		Description: "At the San Francisco Fed",
		URL:         "https://www.federalreserve.gov/speech2",
		PublishedAt: time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC),
	}
	html := FormatFedAlert(s, FedAlertMedium)
	if !strings.Contains(html, "🏛️") {
		t.Error("MEDIUM alert should contain 🏛️ icon")
	}
	if !strings.Contains(html, "Daly") {
		t.Error("alert should include speaker name")
	}
	if strings.Contains(html, "⭐") {
		t.Error("non-voting member should NOT have ⭐")
	}
}

// ---------------------------------------------------------------------------
// FedRSSScheduler — processItems deduplication test
// ---------------------------------------------------------------------------

func TestFedRSSScheduler_Deduplication(t *testing.T) {
	ctx := context.Background()
	alertCount := 0

	sched := NewFedRSSScheduler()
	sched.SetAlertSink(func(_ context.Context, _ string, _ FedAlertLevel) {
		alertCount++
	})

	cutoff := time.Now().UTC().Add(-24 * time.Hour)

	items := []FedSpeech{
		{
			GUID:        "guid-001",
			Title:       "Powell, Remarks on Economy",
			Speaker:     "Powell",
			IsVoting:    true,
			Category:    "Speech",
			PublishedAt: time.Now().UTC(),
		},
	}

	// First call — should trigger alert
	sched.processItems(ctx, items, cutoff)
	if alertCount != 1 {
		t.Errorf("expected 1 alert after first processItems, got %d", alertCount)
	}

	// Second call with same GUID — should be deduplicated
	sched.processItems(ctx, items, cutoff)
	if alertCount != 1 {
		t.Errorf("expected no duplicate alert (still 1), got %d", alertCount)
	}
}

func TestFedRSSScheduler_SkipsOldItems(t *testing.T) {
	ctx := context.Background()
	alertCount := 0

	sched := NewFedRSSScheduler()
	sched.SetAlertSink(func(_ context.Context, _ string, _ FedAlertLevel) {
		alertCount++
	})

	cutoff := time.Now().UTC().Add(-24 * time.Hour)

	// Item is 48 hours old — should be skipped
	items := []FedSpeech{
		{
			GUID:        "guid-old",
			Title:       "Barr, Old Remarks",
			Speaker:     "Barr",
			IsVoting:    true,
			Category:    "Speech",
			PublishedAt: time.Now().UTC().Add(-48 * time.Hour),
		},
	}

	sched.processItems(ctx, items, cutoff)
	if alertCount != 0 {
		t.Errorf("expected 0 alerts for old item, got %d", alertCount)
	}
}
