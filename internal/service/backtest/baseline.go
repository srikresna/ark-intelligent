package backtest

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// BaselineResult holds the comparison between the actual system performance
// and a random baseline computed via Monte Carlo simulation.
type BaselineResult struct {
	RandomWinRate1W float64 `json:"random_win_rate_1w"`
	RandomWinRate2W float64 `json:"random_win_rate_2w"`
	RandomWinRate4W float64 `json:"random_win_rate_4w"`
	RandomAvgReturn float64 `json:"random_avg_return"`
	SystemEdge1W    float64 `json:"system_edge_1w"`
	SystemEdge2W    float64 `json:"system_edge_2w"`
	SystemEdge4W    float64 `json:"system_edge_4w"`
	SystemWinRate1W float64 `json:"system_win_rate_1w"`
	SystemWinRate2W float64 `json:"system_win_rate_2w"`
	SystemWinRate4W float64 `json:"system_win_rate_4w"`
	SystemEV        float64 `json:"system_ev"`
	RandomEV        float64 `json:"random_ev"`
	EVEdge          float64 `json:"ev_edge"`
	NumSimulations  int     `json:"num_simulations"`
}

// BaselineGenerator computes random-strategy baselines to quantify system edge.
type BaselineGenerator struct {
	signalRepo ports.SignalRepository
}

// NewBaselineGenerator creates a new BaselineGenerator.
func NewBaselineGenerator(signalRepo ports.SignalRepository) *BaselineGenerator {
	return &BaselineGenerator{signalRepo: signalRepo}
}

// ComputeBaseline runs Monte Carlo simulations to estimate what a random
// direction strategy would achieve, then compares against actual system results.
func (bg *BaselineGenerator) ComputeBaseline(ctx context.Context, numSims int) (*BaselineResult, error) {
	allSignals, err := bg.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}

	// Filter to evaluated signals (non-empty Outcome1W, not PENDING or EXPIRED).
	var signals []domain.PersistedSignal
	for _, s := range allSignals {
		if s.Outcome1W != "" && s.Outcome1W != domain.OutcomePending && s.Outcome1W != domain.OutcomeExpired {
			signals = append(signals, s)
		}
	}

	if len(signals) == 0 {
		return &BaselineResult{NumSimulations: numSims}, nil
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Accumulators for random baseline across simulations.
	var sumRandWin1W, sumRandWin2W, sumRandWin4W float64
	var sumRandReturn float64

	for sim := 0; sim < numSims; sim++ {
		var wins1W, wins2W, wins4W int
		var eval1W, eval2W, eval4W int
		var totalReturn float64

		for _, s := range signals {
			// Random direction: 50/50 BULLISH or BEARISH.
			randDir := "BULLISH"
			if rng.Intn(2) == 0 {
				randDir = "BEARISH"
			}

			// 1W
			if s.Outcome1W == domain.OutcomeWin || s.Outcome1W == domain.OutcomeLoss {
				eval1W++
				if classifyRandomOutcome(randDir, s.Return1W) == domain.OutcomeWin {
					wins1W++
				}
				totalReturn += classifyRandomReturn(randDir, s.Return1W)
			}

			// 2W
			if s.Outcome2W == domain.OutcomeWin || s.Outcome2W == domain.OutcomeLoss {
				eval2W++
				if classifyRandomOutcome(randDir, s.Return2W) == domain.OutcomeWin {
					wins2W++
				}
			}

			// 4W
			if s.Outcome4W == domain.OutcomeWin || s.Outcome4W == domain.OutcomeLoss {
				eval4W++
				if classifyRandomOutcome(randDir, s.Return4W) == domain.OutcomeWin {
					wins4W++
				}
			}
		}

		if eval1W > 0 {
			sumRandWin1W += float64(wins1W) / float64(eval1W) * 100
			sumRandReturn += totalReturn / float64(eval1W)
		}
		if eval2W > 0 {
			sumRandWin2W += float64(wins2W) / float64(eval2W) * 100
		}
		if eval4W > 0 {
			sumRandWin4W += float64(wins4W) / float64(eval4W) * 100
		}
	}

	n := float64(numSims)
	randWR1W := round2(sumRandWin1W / n)
	randWR2W := round2(sumRandWin2W / n)
	randWR4W := round2(sumRandWin4W / n)
	randAvgRet := round4(sumRandReturn / n)

	// System stats from actual signals.
	sysWR1W, sysWR2W, sysWR4W, sysEV := computeSystemStats(signals)

	// Random EV: average return per trade under random direction.
	randomEV := randAvgRet

	return &BaselineResult{
		RandomWinRate1W: randWR1W,
		RandomWinRate2W: randWR2W,
		RandomWinRate4W: randWR4W,
		RandomAvgReturn: randAvgRet,
		SystemEdge1W:    round2(sysWR1W - randWR1W),
		SystemEdge2W:    round2(sysWR2W - randWR2W),
		SystemEdge4W:    round2(sysWR4W - randWR4W),
		SystemWinRate1W: sysWR1W,
		SystemWinRate2W: sysWR2W,
		SystemWinRate4W: sysWR4W,
		SystemEV:        sysEV,
		RandomEV:        randomEV,
		EVEdge:          round4(sysEV - randomEV),
		NumSimulations:  numSims,
	}, nil
}

// classifyRandomOutcome determines whether a signal would be a WIN or LOSS
// given a randomly assigned direction and the actual return.
func classifyRandomOutcome(direction string, ret float64) string {
	if direction == "BULLISH" && ret > 0 {
		return domain.OutcomeWin
	}
	if direction == "BEARISH" && ret < 0 {
		return domain.OutcomeWin
	}
	return domain.OutcomeLoss
}

// classifyRandomReturn returns the effective return for a random direction.
// BULLISH uses the return as-is; BEARISH inverts it.
func classifyRandomReturn(direction string, ret float64) float64 {
	if direction == "BEARISH" {
		return -ret
	}
	return ret
}

// computeSystemStats computes win rates and EV from the actual signal outcomes.
func computeSystemStats(signals []domain.PersistedSignal) (wr1W, wr2W, wr4W, ev float64) {
	var wins1W, eval1W int
	var wins2W, eval2W int
	var wins4W, eval4W int
	var sumReturn float64

	for _, s := range signals {
		if s.Outcome1W == domain.OutcomeWin || s.Outcome1W == domain.OutcomeLoss {
			eval1W++
			sumReturn += s.Return1W
			if s.Outcome1W == domain.OutcomeWin {
				wins1W++
			}
		}
		if s.Outcome2W == domain.OutcomeWin || s.Outcome2W == domain.OutcomeLoss {
			eval2W++
			if s.Outcome2W == domain.OutcomeWin {
				wins2W++
			}
		}
		if s.Outcome4W == domain.OutcomeWin || s.Outcome4W == domain.OutcomeLoss {
			eval4W++
			if s.Outcome4W == domain.OutcomeWin {
				wins4W++
			}
		}
	}

	if eval1W > 0 {
		wr1W = round2(float64(wins1W) / float64(eval1W) * 100)
		ev = round4(sumReturn / float64(eval1W))
	}
	if eval2W > 0 {
		wr2W = round2(float64(wins2W) / float64(eval2W) * 100)
	}
	if eval4W > 0 {
		wr4W = round2(float64(wins4W) / float64(eval4W) * 100)
	}
	return
}
