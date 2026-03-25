package cot

import (
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

func baseContract(reportType string) domain.COTContract {
	return domain.COTContract{Code: "099741", Currency: "EUR", ReportType: reportType}
}

// ---------------------------------------------------------------------------
// SmartMoney
// ---------------------------------------------------------------------------

func TestDetectSmartMoney(t *testing.T) {
	sd := NewSignalDetector()

	tests := []struct {
		name     string
		analysis domain.COTAnalysis
		wantNil  bool
		wantDir  string
		wantStr  int // expected strength
	}{
		{
			// COTIndexComm < 20 = dealers/commercials at extreme LOW = BEARISH signal
			// (they are net short / reducing exposure heavily)
			name: "bearish_comm_low_index_large_change",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				CommNetChange: 8000,
				COTIndexComm:  15,
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 3,
		},
		{
			// COTIndexComm > 80 = dealers/commercials at extreme HIGH = BULLISH signal
			// (they are accumulating net long positions heavily)
			name: "bullish_comm_high_index",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				CommNetChange: -12000,
				COTIndexComm:  85,
			},
			wantNil: false,
			wantDir: "BULLISH",
			wantStr: 4,
		},
		{
			// COTIndexComm = 10 (< 20) = extreme low = BEARISH
			name: "strength_5_very_large_change_bearish",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				CommNetChange: 20000,
				COTIndexComm:  10,
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 5,
		},
		{
			name: "no_signal_small_change",
			analysis: domain.COTAnalysis{
				Contract:     baseContract("TFF"),
				CommNetChange: 3000,
				COTIndexComm:  15,
			},
			wantNil: true,
		},
		{
			name: "no_signal_comm_index_not_extreme",
			analysis: domain.COTAnalysis{
				Contract:     baseContract("TFF"),
				CommNetChange: 8000,
				COTIndexComm:  50,
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signals := sd.DetectAll([]domain.COTAnalysis{tt.analysis}, nil)
			found := filterByType(signals, SignalSmartMoney)
			if tt.wantNil {
				if len(found) != 0 {
					t.Fatalf("expected no SmartMoney signal, got %d", len(found))
				}
				return
			}
			if len(found) == 0 {
				t.Fatal("expected SmartMoney signal, got none")
			}
			s := found[0]
			if s.Direction != tt.wantDir {
				t.Errorf("direction = %s, want %s", s.Direction, tt.wantDir)
			}
			if s.Strength != tt.wantStr {
				t.Errorf("strength = %d, want %d", s.Strength, tt.wantStr)
			}
			if s.Confidence <= 0 || s.Confidence > 100 {
				t.Errorf("confidence out of range: %f", s.Confidence)
			}
			if s.Type != SignalSmartMoney {
				t.Errorf("type = %s, want %s", s.Type, SignalSmartMoney)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Extreme
// ---------------------------------------------------------------------------

func TestDetectExtreme(t *testing.T) {
	sd := NewSignalDetector()

	tests := []struct {
		name    string
		analysis domain.COTAnalysis
		wantNil bool
		wantDir string
		wantStr int
	}{
		{
			name: "extreme_bull_cotindex_5_with_z_confirm",
			analysis: domain.COTAnalysis{
				Contract:    baseContract("TFF"),
				COTIndex:    5,
				WillcoIndex: 15, // |15-50|=35 > 30 → confirms
			},
			wantNil: false,
			wantDir: "BULLISH",
			wantStr: 5,
		},
		{
			name: "extreme_bull_cotindex_8_no_z_confirm",
			analysis: domain.COTAnalysis{
				Contract:    baseContract("TFF"),
				COTIndex:    8,
				WillcoIndex: 45, // |45-50|=5 < 30 → no z confirm
			},
			wantNil: false,
			wantDir: "BULLISH",
			wantStr: 3,
		},
		{
			name: "extreme_bear_cotindex_95_with_z_confirm",
			analysis: domain.COTAnalysis{
				Contract:    baseContract("TFF"),
				COTIndex:    95,
				WillcoIndex: 85, // |85-50|=35 > 30 → confirms
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 5,
		},
		{
			name: "extreme_bear_cotindex_92_no_z_confirm",
			analysis: domain.COTAnalysis{
				Contract:    baseContract("TFF"),
				COTIndex:    92,
				WillcoIndex: 55,
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 3,
		},
		{
			name: "no_signal_normal_index",
			analysis: domain.COTAnalysis{
				Contract:    baseContract("TFF"),
				COTIndex:    50,
				WillcoIndex: 50,
			},
			wantNil: true,
		},
		{
			name: "boundary_cotindex_10_no_signal",
			analysis: domain.COTAnalysis{
				Contract: baseContract("TFF"),
				COTIndex: 10.5, // > 10 and < 90 → no signal
			},
			wantNil: true,
		},
		{
			name: "boundary_cotindex_exactly_10",
			analysis: domain.COTAnalysis{
				Contract: baseContract("TFF"),
				COTIndex: 10, // <= 10 → triggers
			},
			wantNil: false,
			wantDir: "BULLISH",
		},
		{
			name: "boundary_cotindex_exactly_90",
			analysis: domain.COTAnalysis{
				Contract: baseContract("TFF"),
				COTIndex: 90, // >= 90 → triggers
			},
			wantNil: false,
			wantDir: "BEARISH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signals := sd.DetectAll([]domain.COTAnalysis{tt.analysis}, nil)
			found := filterByType(signals, SignalExtreme)
			if tt.wantNil {
				if len(found) != 0 {
					t.Fatalf("expected no Extreme signal, got %d", len(found))
				}
				return
			}
			if len(found) == 0 {
				t.Fatal("expected Extreme signal, got none")
			}
			s := found[0]
			if s.Direction != tt.wantDir {
				t.Errorf("direction = %s, want %s", s.Direction, tt.wantDir)
			}
			if tt.wantStr != 0 && s.Strength != tt.wantStr {
				t.Errorf("strength = %d, want %d", s.Strength, tt.wantStr)
			}
			if s.Confidence <= 0 || s.Confidence > 100 {
				t.Errorf("confidence out of range: %f", s.Confidence)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Divergence
// ---------------------------------------------------------------------------

func TestDetectDivergence(t *testing.T) {
	sd := NewSignalDetector()

	// Build TFF history where spec and comm move opposite for 2+ consecutive weeks.
	// history[0] = most recent; history is ordered newest first.
	divergentHistoryTFF := []domain.COTRecord{
		// [0] = current week (not used for consecutive check; analysis comes from COTAnalysis)
		{LevFundLong: 120000, LevFundShort: 80000, DealerLong: 50000, DealerShort: 90000},
		// [1] vs [2]: spec goes up, comm goes down → divergence
		{LevFundLong: 110000, LevFundShort: 80000, DealerLong: 55000, DealerShort: 85000},
		// [2] vs [3]: spec goes up, comm goes down → divergence
		{LevFundLong: 100000, LevFundShort: 80000, DealerLong: 60000, DealerShort: 80000},
		// [3] base
		{LevFundLong: 90000, LevFundShort: 80000, DealerLong: 65000, DealerShort: 75000},
	}

	noDivergenceHistory := []domain.COTRecord{
		{LevFundLong: 100000, LevFundShort: 80000, DealerLong: 60000, DealerShort: 80000},
		// [1] vs [2]: both go same direction → no divergence
		{LevFundLong: 90000, LevFundShort: 80000, DealerLong: 55000, DealerShort: 80000},
		{LevFundLong: 80000, LevFundShort: 80000, DealerLong: 50000, DealerShort: 80000},
		{LevFundLong: 70000, LevFundShort: 80000, DealerLong: 45000, DealerShort: 80000},
	}

	tests := []struct {
		name    string
		analysis domain.COTAnalysis
		history []domain.COTRecord
		wantNil bool
		wantDir string
	}{
		{
			name: "divergence_bullish_comm_buying",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				DivergenceFlag: true,
				CommNetChange:  5000,
				NetChange:     -3000,
			},
			history: divergentHistoryTFF,
			wantNil: false,
			wantDir: "BULLISH",
		},
		{
			name: "divergence_bearish_comm_selling",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				DivergenceFlag: true,
				CommNetChange:  -5000,
				NetChange:     3000,
			},
			history: divergentHistoryTFF,
			wantNil: false,
			wantDir: "BEARISH",
		},
		{
			name: "no_signal_flag_false",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				DivergenceFlag: false,
			},
			history: divergentHistoryTFF,
			wantNil: true,
		},
		{
			name: "no_signal_single_week_divergence",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				DivergenceFlag: true,
				CommNetChange:  5000,
			},
			history: noDivergenceHistory,
			wantNil: true,
		},
		{
			name: "no_signal_insufficient_history",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				DivergenceFlag: true,
				CommNetChange:  5000,
			},
			history: divergentHistoryTFF[:2], // only 2 records, need 3+
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			histMap := map[string][]domain.COTRecord{
				tt.analysis.Contract.Code: tt.history,
			}
			signals := sd.DetectAll([]domain.COTAnalysis{tt.analysis}, histMap)
			found := filterByType(signals, SignalDivergence)
			if tt.wantNil {
				if len(found) != 0 {
					t.Fatalf("expected no Divergence signal, got %d", len(found))
				}
				return
			}
			if len(found) == 0 {
				t.Fatal("expected Divergence signal, got none")
			}
			s := found[0]
			if s.Direction != tt.wantDir {
				t.Errorf("direction = %s, want %s", s.Direction, tt.wantDir)
			}
			if s.Strength < 1 || s.Strength > 5 {
				t.Errorf("strength out of range: %d", s.Strength)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MomentumShift
// ---------------------------------------------------------------------------

func TestDetectMomentumShift(t *testing.T) {
	sd := NewSignalDetector()

	// Need 5+ history records. GetSmartMoneyNet for TFF = LevFundLong - LevFundShort.
	// Momentum(data, 4) = data[last] - data[last-4]
	// We need prevMom sign != currentMom sign.
	// history[1:5] → prevNets; Momentum computes from those.
	// currentMom comes from analysis.SpecMomentum4W.

	// Build history where previous momentum was negative (decreasing net positions).
	historyFlipBullish := []domain.COTRecord{
		{LevFundLong: 120000, LevFundShort: 80000}, // [0] current week (not used by prevNets)
		{LevFundLong: 80000, LevFundShort: 80000},  // [1] net=0     (newest in prevNets)
		{LevFundLong: 85000, LevFundShort: 80000},  // [2] net=5000
		{LevFundLong: 90000, LevFundShort: 80000},  // [3] net=10000
		{LevFundLong: 95000, LevFundShort: 80000},  // [4] net=15000
		{LevFundLong: 100000, LevFundShort: 80000}, // [5] net=20000 (oldest in prevNets)
	}
	// prevNets newest-first = [0, 5000, 10000, 15000, 20000]
	// after reverseFloats oldest-first = [20000, 15000, 10000, 5000, 0]
	// Momentum(4) = 0 - 20000 = -20000 (negative prevMom)
	// For a bullish flip: currentMom must be positive.

	historyFlipBearish := []domain.COTRecord{
		{LevFundLong: 80000, LevFundShort: 100000},  // [0] current week (not used)
		{LevFundLong: 115000, LevFundShort: 80000},  // [1] net=35000 (newest in prevNets)
		{LevFundLong: 110000, LevFundShort: 80000},  // [2] net=30000
		{LevFundLong: 108000, LevFundShort: 80000},  // [3] net=28000
		{LevFundLong: 105000, LevFundShort: 80000},  // [4] net=25000
		{LevFundLong: 100000, LevFundShort: 80000},  // [5] net=20000 (oldest in prevNets)
	}
	// prevNets newest-first = [35000, 30000, 28000, 25000, 20000]
	// after reverseFloats oldest-first = [20000, 25000, 28000, 30000, 35000]
	// Momentum(4) = 35000 - 20000 = 15000 (positive prevMom)
	// For a bearish flip: currentMom must be negative.

	tests := []struct {
		name    string
		analysis domain.COTAnalysis
		history []domain.COTRecord
		wantNil bool
		wantDir string
	}{
		{
			name: "bullish_momentum_flip",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				SpecMomentum4W: 5000, // positive (flipped from negative prevMom)
			},
			history: historyFlipBullish, // prevMom = -20000 (negative)
			wantNil: false,
			wantDir: "BULLISH",
		},
		{
			name: "bearish_momentum_flip",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				SpecMomentum4W: -5000, // negative (flipped from positive prevMom)
			},
			history: historyFlipBearish, // prevMom = 15000 (positive)
			wantNil: false,
			wantDir: "BEARISH",
		},
		{
			name: "no_signal_same_sign",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				SpecMomentum4W: 5000, // positive
			},
			history: historyFlipBearish, // prevMom = 15000 (also positive, same sign)
			wantNil: true,
		},
		{
			name: "no_signal_insufficient_history",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				SpecMomentum4W: -5000,
			},
			history: historyFlipBullish[:3], // only 3 records
			wantNil: true,
		},
		{
			name: "no_signal_zero_momentum",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				SpecMomentum4W: 0,
			},
			history: historyFlipBullish,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			histMap := map[string][]domain.COTRecord{
				tt.analysis.Contract.Code: tt.history,
			}
			signals := sd.DetectAll([]domain.COTAnalysis{tt.analysis}, histMap)
			found := filterByType(signals, SignalMomentumShift)
			if tt.wantNil {
				if len(found) != 0 {
					t.Fatalf("expected no MomentumShift signal, got %d", len(found))
				}
				return
			}
			if len(found) == 0 {
				t.Fatal("expected MomentumShift signal, got none")
			}
			s := found[0]
			if s.Direction != tt.wantDir {
				t.Errorf("direction = %s, want %s", s.Direction, tt.wantDir)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Concentration
// ---------------------------------------------------------------------------

func TestDetectConcentration(t *testing.T) {
	sd := NewSignalDetector()

	tests := []struct {
		name    string
		analysis domain.COTAnalysis
		wantNil bool
		wantDir string
		wantStr int
	}{
		{
			name: "bearish_concentrated_long",
			analysis: domain.COTAnalysis{
				Contract:          baseContract("TFF"),
				Top4Concentration: 55,
				NetPosition:       10000, // positive net → BEARISH (unwind risk)
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 3, // 55 is not > 55, so strength stays at 3
		},
		{
			name: "bullish_concentrated_short",
			analysis: domain.COTAnalysis{
				Contract:          baseContract("TFF"),
				Top4Concentration: 52,
				NetPosition:       -10000, // negative net → BULLISH (squeeze risk)
			},
			wantNil: false,
			wantDir: "BULLISH",
			wantStr: 3,
		},
		{
			name: "strength_5_very_concentrated",
			analysis: domain.COTAnalysis{
				Contract:          baseContract("TFF"),
				Top4Concentration: 70,
				NetPosition:       5000,
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 5,
		},
		{
			name: "no_signal_below_threshold",
			analysis: domain.COTAnalysis{
				Contract:          baseContract("TFF"),
				Top4Concentration: 45,
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signals := sd.DetectAll([]domain.COTAnalysis{tt.analysis}, nil)
			found := filterByType(signals, SignalConcentration)
			if tt.wantNil {
				if len(found) != 0 {
					t.Fatalf("expected no Concentration signal, got %d", len(found))
				}
				return
			}
			if len(found) == 0 {
				t.Fatal("expected Concentration signal, got none")
			}
			s := found[0]
			if s.Direction != tt.wantDir {
				t.Errorf("direction = %s, want %s", s.Direction, tt.wantDir)
			}
			if s.Strength != tt.wantStr {
				t.Errorf("strength = %d, want %d", s.Strength, tt.wantStr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CrowdContrarian
// ---------------------------------------------------------------------------

func TestDetectCrowdContrarian(t *testing.T) {
	sd := NewSignalDetector()

	tests := []struct {
		name    string
		analysis domain.COTAnalysis
		wantNil bool
		wantDir string
		wantStr int
	}{
		{
			name: "bearish_crowd_long",
			analysis: domain.COTAnalysis{
				Contract:     baseContract("TFF"),
				CrowdingIndex: 76,
				NetSmallSpec:  5000, // crowd long → contrarian BEARISH
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 4,
		},
		{
			name: "bullish_crowd_short",
			analysis: domain.COTAnalysis{
				Contract:     baseContract("TFF"),
				CrowdingIndex: 80,
				NetSmallSpec:  -5000, // crowd short → contrarian BULLISH
			},
			wantNil: false,
			wantDir: "BULLISH",
			wantStr: 4,
		},
		{
			name: "strength_5_extreme_crowding",
			analysis: domain.COTAnalysis{
				Contract:     baseContract("TFF"),
				CrowdingIndex: 90,
				NetSmallSpec:  3000,
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 5,
		},
		{
			name: "strength_3_just_above_threshold",
			analysis: domain.COTAnalysis{
				Contract:     baseContract("TFF"),
				CrowdingIndex: 71,
				NetSmallSpec:  1000,
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 3,
		},
		{
			name: "no_signal_below_threshold",
			analysis: domain.COTAnalysis{
				Contract:     baseContract("TFF"),
				CrowdingIndex: 65,
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signals := sd.DetectAll([]domain.COTAnalysis{tt.analysis}, nil)
			found := filterByType(signals, SignalCrowdContrarian)
			if tt.wantNil {
				if len(found) != 0 {
					t.Fatalf("expected no CrowdContrarian signal, got %d", len(found))
				}
				return
			}
			if len(found) == 0 {
				t.Fatal("expected CrowdContrarian signal, got none")
			}
			s := found[0]
			if s.Direction != tt.wantDir {
				t.Errorf("direction = %s, want %s", s.Direction, tt.wantDir)
			}
			if s.Strength != tt.wantStr {
				t.Errorf("strength = %d, want %d", s.Strength, tt.wantStr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ThinMarket
// ---------------------------------------------------------------------------

func TestDetectThinMarket(t *testing.T) {
	sd := NewSignalDetector()

	tests := []struct {
		name    string
		analysis domain.COTAnalysis
		wantNil bool
		wantDir string
		wantStr int
	}{
		{
			name: "bearish_thin_market_long_position",
			analysis: domain.COTAnalysis{
				Contract:           baseContract("TFF"),
				ThinMarketAlert:    true,
				ThinMarketDesc:     "Only 8 dealers short EUR",
				NetPosition:        5000, // positive → BEARISH
				TotalTraders:       30,
				DealerShortTraders: 8,
				LevFundLongTraders: 15,
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 5, // minTraders=8 < 10 → strength 5
		},
		{
			name: "bullish_thin_market_short_position",
			analysis: domain.COTAnalysis{
				Contract:           baseContract("TFF"),
				ThinMarketAlert:    true,
				ThinMarketDesc:     "Only 11 lev fund long EUR",
				NetPosition:        -5000, // negative → BULLISH
				TotalTraders:       30,
				DealerShortTraders: 20,
				LevFundLongTraders: 11,
			},
			wantNil: false,
			wantDir: "BULLISH",
			wantStr: 4, // minTraders=11 < 12 → strength 4
		},
		{
			name: "thin_market_disaggregated",
			analysis: domain.COTAnalysis{
				Contract:       domain.COTContract{Code: "088691", Currency: "XAU", ReportType: "DISAGGREGATED"},
				ThinMarketAlert: true,
				ThinMarketDesc:  "Only 7 managed money long XAU",
				NetPosition:     10000,
				TotalTraders:    25,
				MMoneyLongTraders:  7,
				MMoneyShortTraders: 20,
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 5, // minTraders=7 < 10
		},
		{
			name: "no_signal_alert_false",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				ThinMarketAlert: false,
				ThinMarketDesc:  "some desc",
			},
			wantNil: true,
		},
		{
			name: "no_signal_empty_desc",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				ThinMarketAlert: true,
				ThinMarketDesc:  "",
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signals := sd.DetectAll([]domain.COTAnalysis{tt.analysis}, nil)
			found := filterByType(signals, SignalThinMarket)
			if tt.wantNil {
				if len(found) != 0 {
					t.Fatalf("expected no ThinMarket signal, got %d", len(found))
				}
				return
			}
			if len(found) == 0 {
				t.Fatal("expected ThinMarket signal, got none")
			}
			s := found[0]
			if s.Direction != tt.wantDir {
				t.Errorf("direction = %s, want %s", s.Direction, tt.wantDir)
			}
			if s.Strength != tt.wantStr {
				t.Errorf("strength = %d, want %d", s.Strength, tt.wantStr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// sortSignals
// ---------------------------------------------------------------------------

func TestSortSignals(t *testing.T) {
	signals := []Signal{
		{Currency: "A", Strength: 2, Confidence: 60},
		{Currency: "B", Strength: 5, Confidence: 80},
		{Currency: "C", Strength: 5, Confidence: 90},
		{Currency: "D", Strength: 3, Confidence: 70},
	}
	sortSignals(signals)

	// Expected order: C (5,90), B (5,80), D (3,70), A (2,60)
	expected := []string{"C", "B", "D", "A"}
	for i, exp := range expected {
		if signals[i].Currency != exp {
			t.Errorf("position %d: got %s, want %s", i, signals[i].Currency, exp)
		}
	}
}

// ---------------------------------------------------------------------------
// DetectAll — Multiple signals and empty input
// ---------------------------------------------------------------------------

func TestDetectAll_EmptyInput(t *testing.T) {
	sd := NewSignalDetector()
	signals := sd.DetectAll(nil, nil)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for nil input, got %d", len(signals))
	}
	signals = sd.DetectAll([]domain.COTAnalysis{}, nil)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for empty input, got %d", len(signals))
	}
}

func TestDetectAll_MultipleSignals(t *testing.T) {
	sd := NewSignalDetector()

	// An analysis that triggers both Extreme and CrowdContrarian.
	a := domain.COTAnalysis{
		Contract:      baseContract("TFF"),
		COTIndex:      5,      // triggers Extreme
		WillcoIndex:   15,
		CrowdingIndex: 80,     // triggers CrowdContrarian
		NetSmallSpec:  -3000,
	}

	signals := sd.DetectAll([]domain.COTAnalysis{a}, nil)
	if len(signals) < 2 {
		t.Fatalf("expected at least 2 signals, got %d", len(signals))
	}

	types := make(map[SignalType]bool)
	for _, s := range signals {
		types[s.Type] = true
	}
	if !types[SignalExtreme] {
		t.Error("expected Extreme signal")
	}
	if !types[SignalCrowdContrarian] {
		t.Error("expected CrowdContrarian signal")
	}

	// Verify sorting: highest strength first
	for i := 1; i < len(signals); i++ {
		if signals[i].Strength > signals[i-1].Strength {
			t.Errorf("signals not sorted by strength: [%d]=%d > [%d]=%d",
				i, signals[i].Strength, i-1, signals[i-1].Strength)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func filterByType(signals []Signal, typ SignalType) []Signal {
	var out []Signal
	for _, s := range signals {
		if s.Type == typ {
			out = append(out, s)
		}
	}
	return out
}
