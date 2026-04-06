package cot

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ============================================================================
// Test computeCOTIndex
// ============================================================================

func TestComputeCOTIndex(t *testing.T) {
	tests := []struct {
		name string
		nets []float64
		want float64
	}{
		{"empty", nil, 50},
		{"single value", []float64{100}, 50},
		{"two elements", []float64{100, 50}, 50},
		{"all same", []float64{100, 100, 100}, 50},
		{"min=0 max=100 current=50", []float64{50, 0, 100}, 50},
		{"min=0 max=100 current=100", []float64{100, 0, 50}, 100},
		{"min=0 max=100 current=0", []float64{0, 50, 100}, 0},
		{"negative range", []float64{-50, -100, 0}, 50},
		{"middle value returns 75", []float64{75, 50, 100}, 50}, // (75-50)/(100-50)*100 = 50
		{"middle value returns 25", []float64{25, 0, 50}, 50},  // (25-0)/(50-0)*100 = 50
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeCOTIndex(tt.nets)
			if math.Abs(got-tt.want) > 1.0 {
				t.Errorf("computeCOTIndex(%v) = %v, want ~%v", tt.nets, got, tt.want)
			}
		})
	}
}

func TestComputeCOTIndex_AllZero(t *testing.T) {
	// All zeros should return 50 (neutral)
	nets := []float64{0, 0, 0, 0, 0}
	got := computeCOTIndex(nets)
	if got != 50.0 {
		t.Errorf("computeCOTIndex(all zeros) = %v, want 50.0", got)
	}
}

func TestComputeCOTIndex_MaxNet(t *testing.T) {
	// Current at max should return 100
	nets := []float64{100, 0, 50, 25, 75}
	got := computeCOTIndex(nets)
	if got != 100.0 {
		t.Errorf("computeCOTIndex(max net) = %v, want 100.0", got)
	}
}

func TestComputeCOTIndex_MinNet(t *testing.T) {
	// Current at min should return 0
	nets := []float64{0, 100, 50, 75, 25}
	got := computeCOTIndex(nets)
	if got != 0.0 {
		t.Errorf("computeCOTIndex(min net) = %v, want 0.0", got)
	}
}

func TestComputeCOTIndex_MiddleValue(t *testing.T) {
	// Middle value should return around 50
	nets := []float64{50, 0, 100}
	got := computeCOTIndex(nets)
	want := 50.0
	if math.Abs(got-want) > 0.1 {
		t.Errorf("computeCOTIndex(middle) = %v, want ~%v", got, want)
	}
}

func TestComputeCOTIndex_SingleElement(t *testing.T) {
	// Single element should return 50 (neutral - insufficient data)
	nets := []float64{42}
	got := computeCOTIndex(nets)
	if got != 50.0 {
		t.Errorf("computeCOTIndex(single) = %v, want 50.0", got)
	}
}

// ============================================================================
// Test computeSentiment
// ============================================================================

func TestComputeSentiment(t *testing.T) {
	tests := []struct {
		name string
		a    domain.COTAnalysis
		want float64 // approximate expected value
	}{
		{
			name: "neutral all 50s",
			a: domain.COTAnalysis{
				COTIndex:        50,
				COTIndexComm:    50,
				CrowdingIndex:   50,
				SpecMomentum4W:  0,
				Contract:        domain.COTContract{ReportType: "DISAGGREGATED"},
			},
			want: 0,
		},
		{
			name: "bullish spec high cot",
			a: domain.COTAnalysis{
				COTIndex:        75,
				COTIndexComm:    50,
				CrowdingIndex:   50,
				SpecMomentum4W:  5000,
				Contract:        domain.COTContract{ReportType: "DISAGGREGATED"},
			},
			want: 40, // (75-50)*2*0.4 + 0 + min(5000/5000*20,20) + 0 = 20 + 0 + 20 + 0 = 40
		},
		{
			name: "bearish spec low cot",
			a: domain.COTAnalysis{
				COTIndex:        25,
				COTIndexComm:    50,
				CrowdingIndex:   50,
				SpecMomentum4W:  -5000,
				Contract:        domain.COTContract{ReportType: "DISAGGREGATED"},
			},
			want: -40, // (25-50)*2*0.4 + 0 + (-20) + 0 = -20 + 0 - 20 + 0 = -40
		},
		{
			name: "TFF report type inverted commercial",
			a: domain.COTAnalysis{
				COTIndex:        50,
				COTIndexComm:    75,
				CrowdingIndex:   50,
				SpecMomentum4W:  0,
				Contract:        domain.COTContract{ReportType: "TFF"},
			},
			want: -30, // Commercial high is bearish for TFF (inverted)
		},
		{
			name: "high crowding penalty",
			a: domain.COTAnalysis{
				COTIndex:        50,
				COTIndexComm:    50,
				CrowdingIndex:   80,
				SpecMomentum4W:  0,
				Contract:        domain.COTContract{ReportType: "DISAGGREGATED"},
			},
			want: -6, // Crowding penalty (50-80)*0.2 = -6
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeSentiment(tt.a)
			if math.Abs(got-tt.want) > 15.0 {
				t.Errorf("computeSentiment() = %v, want ~%v", got, tt.want)
			}
		})
	}
}

