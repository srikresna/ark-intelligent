package cot

import (
	"context"
	"math"
	"strconv"
	"strings"
	"sync"

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

// minSampleForPlatt is the minimum number of evaluated signals needed
// before using Platt scaling instead of simple win-rate replacement.
const minSampleForPlatt = 20

// SignalTypeStats holds computed win rate and sample size for a signal type.
type SignalTypeStats struct {
	WinRate    float64 // 0-100
	SampleSize int     // number of evaluated signals
	AvgReturn  float64 // average % return at 1W (negative = losing)
	HasEdge    bool    // WinRate >= 50 && SampleSize >= minSampleForSuppression
	Suppressed bool    // WinRate < 50 && SampleSize >= minSampleForSuppression
	PlattA     float64 // Platt scaling coefficient a (0 if not fitted)
	PlattB     float64 // Platt scaling coefficient b (0 if not fitted)
	UsePlatt   bool    // true if Platt scaling is available (n >= minSampleForPlatt)
}

// RecalibratedDetector wraps SignalDetector and adds:
//  1. Confidence recalibration — replaces rule-based confidence with historical win rate
//  2. Signal suppression — drops signal types with confirmed negative EV
//  3. VIX/risk adjustment — applies risk context multiplier to final confidence
//
// Stats are stored at two granularity levels:
//   - granularStats: keyed by "SIGNAL_TYPE:CURRENCY" (e.g. "SMART_MONEY:EUR")
//   - typeStats: keyed by "SIGNAL_TYPE" (pooled across all currencies)
//
// DetectAll prefers granular stats when sample size is sufficient, falling back
// to pooled stats. This prevents a signal type that works for EUR from being
// suppressed because it underperforms on JPY (or vice versa).
type RecalibratedDetector struct {
	base          *SignalDetector
	signalRepo    ports.SignalRepository
	mu            sync.RWMutex // protects typeStats, granularStats, regimeStats, currentRegime
	typeStats     map[string]*SignalTypeStats // pooled by signal type
	granularStats map[string]*SignalTypeStats // keyed "TYPE:CURRENCY"
	regimeStats   map[string]*SignalTypeStats // keyed "TYPE:REGIME" e.g. "MOMENTUM_SHIFT:STRESS"
	currentRegime string                      // FRED regime for current detection run
}

// SetCurrentRegime sets the FRED regime for the current detection run.
// When set, DetectAll will also check regime-specific suppression stats.
func (rd *RecalibratedDetector) SetCurrentRegime(regime string) {
	rd.mu.Lock()
	defer rd.mu.Unlock()
	rd.currentRegime = regime
}

// NewRecalibratedDetector creates a recalibrated detector backed by historical data.
// If signalRepo is nil, it degrades gracefully to the base detector behaviour.
func NewRecalibratedDetector(signalRepo ports.SignalRepository) *RecalibratedDetector {
	return &RecalibratedDetector{
		base:       NewSignalDetector(),
		signalRepo: signalRepo,
	}
}

// LoadTypeStats fetches and caches per-signal-type win rate statistics
// at two granularity levels: per signal type (pooled) and per type+currency (granular).
// Call this once before a detection run to avoid repeated DB reads.
func (rd *RecalibratedDetector) LoadTypeStats(ctx context.Context) error {
	if rd.signalRepo == nil {
		rd.mu.Lock()
		rd.typeStats = nil
		rd.granularStats = nil
		rd.regimeStats = nil
		rd.mu.Unlock()
		return nil
	}

	allSignals, err := rd.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return err
	}

	// Group signals by type (pooled) and by type:currency (granular)
	byType := make(map[string][]domain.PersistedSignal)
	byTypeCurrency := make(map[string][]domain.PersistedSignal)
	for _, s := range allSignals {
		byType[s.SignalType] = append(byType[s.SignalType], s)
		granularKey := s.SignalType + ":" + s.Currency
		byTypeCurrency[granularKey] = append(byTypeCurrency[granularKey], s)
	}

	pooled := computeStatsMap(byType)
	granular := computeStatsMap(byTypeCurrency)

	// Group signals by type:regime
	byTypeRegime := make(map[string][]domain.PersistedSignal)
	for _, s := range allSignals {
		if s.FREDRegime == "" {
			continue
		}
		regimeKey := s.SignalType + ":" + s.FREDRegime
		byTypeRegime[regimeKey] = append(byTypeRegime[regimeKey], s)
	}
	regime := computeStatsMap(byTypeRegime)

	rd.mu.Lock()
	rd.typeStats = pooled
	rd.granularStats = granular
	rd.regimeStats = regime
	rd.mu.Unlock()

	return nil
}

