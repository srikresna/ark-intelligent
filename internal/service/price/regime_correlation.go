package price

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Regime-Aware Correlation Engine
// ---------------------------------------------------------------------------
//
// Extends CorrelationEngine by tagging each historical bar with an HMM regime
// label (RISK_ON / RISK_OFF / CRISIS) and computing separate correlation
// matrices per bucket.  This lets users see how correlations behave differently
// during trending, ranging, and crisis periods — a technique used by
// institutional desks to avoid stale correlation assumptions.

// RegimeTag is the label applied to each daily return.
type RegimeTag string

const (
	RegimeTagTrending RegimeTag = "TRENDING" // ADX > 25 and HMM RISK_ON
	RegimeTagRanging  RegimeTag = "RANGING"  // ADX < 25 or HMM RISK_OFF
	RegimeTagCrisis   RegimeTag = "CRISIS"   // HMM CRISIS
)

// RegimeCorrelationMatrix holds correlation matrices split by regime.
type RegimeCorrelationMatrix struct {
	CurrentRegime RegimeTag                            `json:"current_regime"`
	Overall       *domain.CorrelationMatrix            `json:"overall"`
	PerRegime     map[RegimeTag]*domain.CorrelationMatrix `json:"per_regime"`
	Divergences   []RegimeCorrelationDivergence        `json:"divergences,omitempty"`
	SampleCounts  map[RegimeTag]int                    `json:"sample_counts"` // bars per regime
}

// RegimeCorrelationDivergence flags when the current-regime correlation
// diverges significantly from the overall (unconditional) correlation.
type RegimeCorrelationDivergence struct {
	CurrencyA     string    `json:"currency_a"`
	CurrencyB     string    `json:"currency_b"`
	OverallCorr   float64   `json:"overall_corr"`
	RegimeCorr    float64   `json:"regime_corr"`
	Regime        RegimeTag `json:"regime"`
	Delta         float64   `json:"delta"`
	Significance  string    `json:"significance"` // "HIGH", "MEDIUM"
}

// taggedReturn holds a single daily return with its regime label.
type taggedReturn struct {
	ret   float64
	regime RegimeTag
}