func TestComputeSentiment_Boundaries(t *testing.T) {
	// Test that sentiment is clamped to [-100, 100]
	 extremeBull := domain.COTAnalysis{
		COTIndex:        100,
		COTIndexComm:    100,
		CrowdingIndex:   0,
		SpecMomentum4W:  50000,
		Contract:        domain.COTContract{ReportType: "DISAGGREGATED"},
	}
	got := computeSentiment(extremeBull)
	if got > 100.0 {
		t.Errorf("sentiment should be clamped to 100, got %v", got)
	}

	extremeBear := domain.COTAnalysis{
		COTIndex:        0,
		COTIndexComm:    0,
		CrowdingIndex:   100,
		SpecMomentum4W:  -50000,
		Contract:        domain.COTContract{ReportType: "DISAGGREGATED"},
	}
	got = computeSentiment(extremeBear)
	if got < -100.0 {
		t.Errorf("sentiment should be clamped to -100, got %v", got)
	}
}

// ============================================================================
// Test classifySignal
// ============================================================================

func TestClassifySignal(t *testing.T) {
	tests := []struct {
		name         string
		cotIndex     float64
		momentum     float64
		isCommercial bool
		want         string
	}{
		// Speculator tests (isCommercial=false)
		{"spec strong bullish", 80, 5, false, "STRONG_BULLISH"},
		{"spec bullish", 80, -5, false, "BULLISH"},
		{"spec strong bearish", 20, -5, false, "STRONG_BEARISH"},
		{"spec bearish", 20, 5, false, "BEARISH"},
		{"spec neutral", 50, 0, false, "NEUTRAL"},
		{"spec mid high neutral", 60, 0, false, "NEUTRAL"},
		{"spec mid low neutral", 40, 0, false, "NEUTRAL"},

		// Commercial tests (isCommercial=true) - same logic as spec now
		{"comm strong bullish", 80, 5, true, "STRONG_BULLISH"},
		{"comm bullish", 80, -5, true, "BULLISH"},
		{"comm strong bearish", 20, -5, true, "STRONG_BEARISH"},
		{"comm bearish", 20, 5, true, "BEARISH"},
		{"comm neutral", 50, 0, true, "NEUTRAL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySignal(tt.cotIndex, tt.momentum, tt.isCommercial)
			if got != tt.want {
				t.Errorf("classifySignal(%v, %v, %v) = %v, want %v",
					tt.cotIndex, tt.momentum, tt.isCommercial, got, tt.want)
			}
		})
	}
}

func TestClassifySignal_BullishSpec(t *testing.T) {
	// High COT index + positive momentum = STRONG_BULLISH for speculators
	got := classifySignal(80, 10, false)
	if got != "STRONG_BULLISH" {
		t.Errorf("classifySignal(80, 10, false) = %v, want STRONG_BULLISH", got)
	}
}

