package cot

import (
	"testing"
	"time"

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
		wantStr  int
	}{
		{
			// TFF: COTIndexComm < 20 = dealers forced short by client buying = BULLISH
			name: "bullish_comm_low_index_tff",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				CommNetChange: 8000,
				COTIndexComm:  15,
				OpenInterest:  200000, // changePct = 8000/200000*100 = 4% → strength 3 (not > 4.0)
			},
			wantNil: false,
			wantDir: "BULLISH",
			wantStr: 3, // changePct = 4.0, threshold is > 4.0 for strength 4
		},
		{
			// TFF: COTIndexComm > 80 = dealers forced long by client selling = BEARISH
			name: "bearish_comm_high_index_tff",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				CommNetChange: -12000,
				COTIndexComm:  85,
				OpenInterest:  200000, // changePct = 12000/200000*100 = 6% > 4%
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 4,
		},
		{
			// DISAGGREGATED: COTIndexComm < 20 = heavy hedging = BEARISH
			name: "bearish_comm_low_index_disagg",
			analysis: domain.COTAnalysis{
				Contract:      domain.COTContract{Code: "088691", Currency: "XAU", ReportType: "DISAGGREGATED"},
				CommNetChange: 8000,
				COTIndexComm:  10,
				OpenInterest:  100000, // changePct = 8000/100000*100 = 8% > 6%
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 5,
		},
		{
			// DISAGGREGATED: COTIndexComm > 80 = less hedging = BULLISH
			name: "bullish_comm_high_index_disagg",
			analysis: domain.COTAnalysis{
				Contract:      domain.COTContract{Code: "088691", Currency: "XAU", ReportType: "DISAGGREGATED"},
				CommNetChange: -10000,
				COTIndexComm:  85,
				OpenInterest:  200000,
			},
			wantNil: false,
			wantDir: "BULLISH",
			wantStr: 4,
		},
		{
			// changePct < 2% → no signal (OI-relative threshold)
			name: "no_signal_small_change_pct",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				CommNetChange: 3000,
				COTIndexComm:  15,
				OpenInterest:  300000, // changePct = 3000/300000*100 = 1% < 2%
			},
			wantNil: true,
		},
		{
			name: "no_signal_comm_index_not_extreme",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				CommNetChange: 8000,
				COTIndexComm:  50,
				OpenInterest:  200000,
			},
			wantNil: true,
		},
		{
			// Strength 5: changePct > 6%
			name: "strength_5_very_large_change",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				CommNetChange: 20000,
				COTIndexComm:  10,
				OpenInterest:  200000, // changePct = 20000/200000*100 = 10% > 6%
			},
			wantNil: false,
			wantDir: "BULLISH", // TFF + COTIndexComm < 20 → BULLISH
			wantStr: 5,
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
// Extreme — requires reverting confirmation from history
// ---------------------------------------------------------------------------

