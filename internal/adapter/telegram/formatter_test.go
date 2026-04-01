package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
)

// ---------------------------------------------------------------------------
// Group D — Helper functions (quick wins, pure functions)
// ---------------------------------------------------------------------------

func TestDirectionArrow_BothEmpty(t *testing.T) {
	if got := directionArrow("", ""); got != "⚪" {
		t.Errorf("expected ⚪ for empty inputs, got %q", got)
	}
}

func TestDirectionArrow_ActualEmpty(t *testing.T) {
	if got := directionArrow("", "1.5%"); got != "⚪" {
		t.Errorf("expected ⚪ for empty actual, got %q", got)
	}
}

func TestDirectionArrow_BeatsForecast(t *testing.T) {
	got := directionArrow("3.5%", "3.0%")
	if got != "🟢" {
		t.Errorf("expected 🟢 when actual > forecast, got %q", got)
	}
}

func TestDirectionArrow_MissesForecast(t *testing.T) {
	got := directionArrow("2.5%", "3.0%")
	if got != "🔴" {
		t.Errorf("expected 🔴 when actual < forecast, got %q", got)
	}
}

func TestDirectionArrow_ExactMatch(t *testing.T) {
	got := directionArrow("3.0%", "3.0%")
	if got != "⚪" {
		t.Errorf("expected ⚪ when actual == forecast, got %q", got)
	}
}

func TestDirectionArrow_InvertedIndicator(t *testing.T) {
	// impactDirection=2: higher actual = bearish (e.g. unemployment claims)
	// actual 250K > forecast 230K → bad for currency → 🔴
	got := directionArrow("250K", "230K", 2)
	if got != "🔴" {
		t.Errorf("expected 🔴 for inverted indicator beating forecast, got %q", got)
	}
}

func TestDirectionArrow_NormalIndicator(t *testing.T) {
	// impactDirection=1: higher actual = bullish (e.g. NFP)
	got := directionArrow("250K", "230K", 1)
	if got != "🟢" {
		t.Errorf("expected 🟢 for normal indicator beating forecast, got %q", got)
	}
}

func TestDirectionArrow_NonNumeric(t *testing.T) {
	got := directionArrow("N/A", "3.0%")
	if got != "⚪" {
		t.Errorf("expected ⚪ for non-numeric actual, got %q", got)
	}
}

func TestParseNumeric_Percentage(t *testing.T) {
	got := parseNumeric("3.5%")
	if got == nil || *got != 3.5 {
		t.Errorf("expected 3.5, got %v", got)
	}
}

func TestParseNumeric_WithComma(t *testing.T) {
	got := parseNumeric("1,250")
	if got == nil || *got != 1250 {
		t.Errorf("expected 1250, got %v", got)
	}
}

func TestParseNumeric_WithSuffix(t *testing.T) {
	got := parseNumeric("250K")
	if got == nil || *got != 250 {
		t.Errorf("expected 250, got %v", got)
	}
}

func TestParseNumeric_Empty(t *testing.T) {
	got := parseNumeric("")
	if got != nil {
		t.Errorf("expected nil for empty input, got %v", *got)
	}
}

func TestParseNumeric_NonNumeric(t *testing.T) {
	got := parseNumeric("N/A")
	if got != nil {
		t.Errorf("expected nil for non-numeric, got %v", *got)
	}
}

func TestCotIdxLabel_Boundaries(t *testing.T) {
	tests := []struct {
		idx  float64
		want string
	}{
		{100, "X.Long"},
		{80, "X.Long"},
		{79.9, "Bullish"},
		{60, "Bullish"},
		{59.9, "Neutral"},
		{40, "Neutral"},
		{39.9, "Bearish"},
		{20, "Bearish"},
		{19.9, "X.Short"},
		{0, "X.Short"},
	}
	for _, tt := range tests {
		if got := cotIdxLabel(tt.idx); got != tt.want {
			t.Errorf("cotIdxLabel(%.1f) = %q, want %q", tt.idx, got, tt.want)
		}
	}
}

func TestCotLabel_Boundaries(t *testing.T) {
	tests := []struct {
		idx  float64
		want string
	}{
		{100, "Extreme Long"},
		{80, "Extreme Long"},
		{79, "Bullish"},
		{60, "Bullish"},
		{59, "Neutral"},
		{40, "Neutral"},
		{39, "Bearish"},
		{20, "Bearish"},
		{19, "Extreme Short"},
		{0, "Extreme Short"},
	}
	for _, tt := range tests {
		if got := cotLabel(tt.idx); got != tt.want {
			t.Errorf("cotLabel(%.0f) = %q, want %q", tt.idx, got, tt.want)
		}
	}
}