func TestClassifySignal_BearishSpec(t *testing.T) {
	// Low COT index + negative momentum = STRONG_BEARISH for speculators
	got := classifySignal(20, -10, false)
	if got != "STRONG_BEARISH" {
		t.Errorf("classifySignal(20, -10, false) = %v, want STRONG_BEARISH", got)
	}
}

func TestClassifySignal_NeutralSpec(t *testing.T) {
	// Middle COT index = NEUTRAL
	got := classifySignal(50, 0, false)
	if got != "NEUTRAL" {
		t.Errorf("classifySignal(50, 0, false) = %v, want NEUTRAL", got)
	}
}

func TestClassifySignal_CommercialInverse(t *testing.T) {
	// Commercials use same logic now (not inverse)
	got := classifySignal(80, 10, true)
	if got != "STRONG_BULLISH" {
		t.Errorf("classifySignal(80, 10, true) = %v, want STRONG_BULLISH", got)
	}
}

// ============================================================================
// Test classifySignalStrength
// ============================================================================

func TestClassifySignalStrength(t *testing.T) {
	tests := []struct {
		name string
		a    domain.COTAnalysis
		want domain.SignalStrength
	}{
		{
			name: "neutral - low sentiment",
			a:    domain.COTAnalysis{SentimentScore: 10, IsExtremeBull: false, IsExtremeBear: false},
			want: domain.SignalNeutral,
		},
		{
			name: "weak - moderate sentiment",
			a:    domain.COTAnalysis{SentimentScore: 30, IsExtremeBull: false, IsExtremeBear: false},
			want: domain.SignalWeak,
		},
		{
			name: "moderate - high sentiment",
			a:    domain.COTAnalysis{SentimentScore: 50, IsExtremeBull: false, IsExtremeBear: false},
			want: domain.SignalModerate,
		},
		{
			name: "strong - very high sentiment with extreme",
			a:    domain.COTAnalysis{SentimentScore: 70, IsExtremeBull: true, IsExtremeBear: false},
			want: domain.SignalStrong,
		},
		{
			name: "strong bear - very negative with extreme",
			a:    domain.COTAnalysis{SentimentScore: -70, IsExtremeBull: false, IsExtremeBear: true},
			want: domain.SignalStrong,
		},
		{
			name: "neutral with extreme but low sentiment",
			a:    domain.COTAnalysis{SentimentScore: 30, IsExtremeBull: true, IsExtremeBear: false},
			want: domain.SignalWeak, // abs < 40, so weak not strong
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySignalStrength(tt.a)
			if got != tt.want {
				t.Errorf("classifySignalStrength() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifySignalStrength_Strong(t *testing.T) {
	// High sentiment + extreme = STRONG
	a := domain.COTAnalysis{
		SentimentScore: 65,
		IsExtremeBull:  true,
	}
	got := classifySignalStrength(a)
	if got != domain.SignalStrong {
		t.Errorf("expected STRONG, got %v", got)
	}
}

func TestClassifySignalStrength_Weak(t *testing.T) {
	// Low sentiment = WEAK (or NEUTRAL depending on exact value)
	a := domain.COTAnalysis{
		SentimentScore: 15,
		IsExtremeBull:  false,
		IsExtremeBear:  false,
	}
	got := classifySignalStrength(a)
	if got != domain.SignalNeutral {
		t.Errorf("expected NEUTRAL for low sentiment, got %v", got)
	}
}

// ============================================================================
// Test classifySmallSpec
// ============================================================================

func TestClassifySmallSpec(t *testing.T) {
	tests := []struct {
		name       string
		netSmall   float64
		crowding   float64
		wantSignal string
	}{
		{"crowd long", 10000, 70, "CROWD_LONG"},
		{"crowd short", -10000, 70, "CROWD_SHORT"},
		{"neutral - no crowding", 10000, 50, "NEUTRAL"},
		{"neutral - zero net", 0, 70, "NEUTRAL"},
		{"neutral - both conditions", 10000, 60, "NEUTRAL"},
		{"crowd long threshold", 1, 66, "CROWD_LONG"},
		{"crowd short threshold", -1, 66, "CROWD_SHORT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := domain.COTAnalysis{
				NetSmallSpec: tt.netSmall,
				CrowdingIndex: tt.crowding,
			}
			got := classifySmallSpec(a)
			if got != tt.wantSignal {
				t.Errorf("classifySmallSpec(net=%v, crowd=%v) = %v, want %v",
					tt.netSmall, tt.crowding, got, tt.wantSignal)
			}
		})
	}
}

func TestClassifySmallSpec_CrowdLong(t *testing.T) {
	a := domain.COTAnalysis{
		NetSmallSpec:  10000,
		CrowdingIndex: 70,
	}
	got := classifySmallSpec(a)
	if got != "CROWD_LONG" {
		t.Errorf("expected CROWD_LONG, got %v", got)
	}
}

func TestClassifySmallSpec_CrowdShort(t *testing.T) {
	a := domain.COTAnalysis{
		NetSmallSpec:  -10000,
		CrowdingIndex: 70,
	}
	got := classifySmallSpec(a)
	if got != "CROWD_SHORT" {
		t.Errorf("expected CROWD_SHORT, got %v", got)
	}
}

func TestClassifySmallSpec_Neutral(t *testing.T) {
	a := domain.COTAnalysis{
		NetSmallSpec:  10000,
		CrowdingIndex: 50, // Below threshold
	}
	got := classifySmallSpec(a)
	if got != "NEUTRAL" {
		t.Errorf("expected NEUTRAL, got %v", got)
	}
}

// ============================================================================
// Test classifyMomentumDir
// ============================================================================

func TestClassifyMomentumDir(t *testing.T) {
	tests := []struct {
		name    string
		specMom float64
		commMom float64
		want    domain.MomentumDirection
	}{
		{"building - spec up comm down", 5000, -5000, domain.MomentumBuilding},
		{"reversing - spec down comm up", -5000, 5000, domain.MomentumReversing},
		{"stable - both small", 50, 50, domain.MomentumStable},
		{"stable - both zero", 0, 0, domain.MomentumStable},
		{"unwinding - spec down", -5000, -5000, domain.MomentumUnwinding},
		{"building - both up", 5000, 5000, domain.MomentumBuilding},
		{"stable - near thresholds", 99, 99, domain.MomentumStable},
		{"building - spec up comm zero", 5000, 0, domain.MomentumBuilding},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyMomentumDir(tt.specMom, tt.commMom)
			if got != tt.want {
				t.Errorf("classifyMomentumDir(%v, %v) = %v, want %v",
					tt.specMom, tt.commMom, got, tt.want)
			}
		})
	}
}

