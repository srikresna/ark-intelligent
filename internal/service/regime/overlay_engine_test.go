package regime

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Tests for types.go helpers
// ---------------------------------------------------------------------------

func TestFormatScore(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{67.3, "+67"},
		{-34.7, "-35"},
		{0, "+0"},
		{100, "+100"},
		{-100, "-100"},
		{0.4, "+0"},
	}
	for _, tt := range tests {
		got := formatScore(tt.score)
		if got != tt.want {
			t.Errorf("formatScore(%v) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestHeaderLine(t *testing.T) {
	r := &RegimeOverlay{
		UnifiedScore: 67,
		OverlayColor: "🟢",
		Label:        "BULLISH",
		Description:  "Trending↑, Low Vol, COT Long",
	}
	line := r.HeaderLine()
	if line == "" {
		t.Error("HeaderLine() returned empty string")
	}
	// Should contain the score
	if len(line) < 10 {
		t.Errorf("HeaderLine() too short: %q", line)
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{
		0:    "0",
		1:    "1",
		-1:   "-1",
		100:  "100",
		-100: "-100",
		42:   "42",
	}
	for input, want := range cases {
		got := itoa(input)
		if got != want {
			t.Errorf("itoa(%d) = %q, want %q", input, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests for overlay_engine.go helpers
// ---------------------------------------------------------------------------

func TestClamp(t *testing.T) {
	if clamp(150, -100, 100) != 100 {
		t.Error("clamp should cap at max")
	}
	if clamp(-150, -100, 100) != -100 {
		t.Error("clamp should cap at min")
	}
	if clamp(50, -100, 100) != 50 {
		t.Error("clamp should pass through in-range values")
	}
}

func TestScoreToColor(t *testing.T) {
	if scoreToColor(60) != "🟢" {
		t.Error("high positive score should be green")
	}
	if scoreToColor(-60) != "🔴" {
		t.Error("high negative score should be red")
	}
	if scoreToColor(0) != "🟡" {
		t.Error("neutral score should be yellow")
	}
	if scoreToColor(29) != "🟡" {
		t.Error("score just below 30 should be yellow")
	}
	if scoreToColor(30) != "🟢" {
		t.Error("score at 30 should be green")
	}
	if scoreToColor(-30) != "🔴" {
		t.Error("score at -30 should be red")
	}
}

func TestScoreToLabel(t *testing.T) {
	tests := []struct {
		score    float64
		hmmState string
		want     string
	}{
		{70, "RISK_ON", "BULLISH"},
		{40, "RISK_ON", "MILDLY BULLISH"},
		{0, "RISK_OFF", "NEUTRAL"},
		{-40, "RISK_OFF", "MILDLY BEARISH"},
		{-70, "RISK_OFF", "BEARISH"},
		{-80, "CRISIS", "CRISIS"}, // HMM crisis overrides score
	}
	for _, tt := range tests {
		got := scoreToLabel(tt.score, tt.hmmState)
		if got != tt.want {
			t.Errorf("scoreToLabel(%v, %q) = %q, want %q", tt.score, tt.hmmState, got, tt.want)
		}
	}
}

func TestBuildDescription(t *testing.T) {
	o := &RegimeOverlay{
		ADXStrength:    "STRONG",
		ADXScore:       70,
		GARCHVolRegime: "CONTRACTING",
		COTBias:        "BULLISH",
	}
	desc := buildDescription(o)
	if desc == "" {
		t.Error("buildDescription returned empty")
	}
	// Should mention trend and vol
	if !contains(desc, "Trending") {
		t.Errorf("expected 'Trending' in description, got: %q", desc)
	}
	if !contains(desc, "Low Vol") {
		t.Errorf("expected 'Low Vol' in description, got: %q", desc)
	}
	if !contains(desc, "COT Long") {
		t.Errorf("expected 'COT Long' in description, got: %q", desc)
	}
}

func TestDailyToPriceRecords(t *testing.T) {
	now := time.Now()
	daily := []domain.DailyPrice{
		{ContractCode: "099741", Symbol: "EUR/USD", Date: now, Close: 1.08},
		{ContractCode: "099741", Symbol: "EUR/USD", Date: now.AddDate(0, 0, -1), Close: 1.07},
	}
	records := dailyToPriceRecords(daily)
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
	if records[0].Close != 1.08 {
		t.Errorf("expected Close=1.08, got %v", records[0].Close)
	}
	if records[0].ContractCode != "099741" {
		t.Error("ContractCode not preserved")
	}
}

func TestDailyToOHLCV(t *testing.T) {
	daily := []domain.DailyPrice{
		{Open: 1.0, High: 1.1, Low: 0.9, Close: 1.05, Volume: 100},
	}
	bars := dailyToOHLCV(daily)
	if len(bars) != 1 {
		t.Errorf("expected 1 bar, got %d", len(bars))
	}
	if bars[0].High != 1.1 {
		t.Errorf("High not preserved: got %v", bars[0].High)
	}
}

func TestMaxFloat64(t *testing.T) {
	if maxFloat64(0.1, 0.9, 0.5) != 0.9 {
		t.Error("maxFloat64 failed")
	}
	if maxFloat64(0.0) != 0.0 {
		t.Error("maxFloat64 single value failed")
	}
	if maxFloat64() != 0.0 {
		t.Error("maxFloat64 no values should return 0")
	}
}

// ---------------------------------------------------------------------------
// OverlayEngine integration test with stub repos
// ---------------------------------------------------------------------------

// stubDailyStore returns a pre-built set of fake daily prices.
type stubDailyStore struct {
	prices []domain.DailyPrice
}

func (s *stubDailyStore) GetDailyHistory(_ context.Context, _ string, _ int) ([]domain.DailyPrice, error) {
	return s.prices, nil
}

// buildFakePrices creates n daily prices with a mild uptrend.
func buildFakePrices(n int) []domain.DailyPrice {
	prices := make([]domain.DailyPrice, n)
	now := time.Now()
	base := 1.1000
	for i := 0; i < n; i++ {
		close_ := base + float64(n-i)*0.0001 + 0.0002*math.Sin(float64(i)*0.3)
		prices[i] = domain.DailyPrice{
			ContractCode: "099741",
			Symbol:       "EUR/USD",
			Date:         now.AddDate(0, 0, -i),
			Open:         close_ - 0.0005,
			High:         close_ + 0.0010,
			Low:          close_ - 0.0010,
			Close:        close_,
			Volume:       1000,
		}
	}
	return prices
}

func TestComputeOverlay_NoCOT(t *testing.T) {
	store := &stubDailyStore{prices: buildFakePrices(120)}
	engine := NewOverlayEngine(store, nil) // no COT repo

	ctx := context.Background()
	result, err := engine.ComputeOverlay(ctx, "099741", "EUR/USD", "1d")
	if err != nil {
		t.Fatalf("ComputeOverlay failed: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Symbol != "EUR/USD" {
		t.Errorf("Symbol mismatch: %q", result.Symbol)
	}
	if result.UnifiedScore < -100 || result.UnifiedScore > 100 {
		t.Errorf("UnifiedScore out of range: %v", result.UnifiedScore)
	}
	if result.OverlayColor == "" {
		t.Error("OverlayColor is empty")
	}
	if result.Label == "" {
		t.Error("Label is empty")
	}
	if len(result.ModelsUsed) == 0 {
		t.Error("ModelsUsed is empty — at least one model should succeed")
	}
	// COT should not be in models used
	for _, m := range result.ModelsUsed {
		if m == "cot" {
			t.Error("COT should not be used when cotRepo is nil")
		}
	}
	t.Logf("Overlay: %s  Score=%.1f  Models=%v", result.Label, result.UnifiedScore, result.ModelsUsed)
}

func TestComputeOverlay_Cache(t *testing.T) {
	store := &stubDailyStore{prices: buildFakePrices(120)}
	engine := NewOverlayEngine(store, nil)

	ctx := context.Background()
	r1, err := engine.ComputeOverlay(ctx, "099741", "EUR/USD", "1d")
	if err != nil {
		t.Fatalf("first ComputeOverlay failed: %v", err)
	}

	r2, err := engine.ComputeOverlay(ctx, "099741", "EUR/USD", "1d")
	if err != nil {
		t.Fatalf("second ComputeOverlay failed: %v", err)
	}

	if r1 != r2 {
		t.Error("expected cached result (same pointer) on second call")
	}
}

func TestComputeOverlay_InsufficientData(t *testing.T) {
	store := &stubDailyStore{prices: buildFakePrices(5)} // too few
	engine := NewOverlayEngine(store, nil)

	ctx := context.Background()
	_, err := engine.ComputeOverlay(ctx, "099741", "EUR/USD", "1d")
	if err == nil {
		t.Error("expected error for insufficient data")
	}
}

func TestCacheTTL(t *testing.T) {
	if cacheTTL("1h") != time.Hour {
		t.Error("1h TTL should be 1 hour")
	}
	if cacheTTL("4h") != time.Hour {
		t.Error("4h TTL should be 1 hour")
	}
	if cacheTTL("1d") != 4*time.Hour {
		t.Error("daily TTL should be 4 hours")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(substr) == 0 ||
		findSubstr(s, substr))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
