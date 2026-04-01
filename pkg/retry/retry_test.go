package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestDo_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	result, err := Do(context.Background(), func() (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDo_SuccessAfterRetry(t *testing.T) {
	calls := 0
	result, err := Do(context.Background(), func() (string, error) {
		calls++
		if calls < 3 {
			return "", fmt.Errorf("connection reset")
		}
		return "ok", nil
	}, WithBaseDelay(10*time.Millisecond), WithMaxAttempts(3))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDo_AllAttemptsFail(t *testing.T) {
	calls := 0
	_, err := Do(context.Background(), func() (string, error) {
		calls++
		return "", fmt.Errorf("HTTP 502: bad gateway")
	}, WithBaseDelay(10*time.Millisecond), WithMaxAttempts(3))

	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDo_NonRetryableError(t *testing.T) {
	calls := 0
	_, err := Do(context.Background(), func() (string, error) {
		calls++
		return "", fmt.Errorf("HTTP 404: not found")
	}, WithBaseDelay(10*time.Millisecond))

	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry for 404), got %d", calls)
	}
}

func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := Do(ctx, func() (string, error) {
		calls++
		return "", fmt.Errorf("connection reset")
	}, WithBaseDelay(100*time.Millisecond), WithMaxAttempts(10))

	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestDo_RateLimitRetried(t *testing.T) {
	calls := 0
	result, err := Do(context.Background(), func() (string, error) {
		calls++
		if calls == 1 {
			return "", fmt.Errorf("coingecko: rate limited (429)")
		}
		return "data", nil
	}, WithBaseDelay(10*time.Millisecond))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "data" {
		t.Fatalf("expected 'data', got %q", result)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{context.Canceled, false},
		{context.DeadlineExceeded, false},
		{errors.New("HTTP 400: bad request"), false},
		{errors.New("HTTP 401: unauthorized"), false},
		{errors.New("HTTP 403: forbidden"), false},
		{errors.New("HTTP 404: not found"), false},
		{errors.New("HTTP 429: too many requests"), true},
		{errors.New("rate limited (429)"), true},
		{errors.New("HTTP 500: internal server error"), true},
		{errors.New("HTTP 502: bad gateway"), true},
		{errors.New("HTTP 503: service unavailable"), true},
		{errors.New("connection refused"), true},
		{errors.New("connection reset by peer"), true},
		{errors.New("EOF"), true},
		{errors.New("TLS handshake timeout"), true},
		{errors.New("unknown custom error"), false},
	}

	for _, tt := range tests {
		got := isRetryable(tt.err)
		if got != tt.want {
			t.Errorf("isRetryable(%q) = %v, want %v", tt.err, got, tt.want)
		}
	}
}
