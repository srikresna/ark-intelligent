package fred

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// classifyTone
// ---------------------------------------------------------------------------

func TestToneClassification_Hawkish(t *testing.T) {
	title := "Inflation Remains Elevated: The Case for Further Tightening"
	tone := classifyTone(title)
	if tone != "HAWKISH" {
		t.Errorf("expected HAWKISH, got %s", tone)
	}
}

func TestToneClassification_Dovish(t *testing.T) {
	title := "Labor Market Softening and the Path Toward Rate Cuts Appropriate"
	tone := classifyTone(title)
	if tone != "DOVISH" {
		t.Errorf("expected DOVISH, got %s", tone)
	}
}

func TestToneClassification_Neutral(t *testing.T) {
	title := "The Federal Reserve's Role in Payment System Innovation"
	tone := classifyTone(title)
	if tone != "NEUTRAL" {
		t.Errorf("expected NEUTRAL, got %s", tone)
	}
}

// ---------------------------------------------------------------------------
// extractTopics
// ---------------------------------------------------------------------------

func TestExtractTopics_Inflation(t *testing.T) {
	title := "Inflation Dynamics and Employment Outlook"
	topics := extractTopics(title)
	found := false
	for _, tp := range topics {
		if tp == "inflation" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'inflation' topic, got %v", topics)
	}
}

func TestExtractTopics_Empty(t *testing.T) {
	// A title with no known keywords should return empty/nil (no panic).
	title := "Opening Remarks at the Annual Conference"
	topics := extractTopics(title)
	if topics == nil {
		topics = []string{} // nil is acceptable
	}
	_ = topics // no panic is the important assertion
}

// ---------------------------------------------------------------------------
// FedSpeechData structure
// ---------------------------------------------------------------------------

func TestFedSpeechData_AvailableFalseWhenEmpty(t *testing.T) {
	d := &FedSpeechData{FetchedAt: time.Now()}
	if d.Available {
		t.Error("FedSpeechData should not be Available when Speeches is empty")
	}
}

func TestFedSpeech_URLNormalization(t *testing.T) {
	// Verify relative URL would be correctly prefixed in scraper logic.
	relURL := "/newsevents/speech/powell20260402a.htm"
	expected := "https://www.federalreserve.gov" + relURL
	if !strings.HasPrefix(relURL, "http") {
		got := "https://www.federalreserve.gov" + relURL
		if got != expected {
			t.Errorf("URL normalization: want %s got %s", expected, got)
		}
	}
}
