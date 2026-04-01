package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Factor Decomposition — Attribution of Signal Returns to Contributing Factors
// ---------------------------------------------------------------------------
//
// Decomposes per-signal returns into contributions from:
//   1. COT Positioning  — net position changes, COT index
//   2. Macro Regime     — FRED regime favorability
//   3. Trend Following  — daily price trend alignment
//   4. Volatility       — ATR/VIX environment
//
// Uses partial regression coefficients to attribute observed Return1W to each
// factor's contribution: return ≈ β_cot*x_cot + β_macro*x_macro + β_trend*x_trend + β_vol*x_vol + residual

// FactorDecomposer performs return attribution across signals.
type FactorDecomposer struct {
	signalRepo ports.SignalRepository
}

// FactorContribution holds the decomposition for a single factor.
type FactorContribution struct {
	Name          string  `json:"name"`
	Coefficient   float64 `json:"coefficient"`    // Regression beta
	AvgContrib    float64 `json:"avg_contrib"`    // Average contribution to return (%)
	PctExplained  float64 `json:"pct_explained"`  // % of total R² attributed to this factor
	IsSignificant bool    `json:"is_significant"` // p < 0.05
	PValue        float64 `json:"p_value"`
	Direction     string  `json:"direction"` // "POSITIVE", "NEGATIVE", "NEUTRAL"
}

// DecompositionResult holds the full factor decomposition output.
type DecompositionResult struct {
	Factors          []FactorContribution `json:"factors"`
	RSquared         float64              `json:"r_squared"`
	AdjRSquared      float64              `json:"adj_r_squared"`
	ResidualPct      float64              `json:"residual_pct"`       // % unexplained
	SampleSize       int                  `json:"sample_size"`
	TopFactor        string               `json:"top_factor"`         // Highest contributing factor
	EdgeSource       string               `json:"edge_source"`        // Where the alpha comes from
	PerCurrency      map[string]*DecompositionResult `json:"per_currency,omitempty"`
}

// NewFactorDecomposer creates a new decomposer.
func NewFactorDecomposer(signalRepo ports.SignalRepository) *FactorDecomposer {
	return &FactorDecomposer{signalRepo: signalRepo}
}

// Decompose performs factor decomposition across all evaluated signals.
func (fd *FactorDecomposer) Decompose(ctx context.Context) (*DecompositionResult, error) {
	signals, err := fd.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get signals: %w", err)
	}

	// Filter to evaluated signals with returns
	var evaluated []domain.PersistedSignal
	for _, s := range signals {
		if s.Outcome1W == domain.OutcomeWin || s.Outcome1W == domain.OutcomeLoss {
			if s.EntryPrice > 0 {
				evaluated = append(evaluated, s)
			}
		}
	}

	if len(evaluated) < 10 {
		return nil, fmt.Errorf("insufficient evaluated signals for decomposition: need 10, got %d", len(evaluated))
	}

	result := decomposeSignals(evaluated, "ALL")

	// Per-currency decomposition
	byCurrency := make(map[string][]domain.PersistedSignal)
	for _, s := range evaluated {
		byCurrency[s.Currency] = append(byCurrency[s.Currency], s)
	}
	result.PerCurrency = make(map[string]*DecompositionResult)
	for cur, sigs := range byCurrency {
		if len(sigs) >= 10 {
			result.PerCurrency[cur] = decomposeSignals(sigs, cur)
		}
	}

	return result, nil
}

// hasVariance returns true if the data slice has non-trivial variance.
func hasVariance(data []float64) bool {
	if len(data) < 2 {
		return false
	}
	m := mean(data)
	s := stdDev(data, m)
	return s >= 1e-10
}