func TestConvictionMiniBar_HighLong(t *testing.T) {
	got := convictionMiniBar(80, "LONG")
	if !strings.Contains(got, "🟢") {
		t.Errorf("expected 🟢 for high LONG conviction, got %q", got)
	}
	if !strings.Contains(got, "80") {
		t.Errorf("expected score 80 in output, got %q", got)
	}
	if !strings.Contains(got, "▓▓▓▓") {
		t.Errorf("expected filled blocks for 80 score, got %q", got)
	}
}

func TestConvictionMiniBar_HighShort(t *testing.T) {
	got := convictionMiniBar(70, "SHORT")
	if !strings.Contains(got, "🔴") {
		t.Errorf("expected 🔴 for high SHORT conviction, got %q", got)
	}
}

func TestConvictionMiniBar_Medium(t *testing.T) {
	got := convictionMiniBar(57, "LONG")
	if !strings.Contains(got, "🟡") {
		t.Errorf("expected 🟡 for medium conviction, got %q", got)
	}
}

func TestConvictionMiniBar_Low(t *testing.T) {
	got := convictionMiniBar(30, "NEUTRAL")
	if !strings.Contains(got, "⚪") {
		t.Errorf("expected ⚪ for low conviction, got %q", got)
	}
}

func TestConvictionMiniBar_Zero(t *testing.T) {
	got := convictionMiniBar(0, "NEUTRAL")
	if !strings.Contains(got, "░░░░░") {
		t.Errorf("expected all empty blocks for 0, got %q", got)
	}
}

func TestConvictionMiniBar_MaxFilled(t *testing.T) {
	got := convictionMiniBar(100, "LONG")
	if !strings.Contains(got, "▓▓▓▓▓") {
		t.Errorf("expected 5 filled blocks for 100, got %q", got)
	}
}

