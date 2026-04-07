package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// ============================================================================
// Tests for FormatCOTRaw - Raw CFTC data formatting
// ============================================================================

func TestFormatCOTRaw_Disaggregated(t *testing.T) {
	f := NewFormatter()
	record := domain.COTRecord{
		ContractCode: "088691",
		ContractName: "GOLD",
		ReportDate:   time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
		OpenInterest: 450000,
		ManagedMoneyLong: 150000,
		ManagedMoneyShort: 25000,
		ProdMercLong: 50000,
		ProdMercShort: 200000,
		SwapDealerLong: 25000,
		SwapDealerShort: 50000,
		TotalTradersDisag: 250,
		MMoneyLongTraders: 120,
		MMoneyShortTraders: 45,
	}

	out := f.FormatCOTRaw(record)
	if !strings.Contains(out, "GOLD") {
		t.Errorf("expected GOLD in raw output, got %q", out)
	}
	if !strings.Contains(out, "MANAGED MONEY") {
		t.Errorf("expected MANAGED MONEY section for disaggregated, got %q", out)
	}
	if !strings.Contains(out, "PROD/SWAP") {
		t.Errorf("expected PROD/SWAP section, got %q", out)
	}
}

func TestFormatCOTRaw_TFF(t *testing.T) {
	f := NewFormatter()
	record := domain.COTRecord{
		ContractCode: "099741",
		ContractName: "EURO FX",
		ReportDate:   time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
		OpenInterest: 800000,
		LevFundLong: 150000,
		LevFundShort: 80000,
		AssetMgrLong: 120000,
		AssetMgrShort: 30000,
		DealerLong: 50000,
		DealerShort: 100000,
		TotalTraders: 180,
		LevFundLongTraders: 65,
		LevFundShortTraders: 40,
	}

	out := f.FormatCOTRaw(record)
	if !strings.Contains(out, "LEVERAGED FUNDS") {
		t.Errorf("expected LEVERAGED FUNDS section for TFF, got %q", out)
	}
	if !strings.Contains(out, "ASSET MANAGER") {
		t.Errorf("expected ASSET MANAGER section, got %q", out)
	}
	if !strings.Contains(out, "DEALERS") {
		t.Errorf("expected DEALERS section, got %q", out)
	}
}

func TestFormatCOTRaw_EmptyRecord(t *testing.T) {
	f := NewFormatter()
	record := domain.COTRecord{
		ContractCode: "",
		ContractName: "UNKNOWN",
		ReportDate:   time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
	}

	// Should not panic
	out := f.FormatCOTRaw(record)
	if out == "" {
		t.Error("expected non-empty output even for empty record")
	}
}

func TestFormatCOTRaw_SmallSpecs(t *testing.T) {
	f := NewFormatter()
	record := domain.COTRecord{
		ContractCode: "088691",
		ContractName: "GOLD",
		ReportDate:   time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
		OpenInterest: 450000,
		SmallLong:    50000,
		SmallShort:   35000,
		ManagedMoneyLong: 150000,
		ManagedMoneyShort: 25000,
	}

	out := f.FormatCOTRaw(record)
	if !strings.Contains(out, "SMALL SPECULATORS") {
		t.Errorf("expected SMALL SPECULATORS section when data present, got %q", out)
	}
}

// ============================================================================
// Tests for FormatRankingWithConviction
// ============================================================================

func TestFormatRankingWithConviction_NoConviction(t *testing.T) {
	f := NewFormatter()
	analyses := []domain.COTAnalysis{
		{
			Contract: domain.COTContract{Code: "099741", Currency: "EUR"},
			COTIndex: 75.0,
		},
	}

	// When no conviction data, should fall back to FormatRanking
	out := f.FormatRankingWithConviction(analyses, nil, nil, time.Now())
	if !strings.Contains(out, "Ranking") {
		t.Errorf("expected Ranking in output, got %q", out)
	}
}