// decomposeSignals performs the actual decomposition on a set of signals.
// It dynamically excludes factors with zero variance (missing/constant data)
// and only regresses on factors that carry information.
func decomposeSignals(signals []domain.PersistedSignal, label string) *DecompositionResult {
	n := len(signals)

	// Extract factor scores and target (Return1W)
	allNames := []string{"COT Positioning", "Macro Regime", "Trend Following", "Volatility"}
	allScores := make([][]float64, 4)
	returns := make([]float64, n)

	for i, s := range signals {
		returns[i] = s.Return1W
	}

	// Extract raw scores for each factor
	rawCOT := make([]float64, n)
	rawMacro := make([]float64, n)
	rawTrend := make([]float64, n)
	rawVol := make([]float64, n)
	for i, s := range signals {
		rawCOT[i] = extractCOTScore(&s)
		rawMacro[i] = extractMacroScore(&s)
		rawTrend[i] = extractTrendScore(&s)
		rawVol[i] = extractVolScore(&s)
	}
	allScores[0] = rawCOT
	allScores[1] = rawMacro
	allScores[2] = rawTrend
	allScores[3] = rawVol

	// Determine which factors have variance (i.e., have real data)
	type activeFactor struct {
		idx  int      // original index (0-3)
		name string
		norm []float64
		raw  []float64
	}
	var active []activeFactor
	for j := 0; j < 4; j++ {
		if hasVariance(allScores[j]) {
			active = append(active, activeFactor{
				idx:  j,
				name: allNames[j],
				norm: zScoreNormalize(allScores[j]),
				raw:  allScores[j],
			})
		}
	}

	// If no factors have variance, return a result indicating data insufficiency
	if len(active) == 0 {
		factors := make([]FactorContribution, 4)
		for j := 0; j < 4; j++ {
			factors[j] = FactorContribution{
				Name:      allNames[j],
				Direction: "NO DATA",
				PValue:    1.0,
			}
		}
		return &DecompositionResult{
			Factors:     factors,
			RSquared:    0,
			AdjRSquared: 0,
			ResidualPct: 100,
			SampleSize:  n,
			TopFactor:   "",
			EdgeSource:  fmt.Sprintf("Insufficient enrichment data — only %d/4 factors have variance", len(active)),
		}
	}

	// Build regression matrix with ONLY active factors
	// Return1W = β0 + β1*Factor1 + β2*Factor2 + ...
	p := len(active)
	X := make([][]float64, n)
	for i := 0; i < n; i++ {
		row := make([]float64, p+1)
		row[0] = 1.0 // intercept
		for k, af := range active {
			row[k+1] = af.norm[i]
		}
		X[i] = row
	}

	// Simple OLS via normal equations
	betas, rSquared, pValues := simpleOLS(X, returns)

	// Build factor contributions — include ALL 4 factors in output,
	// mark inactive ones as "NO DATA"
	factors := make([]FactorContribution, 4)

	// First, compute active factor contributions
	totalAbsBeta := 0.0
	activeBetas := make(map[int]float64) // original factor idx → beta
	activePVals := make(map[int]float64)
	activeNorms := make(map[int][]float64)
	for k, af := range active {
		beta := betas[k+1] // skip intercept
		activeBetas[af.idx] = beta
		activeNorms[af.idx] = af.norm
		totalAbsBeta += math.Abs(beta)
		if k+1 < len(pValues) {
			activePVals[af.idx] = pValues[k+1]
		}
	}

	topFactor := ""
	topContrib := 0.0

	for j := 0; j < 4; j++ {
		beta, isActive := activeBetas[j]
		if !isActive {
			// Factor had no variance — mark as no data
			factors[j] = FactorContribution{
				Name:      allNames[j],
				Direction: "NO DATA",
				PValue:    1.0,
			}
			continue
		}

		norms := activeNorms[j]

		// Average absolute contribution = beta * mean(|factor|)
		absSum := 0.0
		for _, v := range norms {
			absSum += math.Abs(v)
		}
		meanAbsNorm := 0.0
		if len(norms) > 0 {
			meanAbsNorm = absSum / float64(len(norms))
		}
		avgContrib := beta * meanAbsNorm

		// % of R² explained (Shapley-like approximation via relative coefficient size)
		pctExplained := 0.0
		if totalAbsBeta > 0 {
			pctExplained = (math.Abs(beta) / totalAbsBeta) * rSquared * 100
		}

		dir := "NEUTRAL"
		if beta > 0.001 {
			dir = "POSITIVE"
		} else if beta < -0.001 {
			dir = "NEGATIVE"
		}

		pVal := 1.0
		if pv, ok := activePVals[j]; ok {
			pVal = pv
		}

		factors[j] = FactorContribution{
			Name:          allNames[j],
			Coefficient:   roundN(beta, 6),
			AvgContrib:    roundN(avgContrib, 4),
			PctExplained:  roundN(pctExplained, 2),
			IsSignificant: pVal < 0.05,
			PValue:        roundN(pVal, 4),
			Direction:     dir,
		}

		if math.Abs(beta) > topContrib {
			topContrib = math.Abs(beta)
			topFactor = allNames[j]
		}
	}

	// Sort by contribution magnitude
	sort.Slice(factors, func(i, j int) bool {
		return factors[i].PctExplained > factors[j].PctExplained
	})

	// Determine edge source
	edgeSource := "No clear alpha source"
	if len(active) < 4 {
		missing := 4 - len(active)
		edgeSource = fmt.Sprintf("Partial analysis (%d/4 factors available) — ", len(active))
		if rSquared > 0.05 {
			edgeSource += fmt.Sprintf("Primary alpha from %s", factors[0].Name)
		} else {
			edgeSource += fmt.Sprintf("%d factors lack enrichment data", missing)
		}
	} else if rSquared > 0.05 {
		sigFactors := 0
		for _, f := range factors {
			if f.IsSignificant {
				sigFactors++
			}
		}
		if sigFactors > 0 {
			edgeSource = fmt.Sprintf("Primary alpha from %s", factors[0].Name)
		}
	}

	return &DecompositionResult{
		Factors:     factors,
		RSquared:    roundN(rSquared, 4),
		AdjRSquared: roundN(adjustedRSquared(rSquared, n, p), 4),
		ResidualPct: roundN((1-rSquared)*100, 2),
		SampleSize:  n,
		TopFactor:   topFactor,
		EdgeSource:  edgeSource,
	}
}