func TestDetectExtreme(t *testing.T) {
	sd := NewSignalDetector()

	// Build history for reverting bullish extreme (COTIndex <= 10):
	// Previous week prevIdx must be <= 10 AND more extreme (lower) than current.
	// GetSmartMoneyNet for TFF = LevFundLong - LevFundShort.
	// computeCOTIndex calculates percentile of the last value within the series.

	// For COTIndex=8 reverting from prevIdx ~5:
	// We need history where the previous week's net was at a lower percentile.
	// history[0] = current week (ignored by prevIdx calc),
	// history[1:] = previous weeks used for prevIdx.
	revertingBullHistory := buildRevertingHistory(5.0, 8.0, "TFF")
	revertingBearHistory := buildRevertingHistory(95.0, 92.0, "TFF")

	tests := []struct {
		name     string
		analysis domain.COTAnalysis
		history  []domain.COTRecord
		wantNil  bool
		wantDir  string
		wantStr  int
	}{
		{
			name: "extreme_bull_reverting_with_z_confirm",
			analysis: domain.COTAnalysis{
				Contract:    baseContract("TFF"),
				COTIndex:    8,
				WillcoIndex: 15, // |15-50|=35 > 30 → confirms
			},
			history: revertingBullHistory,
			wantNil: false,
			wantDir: "BULLISH",
			wantStr: 3,
		},
		{
			name: "extreme_bear_reverting_with_z_confirm",
			analysis: domain.COTAnalysis{
				Contract:    baseContract("TFF"),
				COTIndex:    92,
				WillcoIndex: 85, // |85-50|=35 > 30 → confirms
			},
			history: revertingBearHistory,
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
			history: revertingBullHistory,
			wantNil: true,
		},
		{
			name: "no_signal_no_history",
			analysis: domain.COTAnalysis{
				Contract: baseContract("TFF"),
				COTIndex: 5,
			},
			history: nil,
			wantNil: true,
		},
		{
			// Not reverting: still building extreme (no previous extreme week)
			// Use COTIndex=5, but provide history where prevIdx > 10 (not extreme)
			name: "no_signal_not_reverting",
			analysis: domain.COTAnalysis{
				Contract: baseContract("TFF"),
				COTIndex: 5,
			},
			history: buildNonRevertingHistory("TFF"), // prevIdx is ~50 (not extreme)
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			histMap := map[string][]domain.COTRecord{
				tt.analysis.Contract.Code: tt.history,
			}
			signals := sd.DetectAll([]domain.COTAnalysis{tt.analysis}, histMap)
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

// buildRevertingHistory creates a newest-first history where prevIdx is at
// `prevTarget` percentile and current week's net position is at `currTarget` percentile.
// This is used to test the reverting confirmation logic in detectExtreme.
func buildRevertingHistory(prevTarget, currTarget float64, reportType string) []domain.COTRecord {
	// For TFF, GetSmartMoneyNet = LevFundLong - LevFundShort.
	// computeCOTIndex = percentile(prevNets).
	// We need history[1:] to produce prevIdx = prevTarget.
	//
	// Simple approach: create a linearly spaced series where the most recent
	// value in history[1:] (= history[1]) is at `prevTarget` percentile.
	// Use 10 records so percentile calculation is stable.
	n := 10
	records := make([]domain.COTRecord, n+1) // [0]=current, [1:]=history

	// Base series: spread from 0 to 100000 in equal steps
	for i := 1; i <= n; i++ {
		net := float64(i-1) * 10000 // 0, 10000, 20000, ..., 90000
		records[i] = domain.COTRecord{
			LevFundLong:  100000 + net,
			LevFundShort: 100000,
		}
	}

	// Place history[1] (the most recent previous week) at the prevTarget percentile.
	// Percentile is roughly (rank / (n-1)) * 100 for a sorted series.
	prevNet := prevTarget / 100.0 * 90000 // scale to our 0-90000 range
	records[1] = domain.COTRecord{
		LevFundLong:  100000 + prevNet,
		LevFundShort: 100000,
	}

	// history[0] = current week — not used for prevIdx, just for completeness
	currNet := currTarget / 100.0 * 90000
	records[0] = domain.COTRecord{
		LevFundLong:  100000 + currNet,
		LevFundShort: 100000,
	}

	return records
}

// buildNonRevertingHistory creates a history where prevIdx is NOT at an extreme
// (around 50th percentile), so the reverting check fails.
func buildNonRevertingHistory(reportType string) []domain.COTRecord {
	n := 10
	records := make([]domain.COTRecord, n+1)
	for i := 1; i <= n; i++ {
		net := float64(i-1) * 10000
		records[i] = domain.COTRecord{
			LevFundLong:  100000 + net,
			LevFundShort: 100000,
		}
	}
	// history[1] = previous week at ~50th percentile (middle of range)
	records[1] = domain.COTRecord{
		LevFundLong:  100000 + 45000,
		LevFundShort: 100000,
	}
	// history[0] = current week (irrelevant for prevIdx)
	records[0] = domain.COTRecord{
		LevFundLong:  100000 + 2000,
		LevFundShort: 100000,
	}
	return records
}

// ---------------------------------------------------------------------------
// Divergence
// ---------------------------------------------------------------------------

func TestDetectDivergence(t *testing.T) {
	sd := NewSignalDetector()

	// Build TFF history where spec and comm move opposite for 3+ consecutive weeks.
	// history[0] = most recent; history is ordered newest first.
	// Divergence persistence check: history[i] vs history[i+1] for i=1..3.
	// Need spec going one way, comm going opposite with >0.5% of OI magnitude.
	divergentHistoryTFF := []domain.COTRecord{
		// [0] = current week (not used for consecutive check)
		{LevFundLong: 130000, LevFundShort: 80000, DealerLong: 45000, DealerShort: 95000},
		// [1] vs [2]: spec net goes up (110k-80k=30k → 100k-80k=20k), comm net goes down
		{LevFundLong: 110000, LevFundShort: 80000, DealerLong: 55000, DealerShort: 85000},
		// [2] vs [3]: spec goes up, comm goes down
		{LevFundLong: 100000, LevFundShort: 80000, DealerLong: 60000, DealerShort: 80000},
		// [3] vs [4]: spec goes up, comm goes down
		{LevFundLong: 90000, LevFundShort: 80000, DealerLong: 65000, DealerShort: 75000},
		// [4] base
		{LevFundLong: 80000, LevFundShort: 80000, DealerLong: 70000, DealerShort: 70000},
	}

	noDivergenceHistory := []domain.COTRecord{
		{LevFundLong: 100000, LevFundShort: 80000, DealerLong: 60000, DealerShort: 80000},
		// [1] vs [2]: both go same direction → no divergence
		{LevFundLong: 90000, LevFundShort: 80000, DealerLong: 55000, DealerShort: 80000},
		{LevFundLong: 80000, LevFundShort: 80000, DealerLong: 50000, DealerShort: 80000},
		{LevFundLong: 70000, LevFundShort: 80000, DealerLong: 45000, DealerShort: 80000},
		{LevFundLong: 60000, LevFundShort: 80000, DealerLong: 40000, DealerShort: 80000},
	}

	tests := []struct {
		name     string
		analysis domain.COTAnalysis
		history  []domain.COTRecord
		wantNil  bool
		wantDir  string
	}{
		{
			name: "divergence_bullish_comm_buying",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				DivergenceFlag: true,
				CommNetChange:  5000,
				NetChange:      -3000,
				OpenInterest:   200000,
			},
			history: divergentHistoryTFF,
			wantNil: false,
			wantDir: "BULLISH",
		},
		{
			name: "divergence_bearish_comm_selling",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				DivergenceFlag: true,
				CommNetChange:  -5000,
				NetChange:      3000,
				OpenInterest:   200000,
			},
			history: divergentHistoryTFF,
			wantNil: false,
			wantDir: "BEARISH",
		},
		{
			name: "no_signal_flag_false",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				DivergenceFlag: false,
			},
			history: divergentHistoryTFF,
			wantNil: true,
		},
		{
			name: "no_signal_no_persistent_divergence",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				DivergenceFlag: true,
				CommNetChange:  5000,
				OpenInterest:   200000,
			},
			history: noDivergenceHistory,
			wantNil: true,
		},
		{
			name: "no_signal_insufficient_history",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
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
// MomentumShift — requires 8W momentum confirmation
// ---------------------------------------------------------------------------

func TestDetectMomentumShift(t *testing.T) {
	sd := NewSignalDetector()

	// Need 6+ history records for prevMom calculation.
	// prevNets = history[1:5] → extracted, reversed to oldest-first, Momentum(4) computed.
	historyFlipBullish := []domain.COTRecord{
		{LevFundLong: 120000, LevFundShort: 80000}, // [0] current (not used for prevNets)
		{LevFundLong: 80000, LevFundShort: 80000},  // [1] net=0
		{LevFundLong: 85000, LevFundShort: 80000},  // [2] net=5000
		{LevFundLong: 90000, LevFundShort: 80000},  // [3] net=10000
		{LevFundLong: 95000, LevFundShort: 80000},  // [4] net=15000
		{LevFundLong: 100000, LevFundShort: 80000}, // [5] net=20000
	}
	// prevNets newest-first: [0, 5000, 10000, 15000, 20000]
	// reversed oldest-first: [20000, 15000, 10000, 5000, 0]
	// Momentum(4) = 0 - 20000 = -20000 (negative prevMom)

	historyFlipBearish := []domain.COTRecord{
		{LevFundLong: 80000, LevFundShort: 100000}, // [0]
		{LevFundLong: 115000, LevFundShort: 80000}, // [1] net=35000
		{LevFundLong: 110000, LevFundShort: 80000}, // [2] net=30000
		{LevFundLong: 108000, LevFundShort: 80000}, // [3] net=28000
		{LevFundLong: 105000, LevFundShort: 80000}, // [4] net=25000
		{LevFundLong: 100000, LevFundShort: 80000}, // [5] net=20000
	}
	// prevNets: [35000, 30000, 28000, 25000, 20000] → reversed: [20000, 25000, 28000, 30000, 35000]
	// Momentum(4) = 35000 - 20000 = 15000 (positive prevMom)

	tests := []struct {
		name     string
		analysis domain.COTAnalysis
		history  []domain.COTRecord
		wantNil  bool
		wantDir  string
	}{
		{
			name: "bullish_momentum_flip",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				SpecMomentum4W: 5000,   // positive (flipped from negative prevMom)
				SpecMomentum8W: 3000,   // same sign as 4W → confirms
				OpenInterest:   200000, // magnitude check: |5000-(-20000)|=25000, 25000/200000*100=12.5% > 1%
			},
			history: historyFlipBullish,
			wantNil: false,
			wantDir: "BULLISH",
		},
		{
			name: "bearish_momentum_flip",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				SpecMomentum4W: -5000, // negative (flipped from positive prevMom)
				SpecMomentum8W: -3000, // same sign → confirms
				OpenInterest:   200000,
			},
			history: historyFlipBearish,
			wantNil: false,
			wantDir: "BEARISH",
		},
		{
			name: "no_signal_same_sign",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				SpecMomentum4W: 5000,
				SpecMomentum8W: 3000,
				OpenInterest:   200000,
			},
			history: historyFlipBearish, // prevMom positive, currentMom positive → same sign
			wantNil: true,
		},
		{
			name: "no_signal_8w_disagrees",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				SpecMomentum4W: 5000,  // positive
				SpecMomentum8W: -3000, // negative → disagrees with 4W
				OpenInterest:   200000,
			},
			history: historyFlipBullish,
			wantNil: true,
		},
		{
			name: "no_signal_insufficient_history",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				SpecMomentum4W: -5000,
				SpecMomentum8W: -3000,
			},
			history: historyFlipBullish[:3],
			wantNil: true,
		},
		{
			name: "no_signal_zero_momentum",
			analysis: domain.COTAnalysis{
				Contract:       baseContract("TFF"),
				SpecMomentum4W: 0,
				SpecMomentum8W: 0,
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
// Concentration — threshold is Top4Concentration >= 55
// ---------------------------------------------------------------------------

func TestDetectConcentration(t *testing.T) {
	sd := NewSignalDetector()

	tests := []struct {
		name     string
		analysis domain.COTAnalysis
		wantNil  bool
		wantDir  string
		wantStr  int
	}{
		{
			// Top4LongPct > Top4ShortPct → concentrated longs → BEARISH (unwind risk)
			name: "bearish_concentrated_long",
			analysis: domain.COTAnalysis{
				Contract:          baseContract("TFF"),
				Top4Concentration: 58,
				Top4LongPct:       35,
				Top4ShortPct:      23,
				NetPosition:       10000,
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 4, // > 55 → strength 4
		},
		{
			// Top4ShortPct > Top4LongPct → concentrated shorts → BULLISH (squeeze risk)
			name: "bullish_concentrated_short",
			analysis: domain.COTAnalysis{
				Contract:          baseContract("TFF"),
				Top4Concentration: 60,
				Top4LongPct:       20,
				Top4ShortPct:      40,
				NetPosition:       -10000,
			},
			wantNil: false,
			wantDir: "BULLISH",
			wantStr: 4,
		},
		{
			name: "strength_5_very_concentrated",
			analysis: domain.COTAnalysis{
				Contract:          baseContract("TFF"),
				Top4Concentration: 70,
				Top4LongPct:       45,
				Top4ShortPct:      25,
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
// CrowdContrarian — uses NetPosition for direction (not NetSmallSpec)
// ---------------------------------------------------------------------------

func TestDetectCrowdContrarian(t *testing.T) {
	sd := NewSignalDetector()

	tests := []struct {
		name     string
		analysis domain.COTAnalysis
		wantNil  bool
		wantDir  string
		wantStr  int
	}{
		{
			// Large specs crowded long (NetPosition > 0) → contrarian BEARISH
			name: "bearish_crowd_long",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				CrowdingIndex: 76,
				NetPosition:   5000, // large spec net long → BEARISH
				NetSmallSpec:  5000,
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 4,
		},
		{
			// Large specs crowded short (NetPosition < 0) → contrarian BULLISH
			name: "bullish_crowd_short",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				CrowdingIndex: 80,
				NetPosition:   -5000, // large spec net short → BULLISH
				NetSmallSpec:  -5000,
			},
			wantNil: false,
			wantDir: "BULLISH",
			wantStr: 4,
		},
		{
			name: "strength_5_extreme_crowding",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				CrowdingIndex: 90,
				NetPosition:   3000,
				NetSmallSpec:  3000,
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 5,
		},
		{
			name: "strength_3_just_above_threshold",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
				CrowdingIndex: 71,
				NetPosition:   1000,
				NetSmallSpec:  1000,
			},
			wantNil: false,
			wantDir: "BEARISH",
			wantStr: 3,
		},
		{
			name: "no_signal_below_threshold",
			analysis: domain.COTAnalysis{
				Contract:      baseContract("TFF"),
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
// ThinMarket — direction from which SIDE is thin, not from NetPosition
// ---------------------------------------------------------------------------

func TestDetectThinMarket(t *testing.T) {
	sd := NewSignalDetector()

	tests := []struct {
		name     string
		analysis domain.COTAnalysis
		wantNil  bool
		wantDir  string
		wantStr  int
	}{
		{
			// TFF: DealerShortTraders=8 < 10 → thin shorts → BULLISH (squeeze risk)
			name: "bullish_thin_dealer_shorts",
			analysis: domain.COTAnalysis{
				Contract:           baseContract("TFF"),
				ThinMarketAlert:    true,
				ThinMarketDesc:     "Only 8 dealers short EUR",
				NetPosition:        5000,
				TotalTraders:       30,
				DealerShortTraders: 8,
				LevFundLongTraders: 15,
			},
			wantNil: false,
			wantDir: "BULLISH", // thin shorts → squeeze risk → BULLISH
			wantStr: 4,         // minTraders=8, >= 7 → strength 4
		},
		{
			// TFF: LevFundLongTraders=5 < 10 → thin longs → BEARISH (unwind risk)
			name: "bearish_thin_levfund_longs",
			analysis: domain.COTAnalysis{
				Contract:           baseContract("TFF"),
				ThinMarketAlert:    true,
				ThinMarketDesc:     "Only 5 lev fund long EUR",
				NetPosition:        -5000,
				TotalTraders:       30,
				DealerShortTraders: 20,
				LevFundLongTraders: 5,
			},
			wantNil: false,
			wantDir: "BEARISH", // thin longs → unwind risk → BEARISH
			wantStr: 5,         // minTraders=5, < 7 → strength 5
		},
		{
			// DISAGGREGATED: MMoneyLongTraders=7 < 10 → thin longs → BEARISH
			name: "thin_market_disaggregated",
			analysis: domain.COTAnalysis{
				Contract:           domain.COTContract{Code: "088691", Currency: "XAU", ReportType: "DISAGGREGATED"},
				ThinMarketAlert:    true,
				ThinMarketDesc:     "Only 7 managed money long XAU",
				NetPosition:        10000,
				TotalTraders:       25,
				MMoneyLongTraders:  7,
				MMoneyShortTraders: 20,
			},
			wantNil: false,
			wantDir: "BEARISH", // thin longs → BEARISH
			wantStr: 4,         // minTraders=7, >= 7 → strength 4
		},
		{
			// No signal: all trader counts >= 10
			name: "no_signal_all_thick",
			analysis: domain.COTAnalysis{
				Contract:            baseContract("TFF"),
				ThinMarketAlert:     true,
				ThinMarketDesc:      "some desc",
				TotalTraders:        50,
				DealerShortTraders:  15,
				LevFundLongTraders:  20,
				LevFundShortTraders: 15,
			},
			wantNil: true,
		},
		{
			name: "no_signal_alert_false",
			analysis: domain.COTAnalysis{
				Contract:        baseContract("TFF"),
				ThinMarketAlert: false,
				ThinMarketDesc:  "some desc",
			},
			wantNil: true,
		},
		{
			name: "no_signal_empty_desc",
			analysis: domain.COTAnalysis{
				Contract:        baseContract("TFF"),
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

	// An analysis that triggers both SmartMoney and CrowdContrarian.
	// SmartMoney: COTIndexComm < 20 + large OI-relative change.
	// CrowdContrarian: CrowdingIndex >= 70 + NetPosition set.
	a := domain.COTAnalysis{
		Contract:      baseContract("TFF"),
		COTIndexComm:  15, // triggers SmartMoney (< 20)
		CommNetChange: 8000,
		OpenInterest:  200000, // changePct = 4% > 2%
		CrowdingIndex: 80,     // triggers CrowdContrarian
		NetPosition:   -3000,  // BULLISH contrarian
	}

	signals := sd.DetectAll([]domain.COTAnalysis{a}, nil)
	if len(signals) < 2 {
		t.Fatalf("expected at least 2 signals, got %d", len(signals))
	}

	types := make(map[SignalType]bool)
	for _, s := range signals {
		types[s.Type] = true
	}
	if !types[SignalSmartMoney] {
		t.Error("expected SmartMoney signal")
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

// suppress unused import warning
var _ = time.Now
