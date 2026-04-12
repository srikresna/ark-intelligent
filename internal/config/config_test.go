package config

import (
	"testing"
	"time"
)

// makeValidConfig returns a Config pre-populated with values that pass all
// validation checks. Tests should modify individual fields to trigger errors.
func makeValidConfig(t *testing.T) *Config {
	t.Helper()
	dir := t.TempDir()
	return &Config{
		BotToken:               "test-token",
		ChatID:                 "123456",
		DataDir:                dir,
		GeminiModel:            "gemini-test",
		COTFetchInterval:       6 * time.Hour,
		COTHistoryWeeks:        52,
		ConfluenceCalcInterval: 2 * time.Hour,
		PriceFetchInterval:     6 * time.Hour,
		PriceHistoryWeeks:      52,
		IntradayFetchInterval:  15 * time.Minute,
		IntradayRetentionDays:  60,
		AICacheTTL:             1 * time.Hour,
		AIMaxRPM:               15,
		AIMaxDaily:             200,
		ChatHistoryLimit:       50,
		ImpactBootstrapMonths:  12,
		ClaudeMaxTokens:        8192,
		ClaudeThinkingBudget:   0,
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := makeValidConfig(t)
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected no error for valid config, got: %v", err)
	}
}

func TestValidate_COTHistoryWeeks(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.COTHistoryWeeks = 3
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for COTHistoryWeeks < 4")
	}
}

func TestValidate_COTFetchInterval(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.COTFetchInterval = 30 * time.Second
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for COTFetchInterval < 1m")
	}
}

func TestValidate_ConfluenceCalcInterval(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.ConfluenceCalcInterval = 30 * time.Second
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for ConfluenceCalcInterval < 1m")
	}
}

func TestValidate_PriceFetchInterval(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.PriceFetchInterval = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for PriceFetchInterval <= 0")
	}
}

func TestValidate_PriceHistoryWeeks(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.PriceHistoryWeeks = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for PriceHistoryWeeks < 1")
	}
}

func TestValidate_IntradayFetchInterval(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.IntradayFetchInterval = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for IntradayFetchInterval <= 0")
	}
}

func TestValidate_IntradayRetentionDays(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.IntradayRetentionDays = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for IntradayRetentionDays < 1")
	}
}

func TestValidate_AICacheTTL(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.AICacheTTL = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for AICacheTTL <= 0")
	}
}

func TestValidate_AIMaxRPM(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.AIMaxRPM = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for AIMaxRPM <= 0")
	}
}

func TestValidate_AIMaxDaily(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.AIMaxDaily = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for AIMaxDaily <= 0")
	}
}

func TestValidate_ChatHistoryLimitDefault(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.ChatHistoryLimit = 0
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected no error for ChatHistoryLimit=0 (auto-default), got: %v", err)
	}
	if cfg.ChatHistoryLimit != 50 {
		t.Errorf("expected ChatHistoryLimit reset to 50, got %d", cfg.ChatHistoryLimit)
	}
}

func TestValidate_AIMaxDailyLessThanRPM(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.AIMaxDaily = 5
	cfg.AIMaxRPM = 10
	// Advisory warning only — should not return an error.
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected no error for AIMaxDaily < AIMaxRPM (warn only), got: %v", err)
	}
}

func TestValidate_ImpactBootstrapMonths(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.ImpactBootstrapMonths = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for ImpactBootstrapMonths < 1")
	}
}

func TestValidate_ClaudeModelRequired(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.ClaudeEndpoint = "http://example.com"
	cfg.ClaudeModel = ""
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error when ClaudeEndpoint set without ClaudeModel")
	}
}

func TestValidate_ClaudeMaxTokensRequired(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.ClaudeEndpoint = "http://example.com"
	cfg.ClaudeModel = "claude-test"
	cfg.ClaudeMaxTokens = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for ClaudeMaxTokens <= 0 when Claude enabled")
	}
}

func TestValidate_ClaudeThinkingBudgetNegative(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.ClaudeThinkingBudget = -1
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for ClaudeThinkingBudget < 0")
	}
}

func TestValidate_MassiveS3Mismatch(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.MassiveS3AccessKey = "access-key"
	cfg.MassiveS3SecretKey = ""
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for mismatched Massive S3 credentials")
	}
}

func TestValidate_DataDirNotWritable(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.DataDir = "/nonexistent/path/xyz123"
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for non-writable DataDir")
	}
}