func TestScoreArrow(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{80, "↑↑"},
		{61, "↑↑"},
		{50, "↑"},
		{31, "↑"},
		{0, "→"},
		{-29, "→"},
		{-50, "↓"},
		{-59, "↓"},
		{-70, "↓↓↓"},
		{-100, "↓↓↓"},
	}
	for _, tt := range tests {
		if got := scoreArrow(tt.score); got != tt.want {
			t.Errorf("scoreArrow(%.0f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestScoreDot(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{50, "🟢"},
		{16, "🟢"},
		{15, "⚪"},
		{0, "⚪"},
		{-15, "⚪"},
		{-16, "🔴"},
		{-50, "🔴"},
	}
	for _, tt := range tests {
		if got := scoreDot(tt.score); got != tt.want {
			t.Errorf("scoreDot(%.0f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Group C — Sentiment helpers
// ---------------------------------------------------------------------------

func TestFearGreedEmoji_Boundaries(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0, "😱"},
		{25, "😱"},
		{26, "😟"},
		{45, "😟"},
		{46, "😐"},
		{55, "😐"},
		{56, "😏"},
		{75, "😏"},
		{76, "🤑"},
		{100, "🤑"},
	}
	for _, tt := range tests {
		if got := fearGreedEmoji(tt.score); got != tt.want {
			t.Errorf("fearGreedEmoji(%.0f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestSentimentGauge_ExtremeFear(t *testing.T) {
	got := sentimentGauge(0, 20)
	if !strings.HasPrefix(got, "Fear ") {
		t.Errorf("expected 'Fear ' prefix, got %q", got)
	}
	if !strings.HasSuffix(got, " Greed") {
		t.Errorf("expected ' Greed' suffix, got %q", got)
	}
	// Position indicator should be at position 0
	bar := strings.TrimPrefix(got, "Fear ")
	bar = strings.TrimSuffix(bar, " Greed")
	if bar[0] != '|' {
		t.Errorf("expected indicator at position 0 for score=0, got bar=%q", bar)
	}
}

func TestSentimentGauge_Middle(t *testing.T) {
	got := sentimentGauge(50, 20)
	bar := strings.TrimPrefix(got, "Fear ")
	bar = strings.TrimSuffix(bar, " Greed")
	if len(bar) != 20 {
		t.Errorf("expected bar width 20, got %d", len(bar))
	}
	// Indicator should be roughly in the middle
	idx := strings.Index(bar, "|")
	if idx < 8 || idx > 12 {
		t.Errorf("expected indicator near middle for score=50, got position %d", idx)
	}
}

func TestSentimentGauge_ExtremeGreed(t *testing.T) {
	got := sentimentGauge(100, 20)
	bar := strings.TrimPrefix(got, "Fear ")
	bar = strings.TrimSuffix(bar, " Greed")
	// Clamped to width-1 = 19
	if bar[19] != '|' {
		t.Errorf("expected indicator at last position for score=100, got bar=%q", bar)
	}
}

func TestSentimentBar_Full(t *testing.T) {
	got := sentimentBar(100, "🟢")
	// Should have 10 green circles
	count := strings.Count(got, "🟢")
	if count != 10 {
		t.Errorf("expected 10 emojis for 100%%, got %d", count)
	}
}

func TestSentimentBar_Half(t *testing.T) {
	got := sentimentBar(50, "🔴")
	count := strings.Count(got, "🔴")
	if count != 5 {
		t.Errorf("expected 5 emojis for 50%%, got %d", count)
	}
}

func TestSentimentBar_Zero(t *testing.T) {
	got := sentimentBar(0, "🟢")
	if got != "" {
		t.Errorf("expected empty string for 0%%, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Group D continued — more helpers
// ---------------------------------------------------------------------------

func TestShortDirection(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"BULLISH", "🟢 BULL"},
		{"BEARISH", "🔴 BEAR"},
		{"NEUTRAL", "NEUTRAL"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := shortDirection(tt.input); got != tt.want {
			t.Errorf("shortDirection(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResultBadge(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{domain.OutcomeWin, "✅"},
		{domain.OutcomeLoss, "❌"},
		{"PENDING", "⏳"},
		{"", "⏳"},
	}
	for _, tt := range tests {
		if got := resultBadge(tt.input); got != tt.want {
			t.Errorf("resultBadge(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncateStr_Short(t *testing.T) {
	got := truncateStr("hello", 10)
	if got != "hello" {
		t.Errorf("expected no truncation, got %q", got)
	}
}

func TestTruncateStr_ExactLength(t *testing.T) {
	got := truncateStr("hello", 5)
	if got != "hello" {
		t.Errorf("expected no truncation at exact length, got %q", got)
	}
}

func TestTruncateStr_Truncated(t *testing.T) {
	got := truncateStr("hello world", 7)
	if got != "hello.." {
		t.Errorf("expected 'hello..', got %q", got)
	}
}

func TestContractCodeToFriendly(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"099741", "EUR"},
		{"096742", "GBP"},
		{"097741", "JPY"},
		{"088691", "GOLD"},
		{"067651", "OIL"},
		{"UNKNOWN", "UNKNOWN"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := contractCodeToFriendly(tt.code); got != tt.want {
			t.Errorf("contractCodeToFriendly(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestPairDirection(t *testing.T) {
	tests := []struct {
		pair, favored, want string
	}{
		{"EURUSD", "EUR", "LONG"},
		{"EURUSD", "USD", "SHORT"},
		{"USDJPY", "USD", "LONG"},
		{"USDJPY", "JPY", "SHORT"},
	}
	for _, tt := range tests {
		if got := pairDirection(tt.pair, tt.favored); got != tt.want {
			t.Errorf("pairDirection(%q, %q) = %q, want %q", tt.pair, tt.favored, got, tt.want)
		}
	}
}

func TestFormatPairName(t *testing.T) {
	tests := []struct {
		long, short, want string
	}{
		{"EUR", "USD", "EURUSD"},
		{"GBP", "USD", "GBPUSD"},
		{"AUD", "USD", "AUDUSD"},
		{"USD", "JPY", "USDJPY"},
		{"USD", "CHF", "USDCHF"},
		{"USD", "CAD", "USDCAD"},
		{"USD", "EUR", "EURUSD"},   // reversed — USD quote
		{"EUR", "GBP", "EURGBP"},   // cross pair
		{"AUD", "NZD", "AUDNZD"},   // cross pair
	}
	for _, tt := range tests {
		if got := formatPairName(tt.long, tt.short); got != tt.want {
			t.Errorf("formatPairName(%q, %q) = %q, want %q", tt.long, tt.short, got, tt.want)
		}
	}
}

func TestCommercialSignalLabel(t *testing.T) {
	tests := []struct {
		signal, rt, want string
	}{
		{"BULLISH", "TFF", "BULLISH (dealer)"},
		{"BEARISH", "DISAGGREGATED", "BEARISH (contrarian)"},
		{"NEUTRAL", "", "NEUTRAL"},
	}
	for _, tt := range tests {
		if got := commercialSignalLabel(tt.signal, tt.rt); got != tt.want {
			t.Errorf("commercialSignalLabel(%q, %q) = %q, want %q", tt.signal, tt.rt, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Group A — COT Formatting (exported methods)
// ---------------------------------------------------------------------------

func TestFormatCOTOverview_NilInput(t *testing.T) {
	f := NewFormatter()
	out := f.FormatCOTOverview(nil, nil)
	// Should not panic, should return something non-empty (at least header)
	if out == "" {
		t.Error("expected non-empty output for nil input")
	}
}

func TestFormatCOTOverview_EmptySlice(t *testing.T) {
	f := NewFormatter()
	out := f.FormatCOTOverview([]domain.COTAnalysis{}, []cot.ConvictionScore{})
	if out == "" {
		t.Error("expected non-empty output for empty input")
	}
}

func TestFormatCOTOverview_SingleCurrency(t *testing.T) {
	f := NewFormatter()
	analyses := []domain.COTAnalysis{
		{
			Contract:    domain.COTContract{Code: "099741", Name: "EURO FX", Symbol: "6E", Currency: "EUR"},
			ReportDate:  time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
			COTIndex:    72.5,
			NetPosition: 45000,
			NetChange:   5000,
		},
	}
	out := f.FormatCOTOverview(analyses, nil)
	if !strings.Contains(out, "EUR") {
		t.Errorf("expected output to contain EUR, got %q", out)
	}
}

func TestFormatCOTDetail_NoCrash(t *testing.T) {
	f := NewFormatter()
	a := domain.COTAnalysis{
		Contract:    domain.COTContract{Code: "099741", Name: "EURO FX", Symbol: "6E", Currency: "EUR", ReportType: "TFF"},
		ReportDate:  time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
		COTIndex:    85.0,
		NetPosition: 120000,
		NetChange:   8000,
	}
	out := f.FormatCOTDetail(a)
	if !strings.Contains(out, "EUR") {
		t.Errorf("expected EUR in detail output, got %q", out)
	}
	if !strings.Contains(out, "85") {
		t.Errorf("expected COT index 85 in output, got %q", out)
	}
}

func TestFormatCOTDetailWithCode(t *testing.T) {
	f := NewFormatter()
	a := domain.COTAnalysis{
		Contract:    domain.COTContract{Code: "099741", Name: "EURO FX", Symbol: "6E", Currency: "EUR", ReportType: "TFF"},
		ReportDate:  time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
		COTIndex:    50.0,
		NetPosition: 0,
	}
	out := f.FormatCOTDetailWithCode(a, "EUR")
	if !strings.Contains(out, "EUR") {
		t.Errorf("expected EUR display code in output, got %q", out)
	}
}

// ---------------------------------------------------------------------------
// Group A — Ranking
// ---------------------------------------------------------------------------

func TestFormatRanking_NilInput(t *testing.T) {
	f := NewFormatter()
	out := f.FormatRanking(nil, time.Now())
	if out == "" {
		t.Error("expected non-empty output for nil ranking input")
	}
}

func TestFormatRanking_SingleEntry(t *testing.T) {
	f := NewFormatter()
	analyses := []domain.COTAnalysis{
		{
			Contract: domain.COTContract{Code: "099741", Currency: "EUR"},
			COTIndex: 90.0,
		},
	}
	out := f.FormatRanking(analyses, time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(out, "EUR") {
		t.Errorf("expected EUR in ranking, got %q", out)
	}
}

// ---------------------------------------------------------------------------
// Group B — Macro/FRED (nil/empty safety)
// ---------------------------------------------------------------------------

func TestFormatProgressBar_Extremes(t *testing.T) {
	f := NewFormatter()
	tests := []struct {
		pct       float64
		wantLabel string
	}{
		{90, "EXTREME LONG"},
		{80, "EXTREME LONG"},
		{10, "EXTREME SHORT"},
		{20, "EXTREME SHORT"},
		{50, ""},
	}
	for _, tt := range tests {
		got := f.formatProgressBar(tt.pct, 10)
		if tt.wantLabel != "" && !strings.Contains(got, tt.wantLabel) {
			t.Errorf("formatProgressBar(%.0f) missing label %q, got %q", tt.pct, tt.wantLabel, got)
		}
		if tt.wantLabel == "" && (strings.Contains(got, "EXTREME LONG") || strings.Contains(got, "EXTREME SHORT")) {
			t.Errorf("formatProgressBar(%.0f) should not have extreme label, got %q", tt.pct, got)
		}
	}
}

func TestFormatProgressBar_NegativeAndOver100(t *testing.T) {
	f := NewFormatter()
	// Should not panic
	_ = f.formatProgressBar(-10, 10)
	_ = f.formatProgressBar(110, 10)
}

func TestMomentumLabel(t *testing.T) {
	f := NewFormatter()
	tests := []struct {
		input domain.MomentumDirection
		want  string
	}{
		{"STRONG_UP", "Strong Bullish"},
		{"UP", "Bullish"},
		{"FLAT", "Neutral"},
		{"DOWN", "Bearish"},
		{"STRONG_DOWN", "Strong Bearish"},
		{"UNKNOWN", "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := f.momentumLabel(tt.input); got != tt.want {
			t.Errorf("momentumLabel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Group B — Formatted output safety
// ---------------------------------------------------------------------------

func TestFormatSettings_NoCrash(t *testing.T) {
	f := NewFormatter()
	// Default zero-value prefs should not crash
	out := f.FormatSettings(domain.UserPrefs{})
	if out == "" {
		t.Error("expected non-empty settings output")
	}
}

func TestFormatCalendarDay_Empty(t *testing.T) {
	f := NewFormatter()
	out := f.FormatCalendarDay("2026-04-01", nil, "")
	// Should handle nil events gracefully
	if out == "" {
		t.Error("expected non-empty output for empty calendar day")
	}
}

func TestFormatCalendarDay_WithFilter(t *testing.T) {
	f := NewFormatter()
	events := []domain.NewsEvent{
		{
			Event:    "Non-Farm Payrolls",
			Currency: "USD",
			Impact:   "High",
			Time:     "13:30",
		},
		{
			Event:    "CPI YoY",
			Currency: "EUR",
			Impact:   "High",
			Time:     "10:00",
		},
	}
	out := f.FormatCalendarDay("2026-04-01", events, "USD")
	if !strings.Contains(out, "Non-Farm") {
		t.Error("expected USD event in filtered output")
	}
}

// ---------------------------------------------------------------------------
// Additional safety tests — ensure no panics on nil/zero inputs
// ---------------------------------------------------------------------------

func TestFormatCOTShareText_NoCrash(t *testing.T) {
	f := NewFormatter()
	a := domain.COTAnalysis{
		Contract: domain.COTContract{Code: "099741", Currency: "EUR", Name: "EURO FX"},
	}
	out := f.FormatCOTShareText(a)
	if !strings.Contains(out, "EUR") {
		t.Errorf("expected EUR in share text, got %q", out)
	}
}

func TestFormatOutlookShareText_NoCrash(t *testing.T) {
	f := NewFormatter()
	out := f.FormatOutlookShareText("<b>Test outlook</b>")
	if out == "" {
		t.Error("expected non-empty outlook share text")
	}
}

func TestFormatTrackedEvents_Empty(t *testing.T) {
	f := NewFormatter()
	out := f.FormatTrackedEvents(nil)
	// Should not panic
	_ = out
}

func TestFormatTrackedEvents_WithEvents(t *testing.T) {
	f := NewFormatter()
	events := []string{"NFP", "CPI", "FOMC"}
	out := f.FormatTrackedEvents(events)
	if !strings.Contains(out, "NFP") {
		t.Errorf("expected NFP in tracked events, got %q", out)
	}
}

func TestSignalConfluenceInterpretation(t *testing.T) {
	tests := []struct {
		spec, comm, rt string
		wantContains   string
	}{
		{"BULLISH", "BULLISH", "DISAGGREGATED", ""},  // aligned
		{"BULLISH", "BEARISH", "TFF", ""},              // divergent
		{"STRONG_BULLISH", "STRONG_BULLISH", "TFF", ""}, // strong alignment
	}
	for _, tt := range tests {
		got := signalConfluenceInterpretation(tt.spec, tt.comm, tt.rt)
		// Just ensure no panic and non-empty output
		if got == "" {
			t.Errorf("signalConfluenceInterpretation(%q, %q, %q) returned empty", tt.spec, tt.comm, tt.rt)
		}
	}
}