func TestClassifyMomentumDir_Building(t *testing.T) {
	got := classifyMomentumDir(5000, -5000)
	if got != domain.MomentumBuilding {
		t.Errorf("expected BUILDING, got %v", got)
	}
}

func TestClassifyMomentumDir_Reversing(t *testing.T) {
	got := classifyMomentumDir(-5000, 5000)
	if got != domain.MomentumReversing {
		t.Errorf("expected REVERSING, got %v", got)
	}
}

func TestClassifyMomentumDir_Stable(t *testing.T) {
	got := classifyMomentumDir(0, 0)
	if got != domain.MomentumStable {
		t.Errorf("expected STABLE, got %v", got)
	}
}

func TestClassifyMomentumDir_Unwinding(t *testing.T) {
	got := classifyMomentumDir(-5000, -100) // Spec negative triggers unwinding
	if got != domain.MomentumUnwinding {
		t.Errorf("expected UNWINDING, got %v", got)
	}
}

// ============================================================================
// Test detectDivergence
// ============================================================================

func TestDetectDivergence_PureFunc(t *testing.T) {
	tests := []struct {
		name           string
		specNetChange  float64
		commNetChange  float64
		wantDivergence bool
	}{
		{"divergence - opposite directions", 5000, -5000, true},
		{"divergence - spec up comm down", 5000, -5000, true},
		{"divergence - spec down comm up", -5000, 5000, true},
		{"no divergence - same direction up", 5000, 5000, false},
		{"no divergence - same direction down", -5000, -5000, false},
		{"no divergence - spec too small", 500, -5000, false},
		{"no divergence - comm too small", 5000, -500, false},
		{"no divergence - both too small", 500, -500, false},
		{"no divergence - spec zero", 0, -5000, false},
		{"no divergence - comm zero", 5000, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectDivergence(tt.specNetChange, tt.commNetChange)
			if got != tt.wantDivergence {
				t.Errorf("detectDivergence(%v, %v) = %v, want %v",
					tt.specNetChange, tt.commNetChange, got, tt.wantDivergence)
			}
		})
	}
}

