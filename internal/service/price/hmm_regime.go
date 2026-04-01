package price

import (
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Hidden Markov Model (HMM) — Regime-Switching Detection
// ---------------------------------------------------------------------------
//
// 3-state HMM for market regime detection:
//   State 0: RISK_ON   — low vol, positive drift, trending
//   State 1: RISK_OFF  — moderate vol, no drift, ranging
//   State 2: CRISIS    — high vol, negative drift, dislocations
//
// Observations: daily log-returns discretized into emission bins.
// Training: Baum-Welch (EM) for parameter estimation.
// Inference: Forward algorithm for state probabilities, Viterbi for path.

const (
	HMMRiskOn  = "RISK_ON"
	HMMRiskOff = "RISK_OFF"
	HMMCrisis  = "CRISIS"

	hmmNumStates   = 3
	hmmNumEmissions = 5 // discretized return bins
)

// HMMResult holds the output of HMM regime analysis.
type HMMResult struct {
	CurrentState      string    `json:"current_state"`       // RISK_ON, RISK_OFF, CRISIS
	StateProbabilities [3]float64 `json:"state_probabilities"` // P(state) at current time
	TransitionMatrix  [3][3]float64 `json:"transition_matrix"`  // Estimated transition probs
	ViterbiPath       []string  `json:"viterbi_path,omitempty"` // Last N states (most likely path)
	TransitionWarning string    `json:"transition_warning,omitempty"` // Early warning if regime change likely
	SampleSize        int       `json:"sample_size"`
	Converged         bool      `json:"converged"`
	Iterations        int       `json:"iterations"`
}

// HMMModel holds fitted HMM parameters.
type HMMModel struct {
	Pi [hmmNumStates]float64                       // Initial state distribution
	A  [hmmNumStates][hmmNumStates]float64         // Transition matrix
	B  [hmmNumStates][hmmNumEmissions]float64      // Emission matrix
}

// EstimateHMMRegime fits a 3-state HMM to daily returns and infers the current regime.
// Prices must be newest-first. Requires at least 60 observations.
func EstimateHMMRegime(prices []domain.PriceRecord) (*HMMResult, error) {
	if len(prices) < 60 {
		return nil, fmt.Errorf("insufficient data for HMM: need 60, got %d", len(prices))
	}

	// Compute log-returns chronologically (oldest first)
	n := len(prices)
	returns := make([]float64, 0, n-1)
	for i := n - 1; i > 0; i-- {
		if prices[i].Close <= 0 || prices[i-1].Close <= 0 {
			continue
		}
		returns = append(returns, math.Log(prices[i-1].Close/prices[i].Close))
	}
	if len(returns) < 60 {
		return nil, fmt.Errorf("insufficient valid returns for HMM: need 60, got %d", len(returns))
	}

	// Discretize returns into emission symbols
	obs := discretizeReturns(returns)

	// Initialize HMM with domain-informed priors
	model := initHMMPriors()

	// Baum-Welch training (max 100 iterations, convergence threshold 1e-6)
	maxIter := 100
	converged := false
	var iter int
	prevLogLik := math.Inf(-1)
	for iter = 0; iter < maxIter; iter++ {
		newModel, logLik := baumWelchStep(&model, obs)
		model = newModel
		if iter > 0 && math.Abs(logLik-prevLogLik) < 1e-6 {
			converged = true
			break
		}
		prevLogLik = logLik
	}

	if !converged {
		return nil, fmt.Errorf("insufficient data for regime: Baum-Welch did not converge after %d iterations", maxIter)
	}

	// Forward algorithm for current state probabilities
	stateProbs := forwardFilter(&model, obs)

	// Viterbi for most likely state path (last 20 observations)
	viterbiStates := viterbi(&model, obs)
	pathLen := 20
	if len(viterbiStates) < pathLen {
		pathLen = len(viterbiStates)
	}
	viterbiPath := make([]string, pathLen)
	for i := 0; i < pathLen; i++ {
		viterbiPath[i] = stateLabel(viterbiStates[len(viterbiStates)-pathLen+i])
	}

	// Current state = highest probability
	currentState := 0
	maxProb := stateProbs[0]
	for s := 1; s < hmmNumStates; s++ {
		if stateProbs[s] > maxProb {
			maxProb = stateProbs[s]
			currentState = s
		}
	}

	// Transition warning
	warning := detectTransitionWarning(currentState, stateProbs, model.A)

	result := &HMMResult{
		CurrentState:      stateLabel(currentState),
		StateProbabilities: stateProbs,
		TransitionMatrix:  model.A,
		ViterbiPath:       viterbiPath,
		TransitionWarning: warning,
		SampleSize:        len(returns),
		Converged:         converged,
		Iterations:        iter,
	}

	return result, nil
}

// --- Discretization ---

// discretizeReturns maps continuous returns to discrete emission symbols (0-4).
// Bins: [very negative, negative, neutral, positive, very positive]
func discretizeReturns(returns []float64) []int {
	// Compute thresholds from data (quantile-based)
	sorted := make([]float64, len(returns))
	copy(sorted, returns)
	sortFloat64s(sorted)

	p20 := sorted[len(sorted)*20/100]
	p40 := sorted[len(sorted)*40/100]
	p60 := sorted[len(sorted)*60/100]
	p80 := sorted[len(sorted)*80/100]

	obs := make([]int, len(returns))
	for i, r := range returns {
		switch {
		case r <= p20:
			obs[i] = 0 // very negative
		case r <= p40:
			obs[i] = 1 // negative
		case r <= p60:
			obs[i] = 2 // neutral
		case r <= p80:
			obs[i] = 3 // positive
		default:
			obs[i] = 4 // very positive
		}
	}
	return obs
}

func sortFloat64s(data []float64) {
	sort.Float64s(data)
}

// --- HMM Initialization ---

// initHMMPriors creates an HMM with domain-informed initial parameters.
func initHMMPriors() HMMModel {
	var m HMMModel

	// Initial state: most likely risk-off
	m.Pi = [3]float64{0.40, 0.45, 0.15}

	// Transition matrix: regimes are sticky (high self-transition)
	// RISK_ON tends to stay, occasionally shifts to RISK_OFF
	// CRISIS is rare but sticky when entered
	m.A = [3][3]float64{
		{0.90, 0.08, 0.02}, // RISK_ON → mostly stays
		{0.10, 0.82, 0.08}, // RISK_OFF → can go either way
		{0.05, 0.20, 0.75}, // CRISIS → sticky but can recover to RISK_OFF
	}

	// Emission matrix: map states to return distribution
	// RISK_ON: skewed positive (more positive returns)
	m.B[0] = [5]float64{0.05, 0.10, 0.25, 0.35, 0.25}
	// RISK_OFF: symmetric around neutral
	m.B[1] = [5]float64{0.15, 0.25, 0.30, 0.20, 0.10}
	// CRISIS: skewed negative (more extreme negative returns)
	m.B[2] = [5]float64{0.35, 0.25, 0.20, 0.12, 0.08}

	return m
}

// --- Baum-Welch (EM) ---

func baumWelchStep(m *HMMModel, obs []int) (HMMModel, float64) {
	T := len(obs)
	N := hmmNumStates

	// Forward pass
	alpha := make([][hmmNumStates]float64, T)
	scale := make([]float64, T)

	// t=0
	for i := 0; i < N; i++ {
		alpha[0][i] = m.Pi[i] * m.B[i][obs[0]]
		scale[0] += alpha[0][i]
	}
	if scale[0] > 0 {
		for i := 0; i < N; i++ {
			alpha[0][i] /= scale[0]
		}
	}

	// t=1..T-1
	for t := 1; t < T; t++ {
		for j := 0; j < N; j++ {
			sum := 0.0
			for i := 0; i < N; i++ {
				sum += alpha[t-1][i] * m.A[i][j]
			}
			alpha[t][j] = sum * m.B[j][obs[t]]
			scale[t] += alpha[t][j]
		}
		if scale[t] > 0 {
			for j := 0; j < N; j++ {
				alpha[t][j] /= scale[t]
			}
		}
	}

	// Log-likelihood
	logLikOld := 0.0
	for t := 0; t < T; t++ {
		if scale[t] > 0 {
			logLikOld += math.Log(scale[t])
		}
	}

	// Backward pass
	beta := make([][hmmNumStates]float64, T)
	for i := 0; i < N; i++ {
		beta[T-1][i] = 1.0
	}
	for t := T - 2; t >= 0; t-- {
		for i := 0; i < N; i++ {
			sum := 0.0
			for j := 0; j < N; j++ {
				sum += m.A[i][j] * m.B[j][obs[t+1]] * beta[t+1][j]
			}
			beta[t][i] = sum
			if scale[t+1] > 0 {
				beta[t][i] /= scale[t+1]
			}
		}
	}

	// Compute gamma and xi
	gamma := make([][hmmNumStates]float64, T)
	xi := make([][hmmNumStates][hmmNumStates]float64, T-1)

	for t := 0; t < T; t++ {
		sum := 0.0
		for i := 0; i < N; i++ {
			gamma[t][i] = alpha[t][i] * beta[t][i]
			sum += gamma[t][i]
		}
		if sum > 0 {
			for i := 0; i < N; i++ {
				gamma[t][i] /= sum
			}
		}
	}

	for t := 0; t < T-1; t++ {
		sum := 0.0
		for i := 0; i < N; i++ {
			for j := 0; j < N; j++ {
				xi[t][i][j] = alpha[t][i] * m.A[i][j] * m.B[j][obs[t+1]] * beta[t+1][j]
				sum += xi[t][i][j]
			}
		}
		if sum > 0 {
			for i := 0; i < N; i++ {
				for j := 0; j < N; j++ {
					xi[t][i][j] /= sum
				}
			}
		}
	}

	// Re-estimate parameters
	var newModel HMMModel

	// Pi
	for i := 0; i < N; i++ {
		newModel.Pi[i] = gamma[0][i]
	}

	// A (transition)
	for i := 0; i < N; i++ {
		sumGamma := 0.0
		for t := 0; t < T-1; t++ {
			sumGamma += gamma[t][i]
		}
		for j := 0; j < N; j++ {
			sumXi := 0.0
			for t := 0; t < T-1; t++ {
				sumXi += xi[t][i][j]
			}
			if sumGamma > 0 {
				newModel.A[i][j] = sumXi / sumGamma
			} else {
				newModel.A[i][j] = m.A[i][j] // Keep prior
			}
		}
		// Normalize row
		rowSum := 0.0
		for j := 0; j < N; j++ {
			rowSum += newModel.A[i][j]
		}
		if rowSum > 0 {
			for j := 0; j < N; j++ {
				newModel.A[i][j] /= rowSum
			}
		}
	}

	// B (emission)
	for i := 0; i < N; i++ {
		sumGamma := 0.0
		for t := 0; t < T; t++ {
			sumGamma += gamma[t][i]
		}
		for k := 0; k < hmmNumEmissions; k++ {
			sumObs := 0.0
			for t := 0; t < T; t++ {
				if obs[t] == k {
					sumObs += gamma[t][i]
				}
			}
			if sumGamma > 0 {
				newModel.B[i][k] = sumObs / sumGamma
			} else {
				newModel.B[i][k] = m.B[i][k]
			}
			// Floor to prevent zero emissions
			if newModel.B[i][k] < 1e-6 {
				newModel.B[i][k] = 1e-6
			}
		}
		// Normalize
		rowSum := 0.0
		for k := 0; k < hmmNumEmissions; k++ {
			rowSum += newModel.B[i][k]
		}
		if rowSum > 0 {
			for k := 0; k < hmmNumEmissions; k++ {
				newModel.B[i][k] /= rowSum
			}
		}
	}

	return newModel, logLikOld
}

// --- Forward Filter ---

// forwardFilter returns filtered state probabilities at the last observation.
func forwardFilter(m *HMMModel, obs []int) [hmmNumStates]float64 {
	T := len(obs)
	N := hmmNumStates

	alpha := make([]float64, N)
	for i := 0; i < N; i++ {
		alpha[i] = m.Pi[i] * m.B[i][obs[0]]
	}
	normalize(alpha)

	for t := 1; t < T; t++ {
		newAlpha := make([]float64, N)
		for j := 0; j < N; j++ {
			sum := 0.0
			for i := 0; i < N; i++ {
				sum += alpha[i] * m.A[i][j]
			}
			newAlpha[j] = sum * m.B[j][obs[t]]
		}
		normalize(newAlpha)
		alpha = newAlpha
	}

	var result [hmmNumStates]float64
	copy(result[:], alpha)
	return result
}

// --- Viterbi ---

// viterbi returns the most likely state sequence.
func viterbi(m *HMMModel, obs []int) []int {
	T := len(obs)
	N := hmmNumStates

	// delta[t][i] = max probability of being in state i at time t
	delta := make([][hmmNumStates]float64, T)
	psi := make([][hmmNumStates]int, T)

	// Init
	for i := 0; i < N; i++ {
		delta[0][i] = math.Log(m.Pi[i]+1e-12) + math.Log(m.B[i][obs[0]]+1e-12)
	}

	// Recurse
	for t := 1; t < T; t++ {
		for j := 0; j < N; j++ {
			maxVal := math.Inf(-1)
			maxIdx := 0
			for i := 0; i < N; i++ {
				val := delta[t-1][i] + math.Log(m.A[i][j]+1e-12)
				if val > maxVal {
					maxVal = val
					maxIdx = i
				}
			}
			delta[t][j] = maxVal + math.Log(m.B[j][obs[t]]+1e-12)
			psi[t][j] = maxIdx
		}
	}

	// Backtrack
	path := make([]int, T)
	maxVal := math.Inf(-1)
	for i := 0; i < N; i++ {
		if delta[T-1][i] > maxVal {
			maxVal = delta[T-1][i]
			path[T-1] = i
		}
	}
	for t := T - 2; t >= 0; t-- {
		path[t] = psi[t+1][path[t+1]]
	}

	return path
}

// --- Helpers ---

func normalize(v []float64) {
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	if sum > 0 {
		for i := range v {
			v[i] /= sum
		}
	} else {
		// Fallback: uniform distribution to avoid all-zero cascading
		n := float64(len(v))
		for i := range v {
			v[i] = 1.0 / n
		}
	}
}

func stateLabel(s int) string {
	switch s {
	case 0:
		return HMMRiskOn
	case 1:
		return HMMRiskOff
	case 2:
		return HMMCrisis
	default:
		return "UNKNOWN"
	}
}

// detectTransitionWarning checks if a regime change is likely in the next period.
func detectTransitionWarning(currentState int, probs [3]float64, A [3][3]float64) string {
	// One-step-ahead state probabilities
	var nextProbs [3]float64
	for j := 0; j < hmmNumStates; j++ {
		for i := 0; i < hmmNumStates; i++ {
			nextProbs[j] += probs[i] * A[i][j]
		}
	}

	// Check if a different state is becoming likely
	nextBest := 0
	for j := 1; j < hmmNumStates; j++ {
		if nextProbs[j] > nextProbs[nextBest] {
			nextBest = j
		}
	}

	if nextBest != currentState && nextProbs[nextBest] > 0.30 {
		return fmt.Sprintf("Potential shift to %s (P=%.0f%%)", stateLabel(nextBest), nextProbs[nextBest]*100)
	}

	// Check crisis probability specifically
	if currentState != 2 && nextProbs[2] > 0.15 {
		return fmt.Sprintf("Elevated CRISIS probability (P=%.0f%%)", nextProbs[2]*100)
	}

	return ""
}

// HMMConfidenceMultiplier returns a signal confidence multiplier based on HMM regime.
func HMMConfidenceMultiplier(h *HMMResult) float64 {
	if h == nil {
		return 1.0
	}
	switch h.CurrentState {
	case HMMCrisis:
		return 0.70 // Signals less reliable in crisis
	case HMMRiskOff:
		return 0.90 // Slightly reduce in risk-off
	case HMMRiskOn:
		return 1.05 // Slight boost in risk-on
	default:
		return 1.0
	}
}
