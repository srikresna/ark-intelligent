package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/internal/service/sentiment"
)

// ---------------------------------------------------------------------------
// Group D — Helper functions (pure, no I/O)
// ---------------------------------------------------------------------------

func TestDirectionArrow_EmptyInputs(t *testing.T) {
	if got := directionArrow("", ""); got != "⚪" {
		t.Errorf("empty actual+forecast: want ⚪, got %q", got)
	}
	if got := directionArrow("100", ""); got != "⚪" {
		t.Errorf("empty forecast: want ⚪, got %q", got)
	}
	if got := directionArrow("", "100"); got != "⚪" {
		t.Errorf("empty actual: want ⚪, got %q", got)
	}
}

func TestDirectionArrow_NeutralIndicator(t *testing.T) {
	// higher actual → bullish (green)
	if got := directionArrow("200", "150"); got != "🟢" {
		t.Errorf("actual > forecast default: want 🟢, got %q", got)
	}
	// lower actual → bearish (red)
	if got := directionArrow("100", "150"); got != "🔴" {
		t.Errorf("actual < forecast default: want 🔴, got %q", got)
	}
	// equal → neutral
	if got := directionArrow("150", "150"); got != "⚪" {
		t.Errorf("actual == forecast: want ⚪, got %q", got)
	}
}

func TestDirectionArrow_ImpactDirection(t *testing.T) {
	// dir=1 (higher = bullish for currency): actual > forecast → green
	if got := directionArrow("200", "100", 1); got != "🟢" {
		t.Errorf("dir=1 bullish: want 🟢, got %q", got)
	}
	// dir=2 (higher = bearish, e.g. unemployment): actual > forecast → red
	if got := directionArrow("200", "100", 2); got != "🔴" {
		t.Errorf("dir=2 inverted: want 🔴, got %q", got)
	}
	// dir=2 but actual < forecast → green (unemployment fell = good)
	if got := directionArrow("100", "200", 2); got != "🟢" {
		t.Errorf("dir=2 inverted lower: want 🟢, got %q", got)
	}
}

func TestCotIdxLabel_Boundaries(t *testing.T) {
	tests := []struct {
		idx  float64
		want string
	}{
		{0, "X.Short"},
		{19.9, "X.Short"},
		{20, "Bearish"},
		{39.9, "Bearish"},
		{40, "Neutral"},
		{59.9, "Neutral"},
		{60, "Bullish"},
		{79.9, "Bullish"},
		{80, "X.Long"},
		{100, "X.Long"},
	}
	for _, tc := range tests {
		if got := cotIdxLabel(tc.idx); got != tc.want {
			t.Errorf("cotIdxLabel(%.1f) = %q, want %q", tc.idx, got, tc.want)
		}
	}
}