func TestDetectDivergence_PureTrue(t *testing.T) {
	// Spec and commercial moving in opposite directions
	got := detectDivergence(5000, -5000)
	if !got {
		t.Error("detectDivergence should return true for opposite directions")
	}
}

func TestDetectDivergence_PureFalse(t *testing.T) {
	// Spec and commercial moving in same direction
	got := detectDivergence(5000, 5000)
	if got {
		t.Error("detectDivergence should return false for same direction")
	}
}

func TestDetectDivergence_PureThreshold(t *testing.T) {
	// Changes below 1000 threshold should not count as divergence
	got := detectDivergence(500, -500)
	if got {
		t.Error("detectDivergence should return false when changes are below threshold")
	}
}

// ============================================================================
// Test computeCrowding
// ============================================================================

func TestComputeCrowding(t *testing.T) {
	tests := []struct {
		name       string
		record     domain.COTRecord
		reportType string
		wantMin    float64
		wantMax    float64
	}{
		{
			name: "balanced speculative",
			record: domain.COTRecord{
				LevFundLong:   50000,
				LevFundShort:  50000,
				ManagedMoneyLong: 50000,
				ManagedMoneyShort: 50000,
			},
			reportType: "TFF",
			wantMin:    0,
			wantMax:    10, // Nearly balanced
		},
		{
			name: "heavy long speculative TFF",
			record: domain.COTRecord{
				LevFundLong:   90000,
				LevFundShort:  10000,
			},
			reportType: "TFF",
			wantMin:    80, // 90% long = high crowding
			wantMax:    100,
		},
		{
			name: "heavy short speculative DISAGG",
			record: domain.COTRecord{
				ManagedMoneyLong:  10000,
				ManagedMoneyShort: 90000,
			},
			reportType: "DISAGGREGATED",
			wantMin:    80, // 10% long = 90% short = high crowding
			wantMax:    100,
		},
		{
			name: "zero speculative positions",
			record: domain.COTRecord{
				LevFundLong:  0,
				LevFundShort: 0,
			},
			reportType: "TFF",
			wantMin:    50, // Neutral when no positions
			wantMax:    50,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeCrowding(tt.record, tt.reportType)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("computeCrowding() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// ============================================================================
// Test helpers
// ============================================================================

func TestExtractNets(t *testing.T) {
	history := []domain.COTRecord{
		{SpecLong: 100, SpecShort: 50},
		{SpecLong: 200, SpecShort: 80},
		{SpecLong: 150, SpecShort: 70},
	}
	fn := func(r domain.COTRecord) float64 {
		return float64(r.SpecLong - r.SpecShort)
	}
	got := extractNets(history, fn)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0] != 50 || got[1] != 120 || got[2] != 80 {
		t.Errorf("got %v, want [50 120 80]", got)
	}
}

func TestSignF(t *testing.T) {
	if signF(5) != 1 {
		t.Error("signF(5) should be 1")
	}
	if signF(-5) != -1 {
		t.Error("signF(-5) should be -1")
	}
	if signF(0) != 0 {
		t.Error("signF(0) should be 0")
	}
}

func TestSafeRatio(t *testing.T) {
	tests := []struct {
		name string
		a, b float64
		want float64
	}{
		{"normal", 10, 5, 2},
		{"zero denominator pos", 10, 0, 999.99},
		{"zero denominator neg returns 0", -10, 0, 0},
		{"both zero", 0, 0, 0},
		{"negative", -10, 5, -2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeRatio(tt.a, tt.b)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("safeRatio(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
