package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestUserFriendlyError_Nil(t *testing.T) {
	if got := userFriendlyError(nil, "test"); got != "" {
		t.Errorf("expected empty string for nil error, got %q", got)
	}
}

func TestUserFriendlyError_Timeout(t *testing.T) {
	msg := userFriendlyError(context.DeadlineExceeded, "price")
	if !strings.Contains(msg, "timeout") && !strings.Contains(msg, "Request timeout") {
		t.Errorf("expected timeout message, got %q", msg)
	}
	if !strings.Contains(msg, "/price") {
		t.Errorf("expected command suggestion with /price, got %q", msg)
	}
}

func TestUserFriendlyError_NotFound(t *testing.T) {
	msg := userFriendlyError(errors.New("badger: key not found"), "cot")
	if !strings.Contains(msg, "belum tersedia") {
		t.Errorf("expected data not found message, got %q", msg)
	}
}

func TestUserFriendlyError_Network(t *testing.T) {
	msg := userFriendlyError(errors.New("dial tcp: connection refused"), "macro")
	if !strings.Contains(msg, "Koneksi gagal") {
		t.Errorf("expected connection error message, got %q", msg)
	}
}

func TestUserFriendlyError_RateLimit(t *testing.T) {
	msg := userFriendlyError(errors.New("429 too many requests"), "alpha")
	if !strings.Contains(msg, "Batas request") {
		t.Errorf("expected rate limit message, got %q", msg)
	}
}

func TestUserFriendlyError_Auth(t *testing.T) {
	msg := userFriendlyError(errors.New("403 forbidden"), "admin")
	if !strings.Contains(msg, "Akses ditolak") {
		t.Errorf("expected auth error message, got %q", msg)
	}
}

func TestUserFriendlyError_Badger(t *testing.T) {
	msg := userFriendlyError(errors.New("badger: value log truncated"), "settings")
	if !strings.Contains(msg, "penyimpanan") {
		t.Errorf("expected storage error message, got %q", msg)
	}
}

func TestUserFriendlyError_Insufficient(t *testing.T) {
	msg := userFriendlyError(errors.New("insufficient data for analysis"), "backtest")
	if !strings.Contains(msg, "belum cukup") {
		t.Errorf("expected insufficient data message, got %q", msg)
	}
}

func TestUserFriendlyError_AI(t *testing.T) {
	msg := userFriendlyError(errors.New("gemini generation failed"), "cta")
	if !strings.Contains(msg, "AI") {
		t.Errorf("expected AI error message, got %q", msg)
	}
	// Also test " ai " with spaces
	msg2 := userFriendlyError(errors.New("the ai service is down"), "cta")
	if !strings.Contains(msg2, "AI") {
		t.Errorf("expected AI error message for ' ai ' pattern, got %q", msg2)
	}
}

func TestUserFriendlyError_Chart(t *testing.T) {
	msg := userFriendlyError(errors.New("chart render failed"), "levels")
	if !strings.Contains(msg, "chart") {
		t.Errorf("expected chart error message, got %q", msg)
	}
}

func TestUserFriendlyError_Generic(t *testing.T) {
	msg := userFriendlyError(errors.New("something completely unknown"), "test")
	if !strings.Contains(msg, "kesalahan") {
		t.Errorf("expected generic error message, got %q", msg)
	}
}

func TestSuggestRetry_EmptyCommand(t *testing.T) {
	got := suggestRetry("")
	if !strings.Contains(got, "Coba ulangi") {
		t.Errorf("expected retry suggestion, got %q", got)
	}
}

func TestSuggestRetry_WithCommand(t *testing.T) {
	got := suggestRetry("price")
	if !strings.Contains(got, "/price") {
		t.Errorf("expected /price in suggestion, got %q", got)
	}
}