// --- Factor Score Extraction ---

func extractCOTScore(s *domain.PersistedSignal) float64 {
	// COT contribution: COT index deviation from neutral + sentiment
	score := (s.COTIndex - 50) * 2 // -100 to +100
	if s.SentimentScore != 0 {
		// SentimentScore is already in [-100, +100]; blend directly without extra *100.
		score = score*0.6 + s.SentimentScore*0.4
	}
	// Adjust by direction
	if s.Direction == "BEARISH" {
		score = -score
	}
	return clampF(score, -100, 100)
}

func extractMacroScore(s *domain.PersistedSignal) float64 {
	switch s.FREDRegime {
	case "EXPANSION":
		return 80
	case "GOLDILOCKS":
		return 60
	case "NORMAL":
		return 0
	case "TIGHTENING":
		return -40
	case "STRESS":
		return -60
	case "RECESSION":
		return -80
	case "STAGFLATION":
		return -70
	default:
		return 0
	}
}

func extractTrendScore(s *domain.PersistedSignal) float64 {
	score := s.DailyTrendAdj * 5 // amplify the ±20 range
	// Trend alignment check
	if s.DailyTrend != "" {
		aligned := (s.Direction == "BULLISH" && s.DailyTrend == "UP") ||
			(s.Direction == "BEARISH" && s.DailyTrend == "DOWN")
		if aligned {
			score += 30
		} else if s.DailyTrend != "FLAT" {
			score -= 30
		}
	}
	return clampF(score, -100, 100)
}

func extractVolScore(s *domain.PersistedSignal) float64 {
	// Use conviction score as a proxy for vol-adjusted confidence
	// Higher conviction in low-vol = positive, high vol = negative
	score := s.ConvictionScore - 50
	if s.Direction == "BEARISH" {
		score = -score
	}
	return clampF(score, -100, 100)
}

// --- Statistics Helpers ---

func zScoreNormalize(data []float64) []float64 {
	m := mean(data)
	s := stdDev(data, m)
	result := make([]float64, len(data))
	if s < 1e-10 {
		return result // All zeros if constant
	}
	for i, v := range data {
		result[i] = (v - m) / s
	}
	return result
}

func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func stdDev(data []float64, m float64) float64 {
	if len(data) < 2 {
		return 0
	}
	ss := 0.0
	for _, v := range data {
		d := v - m
		ss += d * d
	}
	// Use sample standard deviation (N-1) for unbiased estimation.
	return math.Sqrt(ss / float64(len(data)-1))
}