// computeStatsMap computes SignalTypeStats for each group in the map.
func computeStatsMap(grouped map[string][]domain.PersistedSignal) map[string]*SignalTypeStats {
	stats := make(map[string]*SignalTypeStats, len(grouped))
	for key, signals := range grouped {
		var wins, evaluated int
		var sumReturn float64
		var confidences []float64
		var outcomes []bool

		for _, s := range signals {
			if s.Outcome1W == "" || s.Outcome1W == domain.OutcomePending {
				continue
			}
			evaluated++
			sumReturn += s.Return1W
			isWin := s.Outcome1W == domain.OutcomeWin
			if isWin {
				wins++
			}
			confidences = append(confidences, s.Confidence)
			outcomes = append(outcomes, isWin)
		}

		st := &SignalTypeStats{
			SampleSize: evaluated,
		}

		if evaluated > 0 {
			st.WinRate = math.Round(float64(wins)/float64(evaluated)*100*100) / 100
			st.AvgReturn = math.Round(sumReturn/float64(evaluated)*10000) / 10000
		}

		// Determine edge / suppression status
		if evaluated >= minSampleForSuppression {
			if st.WinRate >= 50 {
				st.HasEdge = true
			} else {
				st.Suppressed = true
			}
		}

		// Fit Platt scaling when sufficient data is available
		if evaluated >= minSampleForPlatt {
			a, b := mathutil.PlattScaling(confidences, outcomes)
			// Only use Platt if fitting succeeded (non-zero coefficients)
			if a != 0 || b != 0 {
				st.PlattA = a
				st.PlattB = b
				st.UsePlatt = true
			}
		}

		stats[key] = st
	}
	return stats
}

// TypeStats returns the stats for a signal type, or nil if no data yet.
func (rd *RecalibratedDetector) TypeStats(sigType string) *SignalTypeStats {
	rd.mu.RLock()
	defer rd.mu.RUnlock()
	if rd.typeStats == nil {
		return nil
	}
	return rd.typeStats[sigType]
}

// RegimeStats returns the stats for a signal type in a specific regime, or nil if no data.
func (rd *RecalibratedDetector) RegimeStats(sigType, regime string) *SignalTypeStats {
	rd.mu.RLock()
	defer rd.mu.RUnlock()
	if rd.regimeStats == nil {
		return nil
	}
	return rd.regimeStats[sigType+":"+regime]
}

// AllTypeStats returns a shallow copy of all loaded type stats (pooled + granular).
// Granular keys use "TYPE:CURRENCY" format, pooled keys use "TYPE" format.
func (rd *RecalibratedDetector) AllTypeStats() map[string]*SignalTypeStats {
	rd.mu.RLock()
	defer rd.mu.RUnlock()
	if rd.typeStats == nil && rd.granularStats == nil {
		return nil
	}
	out := make(map[string]*SignalTypeStats)
	for k, v := range rd.typeStats {
		out[k] = v
	}
	for k, v := range rd.granularStats {
		out[k] = v
	}
	return out
}