// BuildRegimeCorrelation computes correlation matrices per HMM regime.
// It fetches 120 calendar days of history, tags each day with a regime,
// and builds separate matrices for each bucket.  Minimum 10 returns per
// regime bucket; buckets with fewer are omitted.
func (ce *CorrelationEngine) BuildRegimeCorrelation(ctx context.Context) (*RegimeCorrelationMatrix, error) {
	currencies := domain.DefaultCorrelationCurrencies()
	// Limit to FX + metals + energy + crypto + equity — skip bonds for performance.
	// Use a curated subset for the regime analysis (keeps it fast).
	coreSet := filterCoreCurrencies(currencies)

	const calendarDays = 120
	const minBarsPerBucket = 10

	// 1. Fetch daily history for each currency.
	histories := make(map[string][]domain.DailyPrice) // currency -> newest-first
	var valid []string
	for _, cur := range coreSet {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			continue
		}
		records, err := ce.dailyRepo.GetDailyHistory(ctx, mapping.ContractCode, calendarDays)
		if err != nil || len(records) < 30 {
			continue
		}
		histories[cur] = records
		valid = append(valid, cur)
	}
	if len(valid) < 2 {
		return nil, fmt.Errorf("regime correlation: insufficient currencies (%d)", len(valid))
	}

	// 2. Determine a reference price series for HMM regime detection.
	//    We use SPX500 if available, else the first FX currency.
	refCur := pickReferenceCurrency(valid, histories)
	refPrices := dailyPricesToPriceRecords(histories[refCur])

	var regime *HMMResult
	if len(refPrices) >= 60 {
		var err error
		regime, err = EstimateHMMRegime(refPrices)
		if err != nil {
			corrLog.Warn().Err(err).Msg("regime correlation: HMM failed, using ADX-only classification")
		}
	}

	// 3. Tag each day with a regime label.
	//    returns newest-first; we need chronological order for Viterbi alignment.
	nDays := minDailyLen(histories, valid)
	if nDays < 30 {
		return nil, fmt.Errorf("regime correlation: too few overlapping days (%d)", nDays)
	}

	tags := classifyDays(histories[refCur], regime, nDays)

	// 4. Build per-currency tagged returns.
	currencyTagged := make(map[string][]taggedReturn)
	for _, cur := range valid {
		recs := histories[cur]
		n := nDays
		if n > len(recs)-1 {
			n = len(recs) - 1
		}
		tagged := make([]taggedReturn, 0, n)
		for i := n; i >= 1; i-- { // oldest to newest
			if recs[i].Close > 0 && recs[i-1].Close > 0 {
				ret := (recs[i-1].Close - recs[i].Close) / recs[i].Close * 100
				idx := n - i // chronological index
				if idx < len(tags) {
					tagged = append(tagged, taggedReturn{ret: ret, regime: tags[idx]})
				}
			}
		}
		if len(tagged) > 0 {
			currencyTagged[cur] = tagged
		}
	}

	// 5. Build overall matrix and per-regime matrices.
	overall := buildMatrixFromTagged(valid, currencyTagged, "")
	perRegime := make(map[RegimeTag]*domain.CorrelationMatrix)
	sampleCounts := make(map[RegimeTag]int)

	for _, tag := range []RegimeTag{RegimeTagTrending, RegimeTagRanging, RegimeTagCrisis} {
		count := countTag(tags, tag)
		sampleCounts[tag] = count
		if count >= minBarsPerBucket {
			m := buildMatrixFromTagged(valid, currencyTagged, tag)
			if m != nil {
				perRegime[tag] = m
			}
		}
	}

	// 6. Determine current regime from most recent tag.
	currentRegime := RegimeTagRanging
	if len(tags) > 0 {
		currentRegime = tags[len(tags)-1]
	}

	// 7. Detect divergences: compare regime-specific vs overall.
	var divergences []RegimeCorrelationDivergence
	if currentMatrix, ok := perRegime[currentRegime]; ok && overall != nil {
		divergences = detectRegimeDivergences(valid, overall, currentMatrix, currentRegime)
	}

	return &RegimeCorrelationMatrix{
		CurrentRegime: currentRegime,
		Overall:       overall,
		PerRegime:     perRegime,
		Divergences:   divergences,
		SampleCounts:  sampleCounts,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// filterCoreCurrencies returns a curated subset for regime analysis.
func filterCoreCurrencies(all []string) []string {
	core := map[string]bool{
		"EUR": true, "GBP": true, "JPY": true, "AUD": true,
		"NZD": true, "CAD": true, "CHF": true,
		"XAU": true, "OIL": true,
		"SPX500": true, "BTC": true, "ETH": true,
	}
	var out []string
	for _, c := range all {
		if core[c] {
			out = append(out, c)
		}
	}
	return out
}

// pickReferenceCurrency selects a reference series for regime tagging.
func pickReferenceCurrency(valid []string, histories map[string][]domain.DailyPrice) string {
	for _, pref := range []string{"SPX500", "EUR", "AUD", "GBP"} {
		for _, v := range valid {
			if v == pref {
				return v
			}
		}
	}
	return valid[0]
}

// minDailyLen returns the shortest overlapping history length.
func minDailyLen(histories map[string][]domain.DailyPrice, currencies []string) int {
	minLen := math.MaxInt32
	for _, c := range currencies {
		if l := len(histories[c]); l < minLen {
			minLen = l
		}
	}
	if minLen == math.MaxInt32 {
		return 0
	}
	return minLen
}

// dailyPricesToPriceRecords converts DailyPrice (used by fetcher) to
// PriceRecord (used by HMM).  Records are returned newest-first.
func dailyPricesToPriceRecords(dp []domain.DailyPrice) []domain.PriceRecord {
	out := make([]domain.PriceRecord, len(dp))
	for i, d := range dp {
		out[i] = domain.PriceRecord{
			Date:  d.Date,
			Open:  d.Open,
			High:  d.High,
			Low:   d.Low,
			Close: d.Close,
		}
	}
	return out
}

// classifyDays assigns a RegimeTag to each bar (chronological, oldest-first).
// Uses HMM Viterbi path when available; falls back to simple ADX-like
// volatility classification otherwise.
func classifyDays(refRecs []domain.DailyPrice, hmm *HMMResult, nDays int) []RegimeTag {
	tags := make([]RegimeTag, nDays)

	if hmm != nil && len(hmm.ViterbiPath) > 0 {
		// ViterbiPath is newest-first (same order as bars).
		// Reverse to chronological.
		vp := hmm.ViterbiPath
		for i := 0; i < nDays; i++ {
			vpIdx := len(vp) - 1 - i // map chronological i -> Viterbi index
			if vpIdx < 0 || vpIdx >= len(vp) {
				tags[i] = RegimeTagRanging
				continue
			}
			switch vp[vpIdx] {
			case HMMRiskOn:
				tags[i] = RegimeTagTrending
			case HMMCrisis:
				tags[i] = RegimeTagCrisis
			default: // RISK_OFF
				tags[i] = RegimeTagRanging
			}
		}
		return tags
	}

	// Fallback: simple volatility-based classification.
	// Compute daily absolute returns; high-vol days = crisis, low-vol = ranging.
	if nDays > len(refRecs)-1 {
		nDays = len(refRecs) - 1
	}
	absRets := make([]float64, nDays)
	for i := nDays; i >= 1; i-- {
		if refRecs[i].Close > 0 && refRecs[i-1].Close > 0 {
			absRets[nDays-i] = math.Abs((refRecs[i-1].Close - refRecs[i].Close) / refRecs[i].Close * 100)
		}
	}

	// Compute median absolute return for thresholding.
	sorted := make([]float64, len(absRets))
	copy(sorted, absRets)
	sort.Float64s(sorted)
	median := sorted[len(sorted)/2]
	p90 := sorted[int(float64(len(sorted))*0.90)]

	for i, ar := range absRets {
		switch {
		case ar >= p90:
			tags[i] = RegimeTagCrisis
		case ar > median:
			tags[i] = RegimeTagTrending
		default:
			tags[i] = RegimeTagRanging
		}
	}
	return tags
}

// buildMatrixFromTagged computes a correlation matrix from tagged returns.
// If filterTag is empty, uses all returns.
func buildMatrixFromTagged(currencies []string, tagged map[string][]taggedReturn, filterTag RegimeTag) *domain.CorrelationMatrix {
	// Extract filtered return slices per currency.
	filtered := make(map[string][]float64)
	for _, cur := range currencies {
		tr := tagged[cur]
		if tr == nil {
			continue
		}
		var vals []float64
		for _, t := range tr {
			if filterTag == "" || t.regime == filterTag {
				vals = append(vals, t.ret)
			}
		}
		if len(vals) >= 5 {
			filtered[cur] = vals
		}
	}

	var validCur []string
	for _, c := range currencies {
		if _, ok := filtered[c]; ok {
			validCur = append(validCur, c)
		}
	}
	if len(validCur) < 2 {
		return nil
	}

	matrix := make(map[string]map[string]float64)
	for _, a := range validCur {
		matrix[a] = make(map[string]float64)
		for _, b := range validCur {
			if a == b {
				matrix[a][b] = 1.0
			} else {
				r := pearsonCorrelation(filtered[a], filtered[b])
				if math.IsNaN(r) {
					r = 0
				}
				matrix[a][b] = r
			}
		}
	}

	label := "overall"
	period := 0
	if filterTag != "" {
		label = string(filterTag)
		if len(filtered[validCur[0]]) > 0 {
			period = len(filtered[validCur[0]])
		}
	}
	_ = label

	return &domain.CorrelationMatrix{
		Currencies: validCur,
		Matrix:     matrix,
		Period:     period,
	}
}

// countTag counts how many entries match a given regime tag.
func countTag(tags []RegimeTag, target RegimeTag) int {
	n := 0
	for _, t := range tags {
		if t == target {
			n++
		}
	}
	return n
}

// detectRegimeDivergences compares regime-specific vs overall correlations.
func detectRegimeDivergences(currencies []string, overall, regime *domain.CorrelationMatrix, tag RegimeTag) []RegimeCorrelationDivergence {
	var divs []RegimeCorrelationDivergence
	for _, a := range currencies {
		for _, b := range currencies {
			if a >= b {
				continue
			}
			oCorr, ok1 := overall.Matrix[a][b]
			rCorr, ok2 := regime.Matrix[a][b]
			if !ok1 || !ok2 {
				continue
			}
			if math.IsNaN(oCorr) || math.IsNaN(rCorr) {
				continue
			}
			delta := rCorr - oCorr
			absDelta := math.Abs(delta)

			var sig string
			switch {
			case absDelta >= 0.35:
				sig = "HIGH"
			case absDelta >= 0.20:
				sig = "MEDIUM"
			default:
				continue
			}

			divs = append(divs, RegimeCorrelationDivergence{
				CurrencyA:    a,
				CurrencyB:    b,
				OverallCorr:  roundN(oCorr, 3),
				RegimeCorr:   roundN(rCorr, 3),
				Regime:       tag,
				Delta:        roundN(delta, 3),
				Significance: sig,
			})
		}
	}

	sort.Slice(divs, func(i, j int) bool {
		return math.Abs(divs[i].Delta) > math.Abs(divs[j].Delta)
	})

	// Cap at top 10 divergences.
	if len(divs) > 10 {
		divs = divs[:10]
	}
	return divs
}