func adjustedRSquared(r2 float64, n, p int) float64 {
	if n <= p+1 {
		return 0
	}
	return 1 - (1-r2)*float64(n-1)/float64(n-p-1)
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// simpleOLS performs OLS regression and returns betas, R², and approximate p-values.
func simpleOLS(X [][]float64, y []float64) ([]float64, float64, []float64) {
	n := len(X)
	if n == 0 {
		return nil, 0, nil
	}
	p := len(X[0])

	// X'X
	XtX := make([][]float64, p)
	for i := range XtX {
		XtX[i] = make([]float64, p)
	}
	for i := 0; i < p; i++ {
		for j := 0; j < p; j++ {
			sum := 0.0
			for k := 0; k < n; k++ {
				sum += X[k][i] * X[k][j]
			}
			XtX[i][j] = sum
		}
	}

	// X'y
	Xty := make([]float64, p)
	for i := 0; i < p; i++ {
		sum := 0.0
		for k := 0; k < n; k++ {
			sum += X[k][i] * y[k]
		}
		Xty[i] = sum
	}

	// Solve via Gauss-Jordan
	inv, err := invertMatrix(XtX)
	if err != nil {
		// Fallback: return zeros
		return make([]float64, p), 0, make([]float64, p)
	}

	// beta = (X'X)^-1 X'y
	betas := make([]float64, p)
	for i := 0; i < p; i++ {
		sum := 0.0
		for j := 0; j < p; j++ {
			sum += inv[i][j] * Xty[j]
		}
		betas[i] = sum
	}

	// R²
	meanY := mean(y)
	ssTot := 0.0
	ssRes := 0.0
	for k := 0; k < n; k++ {
		pred := 0.0
		for j := 0; j < p; j++ {
			pred += X[k][j] * betas[j]
		}
		ssRes += (y[k] - pred) * (y[k] - pred)
		ssTot += (y[k] - meanY) * (y[k] - meanY)
	}
	r2 := 0.0
	if ssTot > 1e-10 {
		r2 = 1 - ssRes/ssTot
	}
	// Clamp R² to [0, 1] — negative R² means model is worse than mean
	if r2 < 0 {
		// Log warning: negative R² indicates poor model fit
		log.Warn().Float64("r2", r2).Msg("simpleOLS produced negative R², clamping to 0; model may be worse than mean predictor")
		r2 = 0
	}

	// Approximate p-values from t-statistics
	pValues := make([]float64, p)
	if n > p {
		mse := ssRes / float64(n-p)
		for j := 0; j < p; j++ {
			se := math.Sqrt(math.Abs(inv[j][j] * mse))
			if se > 1e-10 {
				tStat := betas[j] / se
				// Approximate two-tailed p-value using normal distribution
				pValues[j] = 2 * normalCDF(-math.Abs(tStat))
			} else {
				pValues[j] = 1.0
			}
		}
	}

	return betas, r2, pValues
}

// normalCDF approximation using Abramowitz and Stegun formula 7.1.26
func normalCDF(x float64) float64 {
	if x > 6 {
		return 1.0
	}
	if x < -6 {
		return 0.0
	}
	const (
		a1 = 0.254829592
		a2 = -0.284496736
		a3 = 1.421413741
		a4 = -1.453152027
		a5 = 1.061405429
		p  = 0.3275911
	)
	sign := 1.0
	if x < 0 {
		sign = -1.0
	}
	x = math.Abs(x) / math.Sqrt(2.0)
	t := 1.0 / (1.0 + p*x)
	y := 1.0 - (((((a5*t+a4)*t)+a3)*t+a2)*t+a1)*t*math.Exp(-x*x)
	return 0.5 * (1.0 + sign*y)
}

// invertMatrix inverts a square matrix using Gauss-Jordan elimination.
func invertMatrix(A [][]float64) ([][]float64, error) {
	n := len(A)
	// Augmented matrix [A | I]
	aug := make([][]float64, n)
	for i := 0; i < n; i++ {
		aug[i] = make([]float64, 2*n)
		copy(aug[i][:n], A[i])
		aug[i][n+i] = 1.0
	}

	for col := 0; col < n; col++ {
		// Find pivot
		maxRow := col
		maxVal := math.Abs(aug[col][col])
		for row := col + 1; row < n; row++ {
			if math.Abs(aug[row][col]) > maxVal {
				maxVal = math.Abs(aug[row][col])
				maxRow = row
			}
		}
		if maxVal < 1e-14 {
			return nil, fmt.Errorf("singular matrix")
		}
		aug[col], aug[maxRow] = aug[maxRow], aug[col]

		pivot := aug[col][col]
		for j := 0; j < 2*n; j++ {
			aug[col][j] /= pivot
		}

		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			factor := aug[row][col]
			for j := 0; j < 2*n; j++ {
				aug[row][j] -= factor * aug[col][j]
			}
		}
	}

	inv := make([][]float64, n)
	for i := 0; i < n; i++ {
		inv[i] = make([]float64, n)
		copy(inv[i], aug[i][n:])
	}
	return inv, nil
}