// DetectAll runs detection, applies recalibration + suppression + VIX/ATR filter.
//
// Lookup order for each signal:
//  1. Granular stats ("TYPE:CURRENCY") — used if sample size >= threshold.
//  2. Pooled stats ("TYPE") — fallback when granular data is insufficient.
//
// This means suppression/recalibration is per-currency when data allows,
// preventing cross-currency contamination of signal quality metrics.
//
// volCtxMap is optional — keyed by contract code. When present, ATR-based
// volatility multiplier is combined with VIX to avoid double-penalizing.
func (rd *RecalibratedDetector) DetectAll(
	analyses []domain.COTAnalysis,
	historyMap map[string][]domain.COTRecord,
	riskCtx *domain.RiskContext,
	volCtxMap ...map[string]*domain.PriceContext,
) []Signal {
	// Run base detection — all 7 detectors
	rawSignals := rd.base.DetectAll(analyses, historyMap)

	rd.mu.RLock()
	localPooled := rd.typeStats
	localGranular := rd.granularStats
	localRegime := rd.regimeStats
	localCurrentRegime := rd.currentRegime
	rd.mu.RUnlock()

	result := make([]Signal, 0, len(rawSignals))

	for _, sig := range rawSignals {
		sigTypeKey := string(sig.Type)

		// Two-tier stats lookup: granular first, then pooled
		stats := rd.resolveStats(sigTypeKey, sig.Currency, localGranular, localPooled)

		// --- Signal Suppression ---
		if stats != nil && stats.Suppressed {
			continue
		}

		// --- Regime-Conditional Suppression ---
		if localCurrentRegime != "" && localRegime != nil {
			regimeKey := sigTypeKey + ":" + localCurrentRegime
			if rs := localRegime[regimeKey]; rs != nil && rs.Suppressed {
				sig.Factors = append(sig.Factors,
					"🔴 Regime-suppressed: "+sigTypeKey+" has "+fmtWinRate(rs.WinRate)+
						" win rate during "+localCurrentRegime+" (n="+strconv.Itoa(rs.SampleSize)+")",
				)
				continue // drop this signal
			}
		}

		// --- Confidence Recalibration ---
		if stats != nil && stats.SampleSize >= minSampleForRecalibration {
			originalConf := sig.Confidence

			if stats.UsePlatt {
				// Platt scaling: logistic regression maps raw confidence to calibrated probability
				sig.Confidence = mathutil.PlattCalibrate(originalConf, stats.PlattA, stats.PlattB)
			} else {
				// Fallback: replace rule-based confidence with empirical win rate
				sig.Confidence = stats.WinRate
			}

			// BUG-H1 fix: recalculate Strength to stay consistent with new Confidence.
			sig.Strength = confidenceToStrength(sig.Confidence)
			// Annotate the change for transparency
			if math.Abs(originalConf-sig.Confidence) > 5 {
				calMethod := "WinRate"
				if stats.UsePlatt {
					calMethod = "Platt"
				}
				label := "📊 Confidence recalibrated [" + calMethod + "]"
				// Indicate whether granular or pooled stats were used
				granularKey := sigTypeKey + ":" + sig.Currency
				if localGranular != nil && localGranular[granularKey] != nil &&
					localGranular[granularKey].SampleSize >= minSampleForRecalibration {
					label += " [" + sig.Currency + "]"
				}
				sig.Factors = append(sig.Factors,
					label+": "+fmtWinRate(originalConf)+" → "+fmtWinRate(sig.Confidence)+
						" (n="+strconv.Itoa(stats.SampleSize)+")",
				)
			}
		}

		// --- VIX / ATR Volatility Adjustment ---
		// Combine VIX (market-wide) with ATR (per-contract) to avoid double-penalizing.
		// When both are available, average the two multipliers.
		atrMult := 1.0
		var priceCtxs map[string]*domain.PriceContext
		if len(volCtxMap) > 0 {
			priceCtxs = volCtxMap[0]
		}
		if priceCtxs != nil {
			if pc := priceCtxs[sig.ContractCode]; pc != nil && pc.VolatilityMultiplier > 0 {
				atrMult = pc.VolatilityMultiplier
			}
		}

		if riskCtx != nil || atrMult != 1.0 {
			originalConf := sig.Confidence
			vixMult := 1.0
			if riskCtx != nil {
				vixMult = riskCtx.ConfidenceAdjustment()
			}

			// Combine: if both are available, average to avoid stacking penalties.
			// If only one is available, use it directly.
			var combinedMult float64
			switch {
			case riskCtx != nil && atrMult != 1.0:
				combinedMult = (vixMult + atrMult) / 2
			case riskCtx != nil:
				combinedMult = vixMult
			default:
				combinedMult = atrMult
			}

			sig.Confidence = mathutil.Clamp(sig.Confidence*combinedMult, 0, 100)

			if math.Abs(originalConf-sig.Confidence) > 1 {
				// Build label describing the adjustment source(s)
				var parts []string
				if riskCtx != nil && math.Abs(vixMult-1.0) > 0.01 {
					adjLabel := ""
					switch {
					case vixMult < 0.80:
						adjLabel = "VIX dampened"
					case vixMult < 0.95:
						adjLabel = "VIX reduced"
					case vixMult > 1.10:
						adjLabel = "VIX boosted"
					}
					if adjLabel != "" {
						parts = append(parts, "\xF0\x9F\x94\xB4 "+adjLabel+": "+riskCtx.RegimeLabel())
					}
				}
				if atrMult != 1.0 {
					atrLabel := ""
					switch {
					case atrMult < 1.0:
						atrLabel = "ATR expanding"
					case atrMult > 1.0:
						atrLabel = "ATR contracting"
					}
					if atrLabel != "" {
						parts = append(parts, "\xF0\x9F\x93\x8A "+atrLabel)
					}
				}
				if len(parts) > 0 {
					label := strings.Join(parts, " + ")
					sig.Factors = append(sig.Factors,
						label+" (conf "+fmtWinRate(originalConf)+" \xE2\x86\x92 "+fmtWinRate(sig.Confidence)+")",
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

// resolveStats picks the best available stats for a signal.
// Prefers granular (type:currency) when sample size is sufficient for the
// operation at hand; otherwise falls back to pooled (type-only) stats.
func (rd *RecalibratedDetector) resolveStats(
	sigType, currency string,
	granular, pooled map[string]*SignalTypeStats,
) *SignalTypeStats {
	// Try granular first
	if granular != nil {
		key := sigType + ":" + currency
		if gs := granular[key]; gs != nil {
			// Use granular if it has enough data for at least recalibration.
			// For suppression (n>=10), granular is also used when available,
			// preventing a bad currency from poisoning a good one.
			if gs.SampleSize >= minSampleForRecalibration {
				return gs
			}
		}
	}
	// Fall back to pooled
	if pooled != nil {
		return pooled[sigType]
	}
	return nil
}

// SuppressedTypes returns the list of signal types/keys currently being suppressed.
// Includes both pooled ("THIN_MARKET") and granular ("THIN_MARKET:JPY") entries.
func (rd *RecalibratedDetector) SuppressedTypes() []string {
	rd.mu.RLock()
	defer rd.mu.RUnlock()
	var out []string
	for k, v := range rd.typeStats {
		if v.Suppressed {
			out = append(out, k)
		}
	}
	for k, v := range rd.granularStats {
		if v.Suppressed {
			out = append(out, k)
		}
	}
	for k, v := range rd.regimeStats {
		if v.Suppressed {
			out = append(out, k+" [regime]")
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
	return strconv.Itoa(i/10) + "." + strconv.Itoa(i%10) + "%"
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