func TestConvictionMiniBar_Rendering(t *testing.T) {
	tests := []struct {
		score       float64
		dir         string
		wantIcon    string
		wantContain string
	}{
		{0, "LONG", "⚪", "░░░░░"},
		{20, "LONG", "⚪", "▓"},
		{65, "LONG", "🟢", "▓▓▓"},
		{65, "SHORT", "🔴", "▓▓▓"},
		{55, "NEUTRAL", "🟡", "▓▓"},
		{100, "LONG", "🟢", "▓▓▓▓▓"},
	}
	for _, tc := range tests {
		got := convictionMiniBar(tc.score, tc.dir)
		if !strings.Contains(got, tc.wantIcon) {
			t.Errorf("convictionMiniBar(%.0f,%s): want icon %q in %q", tc.score, tc.dir, tc.wantIcon, got)
		}
		if !strings.Contains(got, tc.wantContain) {
			t.Errorf("convictionMiniBar(%.0f,%s): want bar segment %q in %q", tc.score, tc.dir, tc.wantContain, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Group C — Sentiment helpers
// ---------------------------------------------------------------------------

func TestSentimentGauge_Boundaries(t *testing.T) {
	tests := []struct {
		score float64
		width int
	}{
		{0, 15},
		{50, 15},
		{100, 15},
		{0, 10},
		{100, 10},
	}
	for _, tc := range tests {
		got := sentimentGauge(tc.score, tc.width)
		if !strings.HasPrefix(got, "Fear ") {
			t.Errorf("sentimentGauge(%.0f,%d): should start with 'Fear ': %q", tc.score, tc.width, got)
		}
		if !strings.HasSuffix(got, " Greed") {
			t.Errorf("sentimentGauge(%.0f,%d): should end with ' Greed': %q", tc.score, tc.width, got)
		}
		// should contain exactly one '|' marker
		if count := strings.Count(got, "|"); count != 1 {
			t.Errorf("sentimentGauge(%.0f,%d): want 1 '|', got %d in %q", tc.score, tc.width, count, got)
		}
	}
}

func TestFearGreedEmoji_Boundaries(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0, "😱"},   // extreme fear
		{25, "😱"},  // boundary extreme fear
		{26, "😟"},  // fear
		{45, "😟"},  // boundary fear
		{50, "😐"},  // neutral
		{55, "😐"},  // boundary neutral
		{60, "😏"},  // greed
		{75, "😏"},  // boundary greed
		{76, "🤑"},  // extreme greed
		{100, "🤑"}, // max extreme greed
	}
	for _, tc := range tests {
		if got := fearGreedEmoji(tc.score); got != tc.want {
			t.Errorf("fearGreedEmoji(%.0f) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Group A — COT formatting
// ---------------------------------------------------------------------------

func TestFormatCOTOverview_NilInput(t *testing.T) {
	f := NewFormatter()
	got := f.FormatCOTOverview(nil, nil)
	if !strings.Contains(got, "COT") {
		t.Errorf("FormatCOTOverview(nil,nil): want 'COT' in output, got: %q", got)
	}
	// should not crash and should contain base header
	if !strings.Contains(got, "POSITIONING") {
		t.Errorf("FormatCOTOverview(nil,nil): want 'POSITIONING' in output")
	}
}

func TestFormatCOTOverview_EmptySlices(t *testing.T) {
	f := NewFormatter()
	got := f.FormatCOTOverview([]domain.COTAnalysis{}, []cot.ConvictionScore{})
	if !strings.Contains(got, "COT") {
		t.Errorf("FormatCOTOverview(empty,empty): want 'COT' in output")
	}
}

func TestFormatCOTOverview_SingleCurrency(t *testing.T) {
	f := NewFormatter()
	analyses := []domain.COTAnalysis{
		{
			Contract: domain.COTContract{
				Code: "099741", Name: "Euro FX", Symbol: "6E",
				Currency: "EUR", ReportType: "TFF",
			},
			ReportDate:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			NetPosition: 50000,
			NetChange:   1000,
			COTIndex:    72.5,
			MomentumDir: domain.MomentumUp,
		},
	}
	got := f.FormatCOTOverview(analyses, nil)
	if !strings.Contains(got, "Euro FX") {
		t.Errorf("FormatCOTOverview: want 'Euro FX' in output, got: %s", got)
	}
	if !strings.Contains(got, "72") {
		t.Errorf("FormatCOTOverview: want COT index '72' in output")
	}
	if !strings.Contains(got, "LONG") {
		t.Errorf("FormatCOTOverview: want 'LONG' for positive net position")
	}
}

func TestFormatCOTOverview_WithNegativePosition(t *testing.T) {
	f := NewFormatter()
	analyses := []domain.COTAnalysis{
		{
			Contract: domain.COTContract{
				Code: "096742", Name: "British Pound", Symbol: "6B",
				Currency: "GBP", ReportType: "TFF",
			},
			ReportDate:  time.Now(),
			NetPosition: -30000,
			COTIndex:    25.0,
		},
	}
	got := f.FormatCOTOverview(analyses, nil)
	if !strings.Contains(got, "SHORT") {
		t.Errorf("FormatCOTOverview: want 'SHORT' for negative net position")
	}
}

func TestFormatCOTOverview_WithConvictions(t *testing.T) {
	f := NewFormatter()
	analyses := []domain.COTAnalysis{
		{
			Contract: domain.COTContract{
				Code: "099741", Name: "Euro FX", Symbol: "6E",
				Currency: "EUR", ReportType: "TFF",
			},
			ReportDate:  time.Now(),
			NetPosition: 50000,
			COTIndex:    70.0,
		},
	}
	convictions := []cot.ConvictionScore{
		{Currency: "EUR", Score: 75, Direction: "LONG"},
	}
	got := f.FormatCOTOverview(analyses, convictions)
	if !strings.Contains(got, "Conv") {
		t.Errorf("FormatCOTOverview with convictions: want 'Conv' label in output, got: %s", got)
	}
}

func TestFormatCOTDetail_MinimalInput(t *testing.T) {
	f := NewFormatter()
	a := domain.COTAnalysis{
		Contract: domain.COTContract{
			Code: "099741", Name: "Euro FX", Symbol: "6E",
			Currency: "EUR", ReportType: "TFF",
		},
		ReportDate:       time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
		NetPosition:      80000,
		NetChange:        2000,
		COTIndex:         65.0,
		CommercialSignal: "BEARISH",
		SpeculatorSignal: "BULLISH",
		ShortTermBias:    "LONG",
	}
	got := f.FormatCOTDetail(a)
	if !strings.Contains(got, "Euro FX") {
		t.Errorf("FormatCOTDetail: want contract name, got: %s", got)
	}
	if !strings.Contains(got, "COT Analysis") {
		t.Errorf("FormatCOTDetail: want 'COT Analysis' header")
	}
	if !strings.Contains(got, "TFF") {
		t.Errorf("FormatCOTDetail: want report type 'TFF'")
	}
}

func TestFormatCOTDetailWithCode_DisplayCode(t *testing.T) {
	f := NewFormatter()
	a := domain.COTAnalysis{
		Contract: domain.COTContract{
			Code: "099741", Name: "Euro FX", Symbol: "6E",
			Currency: "EUR", ReportType: "TFF",
		},
		ReportDate: time.Now(),
		COTIndex:   60.0,
	}
	got := f.FormatCOTDetailWithCode(a, "EUR")
	if !strings.Contains(got, "/cot EUR") {
		t.Errorf("FormatCOTDetailWithCode: want quick command '/cot EUR', got: %s", got)
	}
}

func TestFormatCOTDetail_Disaggregated(t *testing.T) {
	f := NewFormatter()
	a := domain.COTAnalysis{
		Contract: domain.COTContract{
			Code: "088691", Name: "Gold", Symbol: "GC",
			Currency: "XAU", ReportType: "DISAGGREGATED",
		},
		ReportDate:          time.Now(),
		NetPosition:         120000,
		COTIndex:            80.0,
		SmartDumbDivergence: true,
	}
	got := f.FormatCOTDetail(a)
	if !strings.Contains(got, "Managed Money") {
		t.Errorf("FormatCOTDetail DISAGG: want 'Managed Money' label")
	}
}

func TestFormatRanking_EmptyInput(t *testing.T) {
	f := NewFormatter()
	got := f.FormatRanking(nil, time.Now())
	if !strings.Contains(got, "Ranking") {
		t.Errorf("FormatRanking(nil): want 'Ranking' in output, got: %s", got)
	}
}

func TestFormatRanking_WithMajors(t *testing.T) {
	f := NewFormatter()
	now := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	analyses := []domain.COTAnalysis{
		{
			Contract:      domain.COTContract{Currency: "EUR"},
			SentimentScore: 60,
			COTIndex:      70,
		},
		{
			Contract:      domain.COTContract{Currency: "USD"},
			SentimentScore: -40,
			COTIndex:      30,
		},
		{
			Contract:      domain.COTContract{Currency: "GBP"},
			SentimentScore: 20,
			COTIndex:      55,
		},
	}
	got := f.FormatRanking(analyses, now)
	if !strings.Contains(got, "EUR") {
		t.Errorf("FormatRanking: want 'EUR' in output")
	}
	if !strings.Contains(got, "Currency Strength") {
		t.Errorf("FormatRanking: want 'Currency Strength' header")
	}
	// EUR should appear before USD (higher score)
	eurIdx := strings.Index(got, "EUR")
	usdIdx := strings.Index(got, "USD")
	if eurIdx == -1 || usdIdx == -1 {
		t.Skip("EUR or USD not in output (filtered as non-major)")
	}
	if eurIdx > usdIdx {
		t.Errorf("FormatRanking: EUR (score=60) should rank before USD (score=-40)")
	}
}

// ---------------------------------------------------------------------------
// Group B — Macro/FRED formatting
// ---------------------------------------------------------------------------

func makeMacroData() *fred.MacroData {
	return &fred.MacroData{
		FetchedAt:    time.Date(2026, 3, 25, 9, 0, 0, 0, time.UTC),
		Yield2Y:      4.5,
		Yield5Y:      4.3,
		Yield10Y:     4.2,
		Yield30Y:     4.4,
		Yield3M:      5.3,
		YieldSpread:  -0.3,
		Spread3M10Y:  -1.1,
		CorePCE:      2.8,
		CPI:          3.1,
		FedFundsRate: 5.25,
		Breakeven5Y:  2.5,
		NFCI:         -0.2,
		DXY:          104.5,
		SahmRule:     0.1,
		UnemployRate: 4.0,
		VIX:          18.5,
	}
}

func makeMacroRegime() fred.MacroRegime {
	return fred.MacroRegime{
		Name:          "DISINFLATIONARY",
		YieldCurve:    "INVERTED (-0.30%) 🔴",
		Inflation:     "Cooling (2.8%)",
		Labor:         "Stable",
		FinStress:     "Low",
		RecessionRisk: "LOW",
		Bias:          "USD NEUTRAL, Gold NEUTRAL",
		Description:   "Disinflation with inverted curve",
		Score:         35,
	}
}

func TestFormatMacroRegime_Basic(t *testing.T) {
	f := NewFormatter()
	regime := makeMacroRegime()
	data := makeMacroData()

	got := f.FormatMacroRegime(regime, data)
	if !strings.Contains(got, "MACRO REGIME") {
		t.Errorf("FormatMacroRegime: want 'MACRO REGIME' header, got: %s", got)
	}
	if !strings.Contains(got, "DISINFLATIONARY") {
		t.Errorf("FormatMacroRegime: want regime name 'DISINFLATIONARY'")
	}
	if !strings.Contains(got, "35") {
		t.Errorf("FormatMacroRegime: want risk score '35'")
	}
	if !strings.Contains(got, "INVERTED") {
		t.Errorf("FormatMacroRegime: want yield curve 'INVERTED'")
	}
}

func TestFormatMacroRegime_SahmAlert(t *testing.T) {
	f := NewFormatter()
	regime := makeMacroRegime()
	regime.SahmAlert = true
	regime.SahmLabel = "0.55 — Triggered"
	data := makeMacroData()

	got := f.FormatMacroRegime(regime, data)
	if !strings.Contains(got, "RECESSION SIGNAL") {
		t.Errorf("FormatMacroRegime with SahmAlert: want 'RECESSION SIGNAL'")
	}
}

func TestFormatFREDContext_NilData(t *testing.T) {
	f := NewFormatter()
	regime := makeMacroRegime()
	got := f.FormatFREDContext(nil, regime)
	if got != "" {
		t.Errorf("FormatFREDContext(nil, regime): want empty string, got %q", got)
	}
}

func TestFormatFREDContext_Basic(t *testing.T) {
	f := NewFormatter()
	data := makeMacroData()
	regime := makeMacroRegime()

	got := f.FormatFREDContext(data, regime)
	if !strings.Contains(got, "FRED Macro Context") {
		t.Errorf("FormatFREDContext: want 'FRED Macro Context' header")
	}
	if !strings.Contains(got, "DXY") {
		t.Errorf("FormatFREDContext: want 'DXY' in output")
	}
	if !strings.Contains(got, "DISINFLATIONARY") {
		t.Errorf("FormatFREDContext: want regime name in output")
	}
}

func TestFormatFREDContext_SahmAndInverted(t *testing.T) {
	f := NewFormatter()
	data := makeMacroData() // has YieldSpread=-0.3 and Spread3M10Y=-1.1
	regime := makeMacroRegime()
	regime.SahmAlert = true

	got := f.FormatFREDContext(data, regime)
	if !strings.Contains(got, "SAHM RULE") {
		t.Errorf("FormatFREDContext: want SAHM alert when SahmAlert=true")
	}
	if !strings.Contains(got, "INVERTED") {
		t.Errorf("FormatFREDContext: want INVERTED warning for negative spreads")
	}
}

func TestFormatMacroSummary_Basic(t *testing.T) {
	f := NewFormatter()
	regime := makeMacroRegime()
	data := makeMacroData()
	implications := []fred.TradingImplication{
		{Asset: "Gold", Direction: "BULLISH", Icon: "🥇", Reason: "Risk-off demand rising"},
		{Asset: "USD", Direction: "NEUTRAL", Icon: "💵", Reason: "Stable policy"},
	}

	got := f.FormatMacroSummary(regime, data, implications)
	if !strings.Contains(got, "MACRO SNAPSHOT") {
		t.Errorf("FormatMacroSummary: want 'MACRO SNAPSHOT' header")
	}
	if !strings.Contains(got, "Gold") {
		t.Errorf("FormatMacroSummary: want 'Gold' in implications")
	}
	if !strings.Contains(got, "BULLISH") {
		t.Errorf("FormatMacroSummary: want 'BULLISH' direction for gold")
	}
}

func TestFormatMacroSummary_EmptyImplications(t *testing.T) {
	f := NewFormatter()
	regime := makeMacroRegime()
	data := makeMacroData()

	got := f.FormatMacroSummary(regime, data, nil)
	if !strings.Contains(got, "MACRO SNAPSHOT") {
		t.Errorf("FormatMacroSummary(nil implications): should not crash")
	}
}

// ---------------------------------------------------------------------------
// Group C — Sentiment (FormatSentiment)
// ---------------------------------------------------------------------------

func makeSentimentData() *sentiment.SentimentData {
	return &sentiment.SentimentData{
		FetchedAt:                time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC),
		CNNFearGreed:             45.0,
		CNNFearGreedLabel:        "Fear",
		CNNAvailable:             true,
		CNNPrev1Week:             50.0,
		CNNPrev1Month:            60.0,
		CryptoFearGreed:          30.0,
		CryptoFearGreedLabel:     "Fear",
		CryptoFearGreedAvailable: true,
	}
}

func TestFormatSentiment_Basic(t *testing.T) {
	f := NewFormatter()
	data := makeSentimentData()

	got := f.FormatSentiment(data, "DISINFLATIONARY")
	if !strings.Contains(got, "SENTIMENT") {
		t.Errorf("FormatSentiment: want 'SENTIMENT' in header")
	}
	if !strings.Contains(got, "CNN") {
		t.Errorf("FormatSentiment: want 'CNN' Fear & Greed section")
	}
	if !strings.Contains(got, "45") {
		t.Errorf("FormatSentiment: want CNN score '45' in output")
	}
}

func TestFormatSentiment_ExtremeFear(t *testing.T) {
	f := NewFormatter()
	data := makeSentimentData()
	data.CNNFearGreed = 15 // extreme fear → contrarian BUY signal
	data.CNNFearGreedLabel = "Extreme Fear"

	got := f.FormatSentiment(data, "STRESS")
	if !strings.Contains(got, "Contrarian BUY") {
		t.Errorf("FormatSentiment extreme fear: want 'Contrarian BUY' signal")
	}
}

func TestFormatSentiment_ExtremeGreed(t *testing.T) {
	f := NewFormatter()
	data := makeSentimentData()
	data.CNNFearGreed = 85 // extreme greed → contrarian SELL signal
	data.CNNFearGreedLabel = "Extreme Greed"

	got := f.FormatSentiment(data, "INFLATIONARY")
	if !strings.Contains(got, "Contrarian SELL") {
		t.Errorf("FormatSentiment extreme greed: want 'Contrarian SELL' signal")
	}
}

func TestFormatSentiment_CNNUnavailable(t *testing.T) {
	f := NewFormatter()
	data := makeSentimentData()
	data.CNNAvailable = false

	got := f.FormatSentiment(data, "")
	if !strings.Contains(got, "unavailable") {
		t.Errorf("FormatSentiment CNN unavailable: want 'unavailable' notice")
	}
}

func TestFormatSentiment_VelocityAlert(t *testing.T) {
	f := NewFormatter()
	data := makeSentimentData()
	data.CNNFearGreed = 20
	data.CNNPrev1Month = 55 // drop of 35 → rapid decline alert

	got := f.FormatSentiment(data, "STRESS")
	if !strings.Contains(got, "Penurunan tajam") {
		t.Errorf("FormatSentiment velocity drop: want 'Penurunan tajam' alert")
	}
}
