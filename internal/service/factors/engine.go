// Package factors implements the cross-sectional factor ranking engine.
package factors

import (
	"sort"
	"time"
)

// Engine computes cross-sectional factor scores and ranks a universe of assets.
type Engine struct {
	weights Weights
}

// NewEngine creates a Factor Engine with the given weights.
// Pass DefaultWeights() for production defaults.
func NewEngine(w Weights) *Engine {
	return &Engine{weights: w}
}

// Rank computes factor scores for all profiles, normalizes cross-sectionally,
// and returns a RankingResult sorted by composite score descending (best first).
func (e *Engine) Rank(profiles []AssetProfile) *RankingResult {
	if len(profiles) == 0 {
		return &RankingResult{ComputedAt: time.Now()}
	}

	// --- Step 1: Raw scores per asset ---
	rawMom := make([]float64, len(profiles))
	rawTQ := make([]float64, len(profiles))
	rawLV := make([]float64, len(profiles))
	rawRR := make([]float64, len(profiles))

	for i, p := range profiles {
		rawMom[i] = scoreMomentum(p.DailyCloses)
		rawTQ[i] = scoreTrendQuality(p.DailyCloses)
		rawLV[i] = scoreLowVol(p.DailyCloses, 63)
		rawRR[i] = scoreResidualReversal(p.DailyCloses)
	}

	// --- Step 2: Cross-sectional normalization (z-score → [-1, +1]) ---
	normMom := zscore(rawMom)
	normTQ := zscore(rawTQ)
	normLV := zscore(rawLV)

	// Residual reversal: compute cross-sectional residuals if enough assets
	normRR := make([]float64, len(profiles))
	if len(profiles) >= 5 {
		// Build universe average return for cross-sectional residuals
		universeRets := computeUniverseReturns(profiles, 21)
		for i, p := range profiles {
			if len(p.DailyCloses) >= 22 {
				assetRets := make([]float64, 21)
				for j := 0; j < 21 && j < len(p.DailyCloses)-1; j++ {
					prev := p.DailyCloses[j+1]
					if prev != 0 {
						assetRets[j] = (p.DailyCloses[j] - prev) / prev
					}
				}
				normRR[i] = crossSectionalResiduals(assetRets, universeRets)
			} else {
				normRR[i] = rawRR[i]
			}
		}
	} else {
		copy(normRR, rawRR)
	}
	normRR = zscore(normRR)

	// --- Step 3: Carry-adjusted and crowding scores (already bounded [-1,+1]) ---
	normCA := make([]float64, len(profiles))
	normCR := make([]float64, len(profiles))
	for i, p := range profiles {
		normCA[i] = scoreCarryAdjusted(p)
		normCR[i] = scoreCrowding(p)
	}
	normCA = zscore(normCA)
	normCR = zscore(normCR)

	// --- Step 4: Build ranked assets ---
	ranked := make([]RankedAsset, len(profiles))
	for i, p := range profiles {
		scores := FactorScores{
			Momentum:         normMom[i],
			TrendQuality:     normTQ[i],
			CarryAdjusted:    normCA[i],
			LowVol:           normLV[i],
			ResidualReversal: normRR[i],
			Crowding:         normCR[i],
		}
		composite := scores.Combined(e.weights)
		composite = clamp1(composite)

		ranked[i] = RankedAsset{
			ContractCode:   p.ContractCode,
			Currency:       p.Currency,
			Name:           p.Name,
			Scores:         scores,
			CompositeScore: composite,
			Signal:         CompositeToSignal(composite),
			UpdatedAt:      time.Now(),
		}
	}

	// --- Step 5: Sort by composite score descending ---
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].CompositeScore > ranked[j].CompositeScore
	})

	// --- Step 6: Assign ranks and quartiles ---
	n := len(ranked)
	for i := range ranked {
		ranked[i].Rank = i + 1
		q := (i * 4) / n
		if q > 3 {
			q = 3
		}
		ranked[i].Quartile = q + 1
	}

	return &RankingResult{
		Assets:     ranked,
		ComputedAt: time.Now(),
		AssetCount: n,
	}
}

// computeUniverseReturns computes the equal-weighted average daily returns
// for the universe over the last n bars. Returns returns newest-first.
func computeUniverseReturns(profiles []AssetProfile, n int) []float64 {
	universe := make([]float64, n)
	count := 0

	for _, p := range profiles {
		if len(p.DailyCloses) < n+1 {
			continue
		}
		for i := 0; i < n; i++ {
			prev := p.DailyCloses[i+1]
			if prev == 0 {
				continue
			}
			universe[i] += (p.DailyCloses[i] - prev) / prev
		}
		count++
	}

	if count == 0 {
		return universe
	}
	for i := range universe {
		universe[i] /= float64(count)
	}
	return universe
}
