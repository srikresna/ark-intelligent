package backtest

import (
	"context"
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// MatrixCell holds performance metrics for a single signalType x regime combination.
type MatrixCell struct {
	WinRate    float64 `json:"win_rate"`
	AvgReturn  float64 `json:"avg_return"`
	SampleSize int     `json:"sample_size"`
}

// CondMatrix holds the full conditional performance matrix.
type CondMatrix struct {
	Cells        map[string]map[string]*MatrixCell `json:"cells"` // [signalType][regime]
	BestCombo    string                            `json:"best_combo"`
	BestWinRate  float64                           `json:"best_win_rate"`
	WorstCombo   string                            `json:"worst_combo"`
	WorstWinRate float64                           `json:"worst_win_rate"`
}

// MatrixAnalyzer builds a conditional performance matrix across signal types and regimes.
type MatrixAnalyzer struct {
	signalRepo ports.SignalRepository
}

// NewMatrixAnalyzer creates a new matrix analyzer.
func NewMatrixAnalyzer(signalRepo ports.SignalRepository) *MatrixAnalyzer {
	return &MatrixAnalyzer{signalRepo: signalRepo}
}

// matrixAccumulator tracks running totals for a single cell.
type matrixAccumulator struct {
	wins      int
	total     int
	sumReturn float64
}

// Analyze builds the signalType x FREDRegime performance matrix.
func (ma *MatrixAnalyzer) Analyze(ctx context.Context) (*CondMatrix, error) {
	signals, err := ma.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}

	// Accumulate per cell.
	accum := make(map[string]map[string]*matrixAccumulator)

	for _, s := range signals {
		// Filter to evaluated signals only.
		if s.Outcome1W != domain.OutcomeWin && s.Outcome1W != domain.OutcomeLoss {
			continue
		}

		regime := s.FREDRegime
		if regime == "" {
			regime = "UNKNOWN"
		}

		if accum[s.SignalType] == nil {
			accum[s.SignalType] = make(map[string]*matrixAccumulator)
		}
		cell := accum[s.SignalType][regime]
		if cell == nil {
			cell = &matrixAccumulator{}
			accum[s.SignalType][regime] = cell
		}

		cell.total++
		cell.sumReturn += s.Return1W
		if s.Outcome1W == domain.OutcomeWin {
			cell.wins++
		}
	}

	// Build result matrix.
	result := &CondMatrix{
		Cells: make(map[string]map[string]*MatrixCell, len(accum)),
	}

	bestWR := -1.0
	worstWR := 101.0

	for sigType, regimes := range accum {
		result.Cells[sigType] = make(map[string]*MatrixCell, len(regimes))

		for regime, acc := range regimes {
			if acc.total == 0 {
				continue
			}

			cell := &MatrixCell{
				WinRate:    round2(float64(acc.wins) / float64(acc.total) * 100),
				AvgReturn:  round4(acc.sumReturn / float64(acc.total)),
				SampleSize: acc.total,
			}
			result.Cells[sigType][regime] = cell

			// Track best/worst combos (minimum n=10).
			if acc.total >= 10 {
				combo := fmt.Sprintf("%s/%s", sigType, regime)
				if cell.WinRate > bestWR {
					bestWR = cell.WinRate
					result.BestCombo = combo
					result.BestWinRate = cell.WinRate
				}
				if cell.WinRate < worstWR {
					worstWR = cell.WinRate
					result.WorstCombo = combo
					result.WorstWinRate = cell.WinRate
				}
			}
		}
	}

	return result, nil
}