func TestFormatRankingWithConviction_WithData(t *testing.T) {
	f := NewFormatter()
	analyses := []domain.COTAnalysis{
		{
			Contract: domain.COTContract{Code: "099741", Currency: "EUR"},
			COTIndex: 75.0,
			SentimentScore: 65.0,
		},
		{
			Contract: domain.COTContract{Code: "096742", Currency: "GBP"},
			COTIndex: 45.0,
			SentimentScore: 30.0,
		},
	}
	convictions := []cot.ConvictionScore{
		{Currency: "EUR", Score: 78.0, Direction: "LONG", Label: "Strong Buy", Version: 3, COTComponent: 25, MacroComponent: 20, PriceComponent: 18, CalendarComponent: 15},
		{Currency: "GBP", Score: 45.0, Direction: "NEUTRAL", Label: "Neutral", Version: 3},
	}
	regime := &fred.MacroRegime{Name: "GOLDILOCKS", Score: 25}

	out := f.FormatRankingWithConviction(analyses, convictions, regime, time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(out, "EUR") {
		t.Errorf("expected EUR in ranking, got %q", out)
	}
	// Output uses "Conv" abbreviation for Conviction
	if !strings.Contains(out, "Conv:") {
		t.Errorf("expected Conv mention in output, got %q", out)
	}
}

func TestFormatRankingWithConviction_ThinMarketAlert(t *testing.T) {
	f := NewFormatter()
	analyses := []domain.COTAnalysis{
		{
			Contract: domain.COTContract{Code: "099741", Currency: "EUR"},
			COTIndex: 75.0,
			SentimentScore: 65.0,
			ThinMarketAlert: true,
			ThinMarketDesc: "Low liquidity period",
		},
	}
	convictions := []cot.ConvictionScore{
		{Currency: "EUR", Score: 70.0, Direction: "LONG"},
	}

	out := f.FormatRankingWithConviction(analyses, convictions, nil, time.Now())
	if !strings.Contains(out, "THIN") && !strings.Contains(out, "thin") {
		t.Errorf("expected thin market warning in output, got %q", out)
	}
}

func TestFormatRankingWithConviction_WithRegime(t *testing.T) {
	f := NewFormatter()
	analyses := []domain.COTAnalysis{
		{
			Contract: domain.COTContract{Code: "099741", Currency: "EUR"},
			COTIndex: 75.0,
		},
	}
	convictions := []cot.ConvictionScore{
		{Currency: "EUR", Score: 70.0, Direction: "LONG"},
	}
	regime := &fred.MacroRegime{Name: "STRESS", Score: 75}

	out := f.FormatRankingWithConviction(analyses, convictions, regime, time.Now())
	if !strings.Contains(out, "Regime") && !strings.Contains(out, "regime") {
		t.Errorf("expected regime mention in output, got %q", out)
	}
}

// ============================================================================
// Tests for FormatConvictionBlock
// ============================================================================

func TestFormatConvictionBlock_StrongBuy(t *testing.T) {
	f := NewFormatter()
	cs := cot.ConvictionScore{
		Score: 80.0,
		Direction: "LONG",
		Label: "Strong Buy",
	}

	out := f.FormatConvictionBlock(cs)
	if !strings.Contains(out, "STRONG BUY") && !strings.Contains(out, "KUAT") {
		t.Errorf("expected strong buy signal in output, got %q", out)
	}
}

func TestFormatConvictionBlock_StrongSell(t *testing.T) {
	f := NewFormatter()
	cs := cot.ConvictionScore{
		Score: 85.0,
		Direction: "SHORT",
		Label: "Strong Sell",
	}

	out := f.FormatConvictionBlock(cs)
	if !strings.Contains(out, "STRONG SELL") && !strings.Contains(out, "SELL SIGNAL") {
		t.Errorf("expected strong sell signal in output, got %q", out)
	}
}

func TestFormatConvictionBlock_Neutral(t *testing.T) {
	f := NewFormatter()
	cs := cot.ConvictionScore{
		Score: 45.0,
		Direction: "NEUTRAL",
		Label: "Neutral",
	}

	out := f.FormatConvictionBlock(cs)
	if !strings.Contains(out, "NETRAL") && !strings.Contains(out, "TIDAK JELAS") && !strings.Contains(out, "Neutral") {
		t.Errorf("expected neutral indication in output, got %q", out)
	}
}

func TestFormatConvictionBlock_WithComponents(t *testing.T) {
	f := NewFormatter()
	cs := cot.ConvictionScore{
		Score: 70.0,
		Direction: "LONG",
		Label: "Buy",
		COTBias: "BULLISH",
		FREDRegime: "GOLDILOCKS",
		Version: 3,
		COTComponent: 25,
		MacroComponent: 20,
		PriceComponent: 15,
		CalendarComponent: 10,
	}

	out := f.FormatConvictionBlock(cs)
	if !strings.Contains(out, "Bullish") && !strings.Contains(out, "bullish") {
		t.Errorf("expected bullish bias mention, got %q", out)
	}
}

// ============================================================================
// Tests for FormatBiasHTML
// ============================================================================

func TestFormatBiasHTML_NoSignals(t *testing.T) {
	f := NewFormatter()
	out := f.FormatBiasHTML(nil, "")
	if !strings.Contains(out, "No actionable") {
		t.Errorf("expected 'No actionable' message for empty signals, got %q", out)
	}
}

func TestFormatBiasHTML_WithSignals(t *testing.T) {
	f := NewFormatter()
	signals := []cot.Signal{
		{
			Currency:    "EUR",
			Direction:   "BULLISH",
			Type:        "Extreme Long",
			Strength:    5,
			Confidence:  85.0,
			Description: "Commercials at extreme short",
			Factors:     []string{"COT Index > 80", "Commercial extreme"},
		},
		{
			Currency:    "GBP",
			Direction:   "BEARISH",
			Type:        "Divergence",
			Strength:    4,
			Confidence:  72.0,
			Description: "Price up but COT down",
			Factors:     []string{"Divergence detected"},
		},
	}

	out := f.FormatBiasHTML(signals, "")
	if !strings.Contains(out, "EUR") {
		t.Errorf("expected EUR in bias output, got %q", out)
	}
	if !strings.Contains(out, "GBP") {
		t.Errorf("expected GBP in bias output, got %q", out)
	}
	if !strings.Contains(out, "Long") && !strings.Contains(out, "Bullish") {
		t.Errorf("expected bullish indication for EUR, got %q", out)
	}
}

func TestFormatBiasHTML_WithFilter(t *testing.T) {
	f := NewFormatter()
	signals := []cot.Signal{
		{
			Currency:    "EUR",
			Direction:   "BULLISH",
			Type:        "Extreme Long",
			Strength:    5,
			Confidence:  85.0,
			Description: "Commercials at extreme short",
		},
	}

	out := f.FormatBiasHTML(signals, "EUR")
	if !strings.Contains(out, "Filtered") && !strings.Contains(out, "EUR") {
		t.Errorf("expected filter indication in output, got %q", out)
	}
}

func TestFormatBiasHTML_MoreThan10Signals(t *testing.T) {
	f := NewFormatter()
	signals := make([]cot.Signal, 15)
	for i := 0; i < 15; i++ {
		signals[i] = cot.Signal{
			Currency:  "EUR",
			Direction: "BULLISH",
			Type:      "Test",
			Strength:  3,
		}
	}

	out := f.FormatBiasHTML(signals, "")
	if !strings.Contains(out, "+5 more") && !strings.Contains(out, "more") {
		t.Errorf("expected truncation message for many signals, got %q", out)
	}
}

// ============================================================================
// Tests for FormatBiasSummary
// ============================================================================

func TestFormatBiasSummary_NoSignals(t *testing.T) {
	f := NewFormatter()
	out := f.FormatBiasSummary(nil)
	if out != "" {
		t.Errorf("expected empty output for no signals, got %q", out)
	}
}

func TestFormatBiasSummary_EmptySlice(t *testing.T) {
	f := NewFormatter()
	out := f.FormatBiasSummary([]cot.Signal{})
	if out != "" {
		t.Errorf("expected empty output for empty slice, got %q", out)
	}
}

func TestFormatBiasSummary_WithSignals(t *testing.T) {
	f := NewFormatter()
	signals := []cot.Signal{
		{
			Currency:    "EUR",
			Direction:   "BULLISH",
			Type:        "Extreme Long",
			Strength:    5,
			Confidence:  85.0,
			Description: "Commercials at extreme short",
		},
	}

	out := f.FormatBiasSummary(signals)
	// FormatBiasSummary shows signal type and description, not currency directly
	if !strings.Contains(out, "Extreme Long") {
		t.Errorf("expected signal type in summary, got %q", out)
	}
	if !strings.Contains(out, "Active Biases") && !strings.Contains(out, "Biases") {
		t.Errorf("expected 'Active Biases' header, got %q", out)
	}
}

func TestFormatBiasSummary_MoreThan3Signals(t *testing.T) {
	f := NewFormatter()
	signals := []cot.Signal{
		{Currency: "EUR", Direction: "BULLISH", Type: "T1", Strength: 5, Confidence: 80, Description: "D1"},
		{Currency: "GBP", Direction: "BEARISH", Type: "T2", Strength: 4, Confidence: 75, Description: "D2"},
		{Currency: "JPY", Direction: "BULLISH", Type: "T3", Strength: 3, Confidence: 70, Description: "D3"},
		{Currency: "AUD", Direction: "BEARISH", Type: "T4", Strength: 4, Confidence: 72, Description: "D4"},
	}

	out := f.FormatBiasSummary(signals)
	if !strings.Contains(out, "+1 more") || !strings.Contains(out, "more signals") {
		t.Errorf("expected truncation for >3 signals, got %q", out)
	}
}

// ============================================================================
// Tests for FormatPriceCOTDivergence
// ============================================================================

func TestFormatPriceCOTDivergence_PriceUpCOTBearish(t *testing.T) {
	f := NewFormatter()
	div := pricesvc.PriceCOTDivergence{
		Currency:     "EUR",
		PriceTrend:   "UP",
		COTDirection: "BEARISH",
		COTIndex:     25.0,
		Severity:     "MEDIUM",
		Description:  "Price rising but institutions selling",
	}

	out := f.FormatPriceCOTDivergence(div)
	// FormatPriceCOTDivergence uses Indonesian: "SINYAL BERTENTANGAN" for divergence
	if !strings.Contains(out, "SINYAL BERTENTANGAN") && !strings.Contains(out, "Bertentangan") && !strings.Contains(out, "bertentangan") {
		t.Errorf("expected divergence warning (SINYAL BERTENTANGAN), got %q", out)
	}
}

func TestFormatPriceCOTDivergence_PriceDownCOTBullish(t *testing.T) {
	f := NewFormatter()
	div := pricesvc.PriceCOTDivergence{
		Currency:     "EUR",
		PriceTrend:   "DOWN",
		COTDirection: "BULLISH",
		COTIndex:     85.0,
		Severity:     "MEDIUM",
		Description:  "Price falling but institutions buying",
	}

	out := f.FormatPriceCOTDivergence(div)
	if !strings.Contains(out, "turun") && !strings.Contains(out, "beli") && !strings.Contains(out, "falling") {
		t.Errorf("expected bearish divergence description, got %q", out)
	}
}

func TestFormatPriceCOTDivergence_HighSeverity(t *testing.T) {
	f := NewFormatter()
	div := pricesvc.PriceCOTDivergence{
		Currency:     "EUR",
		PriceTrend:   "UP",
		COTDirection: "BEARISH",
		COTIndex:     15.0,
		Severity:     "HIGH",
		Description:  "Extreme divergence detected",
	}

	out := f.FormatPriceCOTDivergence(div)
	if !strings.Contains(out, "🚨") && !strings.Contains(out, "KERAS") && !strings.Contains(out, "HIGH") {
		t.Errorf("expected high severity warning, got %q", out)
	}
}

// ============================================================================
// Tests for FormatPriceCOTAlignment
// ============================================================================

func TestFormatPriceCOTAlignment_AgreedUp(t *testing.T) {
	f := NewFormatter()
	pc := &domain.PriceContext{
		Trend4W: "UP",
	}
	a := domain.COTAnalysis{
		Contract: domain.COTContract{Currency: "EUR"},
		COTIndex: 75.0,
	}

	out := f.FormatPriceCOTAlignment(pc, a)
	if !strings.Contains(out, "SELARAS") && !strings.Contains(out, "separas") && !strings.Contains(out, "aligned") {
		t.Errorf("expected alignment confirmation, got %q", out)
	}
}

func TestFormatPriceCOTAlignment_AgreedDown(t *testing.T) {
	f := NewFormatter()
	pc := &domain.PriceContext{
		Trend4W: "DOWN",
	}
	a := domain.COTAnalysis{
		Contract: domain.COTContract{Currency: "EUR"},
		COTIndex: 25.0,
	}

	out := f.FormatPriceCOTAlignment(pc, a)
	if !strings.Contains(out, "SELARAS") && !strings.Contains(out, "separas") && !strings.Contains(out, "aligned") {
		t.Errorf("expected alignment confirmation for down trend, got %q", out)
	}
}

func TestFormatPriceCOTAlignment_PriceUpCOTNeutral(t *testing.T) {
	f := NewFormatter()
	pc := &domain.PriceContext{
		Trend4W: "UP",
	}
	a := domain.COTAnalysis{
		Contract: domain.COTContract{Currency: "EUR"},
		COTIndex: 50.0,
	}

	out := f.FormatPriceCOTAlignment(pc, a)
	if !strings.Contains(out, "netral") && !strings.Contains(out, "neutral") && !strings.Contains(out, "belum") {
		t.Errorf("expected neutral warning, got %q", out)
	}
}

func TestFormatPriceCOTAlignment_NilPriceContext(t *testing.T) {
	f := NewFormatter()
	a := domain.COTAnalysis{
		Contract: domain.COTContract{Currency: "EUR"},
		COTIndex: 75.0,
	}

	out := f.FormatPriceCOTAlignment(nil, a)
	if out != "" {
		t.Errorf("expected empty output for nil price context, got %q", out)
	}
}

func TestFormatPriceCOTAlignment_FlatPrice(t *testing.T) {
	f := NewFormatter()
	pc := &domain.PriceContext{
		Trend4W: "FLAT",
	}
	a := domain.COTAnalysis{
		Contract: domain.COTContract{Currency: "EUR"},
		COTIndex: 75.0,
	}

	out := f.FormatPriceCOTAlignment(pc, a)
	if !strings.Contains(out, "sideways") && !strings.Contains(out, "netral") && !strings.Contains(out, "belum") {
		t.Errorf("expected sideways/accumulation message, got %q", out)
	}
}

// ============================================================================
// Tests for FormatStrengthRanking
// ============================================================================

func TestFormatStrengthRanking_Empty(t *testing.T) {
	f := NewFormatter()
	out := f.FormatStrengthRanking(nil)
	if out != "" {
		t.Errorf("expected empty output for nil input, got %q", out)
	}
}

func TestFormatStrengthRanking_WithData(t *testing.T) {
	f := NewFormatter()
	strengths := []pricesvc.CurrencyStrength{
		{
			Currency:      "EUR",
			PriceScore:    75.5,
			COTScore:      72.0,
			CombinedScore: 73.8,
			Divergence:    false,
		},
		{
			Currency:       "GBP",
			PriceScore:    45.2,
			COTScore:      80.1,
			CombinedScore: 62.6,
			Divergence:    true,
			DivergenceMsg: "Price-COT divergence detected",
		},
	}

	out := f.FormatStrengthRanking(strengths)
	if !strings.Contains(out, "EUR") {
		t.Errorf("expected EUR in ranking, got %q", out)
	}
	if !strings.Contains(out, "GBP") {
		t.Errorf("expected GBP in ranking, got %q", out)
	}
}

func TestFormatStrengthRanking_WithDivergence(t *testing.T) {
	f := NewFormatter()
	strengths := []pricesvc.CurrencyStrength{
		{
			Currency:       "GBP",
			PriceScore:    45.2,
			COTScore:      80.1,
			CombinedScore: 62.6,
			Divergence:    true,
			DivergenceMsg: "Price-COT divergence detected",
		},
	}

	out := f.FormatStrengthRanking(strengths)
	if !strings.Contains(out, "divergence") && !strings.Contains(out, "!") {
		t.Errorf("expected divergence indicator, got %q", out)
	}
}

// ============================================================================
// Tests for buildBestPairs (detailed)
// ============================================================================

func TestBuildBestPairs_MinimumSpread(t *testing.T) {
	entries := []rankEntry{
		{Currency: "EUR", Score: 80.0, COTIndex: 85.0},
		{Currency: "GBP", Score: 50.0, COTIndex: 55.0},
		{Currency: "JPY", Score: 20.0, COTIndex: 25.0},
	}

	pairs := buildBestPairs(entries)
	// EUR (80) vs JPY (20) = spread 60, should be included
	if len(pairs) == 0 {
		t.Errorf("expected pairs to be generated, got none")
	}
}

func TestBuildBestPairs_NoMeaningfulSpread(t *testing.T) {
	entries := []rankEntry{
		{Currency: "EUR", Score: 55.0, COTIndex: 55.0},
		{Currency: "GBP", Score: 50.0, COTIndex: 50.0},
		{Currency: "JPY", Score: 45.0, COTIndex: 45.0},
	}

	pairs := buildBestPairs(entries)
	// All spreads are < 30, so fallback to best available
	if len(pairs) == 0 {
		t.Errorf("expected at least one pair (fallback), got none")
	}
}

func TestBuildBestPairs_InsufficientEntries(t *testing.T) {
	entries := []rankEntry{
		{Currency: "EUR", Score: 80.0, COTIndex: 85.0},
	}

	pairs := buildBestPairs(entries)
	if pairs != nil {
		t.Errorf("expected nil for insufficient entries, got %v", pairs)
	}
}

// ============================================================================
// Tests for signalConfluenceInterpretation (detailed)
// ============================================================================

func TestSignalConfluenceInterpretation_StrongAgreement(t *testing.T) {
	out := signalConfluenceInterpretation("STRONG_BULLISH", "STRONG_BULLISH", "TFF")
	if !strings.Contains(out, "KUAT") && !strings.Contains(out, "STRONG") && !strings.Contains(out, "strong") {
		t.Errorf("expected strong agreement message, got %q", out)
	}
}

func TestSignalConfluenceInterpretation_CommodityDivergence(t *testing.T) {
	// For DISAGGREGATED, spec bullish + comm bearish is NORMAL
	out := signalConfluenceInterpretation("BULLISH", "BEARISH", "DISAGGREGATED")
	if !strings.Contains(out, "Normal") && !strings.Contains(out, "normal") && !strings.Contains(out, "NORMAL") {
		t.Errorf("expected normal divergence message for commodities, got %q", out)
	}
}

func TestSignalConfluenceInterpretation_ForexDivergence(t *testing.T) {
	// For TFF, divergence is a warning
	out := signalConfluenceInterpretation("BULLISH", "BEARISH", "TFF")
	if !strings.Contains(out, "KONFLIK") && !strings.Contains(out, "konflik") && !strings.Contains(out, "conflict") {
		t.Errorf("expected conflict warning for forex divergence, got %q", out)
	}
}

func TestSignalConfluenceInterpretation_CommercialNeutral(t *testing.T) {
	out := signalConfluenceInterpretation("BULLISH", "NEUTRAL", "TFF")
	if !strings.Contains(out, "netral") && !strings.Contains(out, "neutral") && !strings.Contains(out, "belum") {
		t.Errorf("expected neutral hedger message, got %q", out)
	}
}

// ============================================================================
// Tests for FormatCOTShareText
// ============================================================================

func TestFormatCOTShareText_Bullish(t *testing.T) {
	f := NewFormatter()
	a := domain.COTAnalysis{
		Contract: domain.COTContract{
			Code: "099741",
			Name: "EURO FX",
			Currency: "EUR",
		},
		ReportDate:  time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
		NetPosition: 50000,
		COTIndex:    75.5,
		NetChange:   3000,
		SpeculatorSignal: "BULLISH",
		SpecMomentum4W: 2500,
	}

	out := f.FormatCOTShareText(a)
	if !strings.Contains(out, "EUR") {
		t.Errorf("expected EUR in share text, got %q", out)
	}
	if !strings.Contains(out, "Bullish") && !strings.Contains(out, "BULLISH") {
		t.Errorf("expected bullish indication, got %q", out)
	}
}

func TestFormatCOTShareText_Bearish(t *testing.T) {
	f := NewFormatter()
	a := domain.COTAnalysis{
		Contract: domain.COTContract{
			Code: "096742",
			Name: "BRITISH POUND",
			Currency: "GBP",
		},
		ReportDate:  time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
		NetPosition: -45000,
		COTIndex:    22.0,
		NetChange:   -2500,
		SpeculatorSignal: "BEARISH",
	}

	out := f.FormatCOTShareText(a)
	if !strings.Contains(out, "Bearish") && !strings.Contains(out, "BEARISH") {
		t.Errorf("expected bearish indication, got %q", out)
	}
}

func TestFormatCOTShareText_Neutral(t *testing.T) {
	f := NewFormatter()
	a := domain.COTAnalysis{
		Contract: domain.COTContract{
			Code: "097741",
			Name: "JAPANESE YEN",
			Currency: "JPY",
		},
		ReportDate:  time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
		NetPosition: 0,
		COTIndex:    50.0,
		NetChange:   0,
		SpeculatorSignal: "NEUTRAL",
	}

	out := f.FormatCOTShareText(a)
	if !strings.Contains(out, "Neutral") && !strings.Contains(out, "NEUTRAL") {
		t.Errorf("expected neutral indication, got %q", out)
	}
}

// ============================================================================
// Tests for FormatCOTDetail (comprehensive alert testing)
// ============================================================================

func TestFormatCOTDetail_AllAlerts(t *testing.T) {
	f := NewFormatter()
	a := domain.COTAnalysis{
		Contract: domain.COTContract{
			Code: "088691",
			Name: "GOLD",
			Currency: "GOLD",
			ReportType: "DISAGGREGATED",
		},
		ReportDate:  time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
		COTIndex:    85.0,
		NetPosition: 120000,
		NetChange:   8000,
		AssetMgrAlert: true,
		AssetMgrZScore: 2.5,
		ThinMarketAlert: true,
		ThinMarketDesc: "Holiday trading period",
		SmartDumbDivergence: true,
		CommExtremeBull: true,
		CommExtremeBear: false,
		CategoryDivergence: true,
		CategoryDivergenceDesc: "Managed Money vs SwapDealer divergence",
		DealerAlert: true,
		LevFundAlert: true,
		ManagedMoneyAlert: true,
		SwapDealerAlert: false,
		DealerZScore: 2.3,
		LevFundZScore: -2.1,
		ManagedMoneyZScore: 2.8,
		SwapDealerZScore: 0.5,
	}

	out := f.FormatCOTDetail(a)
	if !strings.Contains(out, "Asset Manager") || !strings.Contains(out, "Structural") {
		t.Errorf("expected Asset Manager alert, got %q", out)
	}
	if !strings.Contains(out, "THIN MARKET") && !strings.Contains(out, "Thin") {
		t.Errorf("expected thin market alert, got %q", out)
	}
}

// ============================================================================
// Tests for FormatCOTOverview with conviction
// ============================================================================

func TestFormatCOTOverview_WithConviction(t *testing.T) {
	f := NewFormatter()
	analyses := []domain.COTAnalysis{
		{
			Contract: domain.COTContract{
				Code: "099741",
				Name: "EURO FX",
				Currency: "EUR",
			},
			COTIndex:    75.0,
			NetPosition: 45000,
			NetChange:   3000,
			MomentumDir: "UP",
		},
	}
	convictions := []cot.ConvictionScore{
		{
			Currency: "EUR",
			Score:    72.0,
			Direction: "LONG",
		},
	}

	out := f.FormatCOTOverview(analyses, convictions)
	if !strings.Contains(out, "Conv") {
		t.Errorf("expected Conv indication when convictions present, got %q", out)
	}
}

// ============================================================================
// Edge case tests
// ============================================================================

func TestBuildBestPairs_MultipleCombinations(t *testing.T) {
	entries := []rankEntry{
		{Currency: "EUR", Score: 90.0, COTIndex: 95.0},
		{Currency: "GBP", Score: 70.0, COTIndex: 75.0},
		{Currency: "AUD", Score: 60.0, COTIndex: 65.0},
		{Currency: "NZD", Score: 40.0, COTIndex: 45.0},
		{Currency: "JPY", Score: 10.0, COTIndex: 15.0},
	}

	pairs := buildBestPairs(entries)
	// Should generate up to 3 pairs
	if len(pairs) == 0 {
		t.Errorf("expected pairs to be generated, got none")
	}
	if len(pairs) > 3 {
		t.Errorf("expected at most 3 pairs, got %d", len(pairs))
	}
}
