package cot

import (
	"context"
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

// minSampleForSuppression is the minimum number of evaluated signals needed
// before a signal type can be suppressed based on win rate.
// Below this threshold, we keep the signal but flag it as unproven.
const minSampleForSuppression = 10

// minSampleForRecalibration is the minimum number of evaluated signals needed
// before replacing hardcoded confidence with historical win rate.
const minSampleForRecalibration = 5

// SignalTypeStats holds computed win rate and sample size for a signal type.
type SignalTypeStats struct {
	WinRate    float64 // 0-100
	SampleSize int     // number of evaluated signals
	AvgReturn  float64 // average % return at 1W (negative = losing)
	HasEdge    bool    // WinRate >= 50 && SampleSize >= minSampleForSuppression
	Suppressed bool    // WinRate < 50 && SampleSize >= minSampleForSuppression
}

// RecalibratedDetector wraps SignalDetector and adds:
//  1. Confidence recalibration — replaces rule-based confidence with historical win rate
//  2. Signal suppression — drops signal types with confirmed negative EV
//  3. VIX/risk adjustment — applies risk context multiplier to final confidence
type RecalibratedDetector struct {
	base       *SignalDetector
	signalRepo ports.SignalRepository
	// cached stats — loaded once per detection run
	typeStats map[string]*SignalTypeStats
}

// NewRecalibratedDetector creates a recalibrated detector backed by historical data.
// If signalRepo is nil, it degrades gracefully to the base detector behaviour.
func NewRecalibratedDetector(signalRepo ports.SignalRepository) *RecalibratedDetector {
	return &RecalibratedDetector{
		base:       NewSignalDetector(),
		signalRepo: signalRepo,
	}
}

// LoadTypeStats fetches and caches per-signal-type win rate statistics.
// Call this once before a detection run to avoid repeated DB reads.
func (rd *RecalibratedDetector) LoadTypeStats(ctx context.Context) error {
	if rd.signalRepo == nil {
		rd.typeStats = nil
		return nil
	}

	allSignals, err := rd.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return err
	}

	// Group signals by type
	grouped := make(map[string][]domain.PersistedSignal)
	for _, s := range allSignals {
		grouped[s.SignalType] = append(grouped[s.SignalType], s)
	}

	rd.typeStats = make(map[string]*SignalTypeStats, len(grouped))

	for sigType, signals := range grouped {
		var wins, evaluated int
		var sumReturn float64

		for _, s := range signals {
			if s.Outcome1W == "" || s.Outcome1W == domain.OutcomePending {
				continue
			}
			evaluated++
			sumReturn += s.Return1W
			if s.Outcome1W == domain.OutcomeWin {
				wins++
			}
		}

		stats := &SignalTypeStats{
			SampleSize: evaluated,
		}

		if evaluated > 0 {
			stats.WinRate = math.Round(float64(wins)/float64(evaluated)*100*100) / 100
			stats.AvgReturn = math.Round(sumReturn/float64(evaluated)*10000) / 10000
		}

		// Determine edge / suppression status
		if evaluated >= minSampleForSuppression {
			if stats.WinRate >= 50 {
				stats.HasEdge = true
			} else {
				stats.Suppressed = true
			}
		}

		rd.typeStats[sigType] = stats
	}

	return nil
}

// TypeStats returns the stats for a signal type, or nil if no data yet.
func (rd *RecalibratedDetector) TypeStats(sigType string) *SignalTypeStats {
	if rd.typeStats == nil {
		return nil
	}
	return rd.typeStats[string(sigType)]
}

// AllTypeStats returns a snapshot of all loaded type stats.
func (rd *RecalibratedDetector) AllTypeStats() map[string]*SignalTypeStats {
	return rd.typeStats
}

