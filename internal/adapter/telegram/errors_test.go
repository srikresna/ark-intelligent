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

// ---------------------------------------------------------------------------
// callbackFriendlyError tests
// ---------------------------------------------------------------------------

func TestCallbackFriendlyError_Nil(t *testing.T) {
	if got := callbackFriendlyError(nil); got != "" {
		t.Errorf("expected empty string for nil error, got %q", got)
	}
}

func TestCallbackFriendlyError_Timeout(t *testing.T) {
	msg := callbackFriendlyError(context.DeadlineExceeded)
	if !strings.Contains(msg, "timeout") && !strings.Contains(msg, "Timeout") {
		t.Errorf("expected timeout message, got %q", msg)
	}
	if len([]rune(msg)) > 200 {
		t.Errorf("message exceeds 200 chars: len=%d, msg=%q", len([]rune(msg)), msg)
	}
}

func TestCallbackFriendlyError_RateLimit(t *testing.T) {
	msg := callbackFriendlyError(errors.New("429 too many requests"))
	if !strings.Contains(msg, "Batas request") {
		t.Errorf("expected rate limit message, got %q", msg)
	}
	if len([]rune(msg)) > 200 {
		t.Errorf("message exceeds 200 chars")
	}
}

func TestCallbackFriendlyError_Network(t *testing.T) {
	msg := callbackFriendlyError(errors.New("dial tcp: connection refused"))
	if !strings.Contains(msg, "Koneksi gagal") {
		t.Errorf("expected connection error message, got %q", msg)
	}
}

func TestCallbackFriendlyError_AI(t *testing.T) {
	msg := callbackFriendlyError(errors.New("gemini generation failed"))
	if !strings.Contains(msg, "AI") {
		t.Errorf("expected AI error message, got %q", msg)
	}
}

func TestCallbackFriendlyError_Generic(t *testing.T) {
	msg := callbackFriendlyError(errors.New("completely unknown error"))
	if !strings.Contains(msg, "kesalahan") {
		t.Errorf("expected generic error message, got %q", msg)
	}
	if len([]rune(msg)) > 200 {
		t.Errorf("message exceeds 200 chars")
	}
}

func TestCallbackFriendlyError_AllUnder200Chars(t *testing.T) {
	testErrs := []error{
		context.DeadlineExceeded,
		errors.New("timeout"),
		errors.New("not found"),
		errors.New("key not found"),
		errors.New("insufficient data"),
		errors.New("connection refused"),
		errors.New("dial tcp: no such host"),
		errors.New("429 rate limit"),
		errors.New("quota exceeded"),
		errors.New("chart render failed"),
		errors.New("gemini generation failed"),
		errors.New("403 forbidden"),
		errors.New("401 unauthorized"),
		errors.New("badger: value log error"),
		errors.New("some unknown error xyz"),
	}
	for _, err := range testErrs {
		msg := callbackFriendlyError(err)
		if l := len([]rune(msg)); l > 200 {
			t.Errorf("callbackFriendlyError(%q) = %d chars (>200): %q", err, l, msg)
		}
	}
}

// ---------------------------------------------------------------------------
// sessionExpiredMessage tests
// ---------------------------------------------------------------------------

func TestSessionExpiredMessage_ContainsCommand(t *testing.T) {
	commands := []string{"cta", "ict", "quant", "smc", "vp", "alpha", "wyckoff"}
	for _, cmd := range commands {
		msg := sessionExpiredMessage(cmd)
		if !strings.Contains(msg, "/"+cmd) {
			t.Errorf("sessionExpiredMessage(%q) missing /<command>, got %q", cmd, msg)
		}
	}
}

func TestSessionExpiredMessage_ConsistentEmoji(t *testing.T) {
	// All expired messages must use the canonical ⏳ emoji, not ⏰ or others.
	for _, cmd := range []string{"cta", "ict", "quant"} {
		msg := sessionExpiredMessage(cmd)
		if !strings.Contains(msg, "⏳") {
			t.Errorf("sessionExpiredMessage(%q) missing ⏳ emoji, got %q", cmd, msg)
		}
		if strings.Contains(msg, "⏰") {
			t.Errorf("sessionExpiredMessage(%q) uses non-canonical ⏰ emoji, got %q", cmd, msg)
		}
	}
}

func TestSessionExpiredMessage_IndonesianLanguage(t *testing.T) {
	msg := sessionExpiredMessage("cta")
	// Must contain Indonesian "Sesi berakhir" or equivalent.
	if !strings.Contains(msg, "Sesi berakhir") {
		t.Errorf("sessionExpiredMessage missing Indonesian 'Sesi berakhir', got %q", msg)
	}
}

func TestSessionExpiredMessage_CodeTagFormat(t *testing.T) {
	// Command must be wrapped in <code> HTML tags for Telegram formatting.
	msg := sessionExpiredMessage("quant")
	if !strings.Contains(msg, "<code>/quant</code>") {
		t.Errorf("sessionExpiredMessage missing <code>/quant</code>, got %q", msg)
	}
}

func TestSessionExpiredMessage_NotEmpty(t *testing.T) {
	msg := sessionExpiredMessage("")
	if msg == "" {
		t.Error("sessionExpiredMessage(\"\") returned empty string")
	}
}