// DetectAll runs detection, applies recalibration + suppression + VIX filter.
//
//   - Suppressed signal types (confirmed negative EV, n >= 10) are dropped entirely.
//   - Confidence is replaced with historical win rate when n >= 5.
//   - riskCtx (optional) applies VIX/SPX multiplier to final confidence.
func (rd *RecalibratedDetector) DetectAll(
	analyses []domain.COTAnalysis,
	historyMap map[string][]domain.COTRecord,
	riskCtx *domain.RiskContext,
) []Signal {
	// Run base detection — all 7 detectors
	rawSignals := rd.base.DetectAll(analyses, historyMap)

	result := make([]Signal, 0, len(rawSignals))

	for _, sig := range rawSignals {
		sigTypeKey := string(sig.Type)
		stats := rd.TypeStats(sigTypeKey)

		// --- Signal Suppression ---
		if stats != nil && stats.Suppressed {
			// Log suppression at debug level (no logger in this package — use signal factor annotation)
			// Append suppression note to factors so it's visible in debug output
			sig.Factors = append(sig.Factors,
				"⛔ SUPPRESSED: win rate "+fmtWinRate(stats.WinRate)+" (n="+intToStr(stats.SampleSize)+")",
			)
			// Skip — do not include in output
			continue
		}

		// --- Confidence Recalibration ---
		if stats != nil && stats.SampleSize >= minSampleForRecalibration {
			originalConf := sig.Confidence
			// Replace rule-based confidence with empirical win rate
			sig.Confidence = stats.WinRate
			// BUG-H1 fix: recalculate Strength to stay consistent with new Confidence.
			// Strength must reflect empirical quality, not the stale rule-based estimate.
			sig.Strength = confidenceToStrength(sig.Confidence)
			// Annotate the change for transparency
			if math.Abs(originalConf-sig.Confidence) > 5 {
				sig.Factors = append(sig.Factors,
					"📊 Confidence recalibrated: "+fmtWinRate(originalConf)+" → "+fmtWinRate(sig.Confidence)+
						" (n="+intToStr(stats.SampleSize)+")",
				)
			}
		}

		// --- VIX / Risk Context Adjustment ---
		if riskCtx != nil {
			originalConf := sig.Confidence
			sig.Confidence = riskCtx.AdjustConfidence(sig.Confidence)
			if math.Abs(originalConf-sig.Confidence) > 1 {
				adj := riskCtx.ConfidenceAdjustment()
				adjLabel := ""
				switch {
				case adj < 0.80:
					adjLabel = "🔴 VIX dampened"
				case adj < 0.95:
					adjLabel = "🟡 VIX reduced"
				case adj > 1.10:
					adjLabel = "🟢 VIX boosted"
				}
				if adjLabel != "" {
					sig.Factors = append(sig.Factors,
						adjLabel+": "+riskCtx.RegimeLabel()+
							" (conf "+fmtWinRate(originalConf)+" → "+fmtWinRate(sig.Confidence)+")",
					)
				}
			}
		}

		// Re-clamp confidence after all adjustments
		sig.Confidence = mathutil.Clamp(sig.Confidence, 0, 100)

		result = append(result, sig)
	}

	return result
}

// SuppressedTypes returns the list of signal types currently being suppressed.
func (rd *RecalibratedDetector) SuppressedTypes() []string {
	var out []string
	for k, v := range rd.typeStats {
		if v.Suppressed {
			out = append(out, k)
		}
	}
	return out
}

// fmtWinRate formats a float as "XX.X%".
// Handles range [0, 100] correctly. Values outside this range are clamped.
func fmtWinRate(v float64) string {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	i := int(v*10 + 0.5) // round to nearest tenth
	return intToStr(i/10) + "." + intToStr(i%10) + "%"
}

// confidenceToStrength maps a confidence value [0,100] to Strength [1,5].
// Mirrors the inverse of the base detector's strength-to-confidence mapping
// so Strength stays consistent after recalibration.
//
//	< 40%  → 1   (weak / noise)
//	40-54% → 2   (below-average)
//	55-64% → 3   (moderate)
//	65-74% → 4   (strong)
//	≥ 75%  → 5   (very strong)
func confidenceToStrength(conf float64) int {
	switch {
	case conf >= 75:
		return 5
	case conf >= 65:
		return 4
	case conf >= 55:
		return 3
	case conf >= 40:
		return 2
	default:
		return 1
	}
}

// intToStr converts a non-negative int to string (avoids strconv import).
func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	if i < 0 {
		return "-" + intToStr(-i)
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits
}
