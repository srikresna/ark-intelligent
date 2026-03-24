package backtest

import (
	"context"
	"fmt"
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

// ---------------------------------------------------------------------------
// Logistic Regression Signal Calibration
// ---------------------------------------------------------------------------
//
// Multi-feature logistic regression for signal confidence calibration.
// Uses PersistedSignal fields as features to predict P(win) at each horizon.
//
// Features extracted from PersistedSignal:
//   x0: Normalized strength (1-5 → 0.0-1.0, clamped)
//   x1: Normalized raw confidence (0-100 → 0.0-1.0, clamped)
//   x2: COT index (0-100 → 0.0-1.0)
//   x3: Sentiment score (-1 to 1 → 0.0-1.0)
//   x4: Conviction score (0-100 → 0.0-1.0)
//   x5: Daily trend alignment (-1, 0, or +1)
//   x6: FRED regime encoding (-1.0 to +1.0)
//
// Training uses iterative reweighted least squares (IRLS) for
// logistic regression, with L2 regularization (ridge) to prevent overfitting.

// LogisticCalibrator trains and applies multi-feature logistic regression
// for signal win probability estimation.
type LogisticCalibrator struct {
	signalRepo ports.SignalRepository
}

// LogisticModel holds trained logistic regression weights.
type LogisticModel struct {
	Weights     []float64 `json:"weights"`      // Feature weights (including bias at index 0)
	NumFeatures int       `json:"num_features"` // Number of features (excluding bias)
	Horizon     string    `json:"horizon"`       // "1W", "2W", or "4W"
	SampleSize  int       `json:"sample_size"`   // Training set size
	Converged   bool      `json:"converged"`     // Whether IRLS converged
	Iterations  int       `json:"iterations"`    // Number of IRLS iterations used
	TrainAUC    float64   `json:"train_auc"`     // Approximate training AUC
	BrierScore  float64   `json:"brier_score"`   // Training Brier score
}

// CalibrationResult holds the output of logistic calibration for a signal.
type CalibrationResult struct {
	ProbWin1W      float64        `json:"prob_win_1w"`       // P(win) at 1 week
	ProbWin2W      float64        `json:"prob_win_2w"`       // P(win) at 2 weeks
	ProbWin4W      float64        `json:"prob_win_4w"`       // P(win) at 4 weeks
	BestHorizon    string         `json:"best_horizon"`      // Horizon with highest P(win)
	BestProb       float64        `json:"best_prob"`         // Highest P(win)
	Calibrated     float64        `json:"calibrated"`        // Calibrated confidence (0-100, at best horizon)
	Method         string         `json:"method"`            // "LOGISTIC", "PLATT", "WINRATE"
	Models         []*LogisticModel `json:"models,omitempty"` // The models used
}

// NewLogisticCalibrator creates a new calibrator.
func NewLogisticCalibrator(signalRepo ports.SignalRepository) *LogisticCalibrator {
	return &LogisticCalibrator{signalRepo: signalRepo}
}

// TrainModels trains logistic regression models for each horizon (1W, 2W, 4W).
// Returns trained models keyed by horizon string.
func (lc *LogisticCalibrator) TrainModels(ctx context.Context) (map[string]*LogisticModel, error) {
	signals, err := lc.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get training signals: %w", err)
	}

	models := make(map[string]*LogisticModel)

	// Train for each horizon
	for _, horizon := range []string{"1W", "2W", "4W"} {
		training := filterEvaluated(signals, horizon)
		if len(training) < 30 { // Minimum for meaningful logistic regression
			continue
		}

		model, err := trainLogistic(training, horizon)
		if err != nil {
			continue
		}
		models[horizon] = model
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("insufficient data to train any logistic model (need ≥30 evaluated signals per horizon)")
	}

	return models, nil
}

// Predict computes P(win) for a signal using trained models.
func (lc *LogisticCalibrator) Predict(signal *domain.PersistedSignal, models map[string]*LogisticModel) *CalibrationResult {
	features := extractFeatures(signal)
	result := &CalibrationResult{Method: "LOGISTIC"}

	bestProb := 0.0
	bestHorizon := "1W"

	for _, horizon := range []string{"1W", "2W", "4W"} {
		model, ok := models[horizon]
		if !ok {
			continue
		}
		prob := logisticPredict(features, model.Weights)
		switch horizon {
		case "1W":
			result.ProbWin1W = roundN(prob, 4)
		case "2W":
			result.ProbWin2W = roundN(prob, 4)
		case "4W":
			result.ProbWin4W = roundN(prob, 4)
		}
		if prob > bestProb {
			bestProb = prob
			bestHorizon = horizon
		}
	}

	result.BestHorizon = bestHorizon
	result.BestProb = roundN(bestProb, 4)
	result.Calibrated = roundN(bestProb*100, 2)

	return result
}

// --- Feature Extraction ---

const numFeatures = 7

// extractFeatures converts a PersistedSignal into a feature vector.
//
// Features:
//   x0: Normalized strength (1-5 → 0.0-1.0, clamped)
//   x1: Normalized raw confidence (0-100 → 0.0-1.0, clamped)
//   x2: COT index (0-100 → 0.0-1.0)
//   x3: Sentiment score (-1 to 1 → 0.0-1.0)
//   x4: Conviction score (0-100 → 0.0-1.0)
//   x5: Daily trend alignment (-1, 0, or +1)
//   x6: FRED regime encoding (-1.0 to +1.0)
func extractFeatures(s *domain.PersistedSignal) []float64 {
	features := make([]float64, numFeatures)

	// x0: Normalized strength (1-5 → 0.0-1.0)
	features[0] = float64(s.Strength-1) / 4.0
	if features[0] < 0 {
		features[0] = 0
	}
	if features[0] > 1 {
		features[0] = 1
	}

	// x1: Normalized raw confidence (0-100 → 0.0-1.0)
	conf := s.RawConfidence
	if conf == 0 {
		conf = s.Confidence
	}
	features[1] = conf / 100.0
	if features[1] > 1 {
		features[1] = 1
	}
	if features[1] < 0 {
		features[1] = 0
	}

	// x2: COT index (0-100 → 0.0-1.0)
	features[2] = s.COTIndex / 100.0

	// x3: Sentiment score (-1 to 1 → 0.0-1.0)
	features[3] = (s.SentimentScore + 1) / 2.0

	// x4: Conviction score (0-100 → 0.0-1.0)
	features[4] = s.ConvictionScore / 100.0

	// x5: Daily trend alignment
	features[5] = encodeTrendAlignment(s.Direction, s.DailyTrend)

	// x6: FRED regime encoding
	features[6] = encodeFREDRegime(s.FREDRegime)

	return features
}

// encodeTrendAlignment returns +1 if signal and trend align, -1 if opposed, 0 if flat.
func encodeTrendAlignment(direction, dailyTrend string) float64 {
	if dailyTrend == "" || dailyTrend == "FLAT" {
		return 0
	}
	aligned := (direction == "BULLISH" && dailyTrend == "UP") ||
		(direction == "BEARISH" && dailyTrend == "DOWN")
	if aligned {
		return 1
	}
	return -1
}

// encodeFREDRegime maps FRED regime strings to numeric values.
func encodeFREDRegime(regime string) float64 {
	switch regime {
	case "EXPANSION", "GOLDILOCKS":
		return 1.0
	case "STRESS", "RECESSION", "STAGFLATION":
		return -1.0
	case "TIGHTENING":
		return -0.5
	default:
		return 0.0
	}
}

// --- Logistic Regression Training (IRLS) ---

// trainLogistic fits a logistic regression model using IRLS.
func trainLogistic(signals []domain.PersistedSignal, horizon string) (*LogisticModel, error) {
	n := len(signals)

	// Build feature matrix X (n × numFeatures+1, with bias column)
	// and target vector y
	X := make([][]float64, n)
	y := make([]float64, n)

	for i, s := range signals {
		features := extractFeatures(&s)
		// Prepend bias term
		row := make([]float64, numFeatures+1)
		row[0] = 1.0 // bias
		copy(row[1:], features)
		X[i] = row

		// Target: 1 if WIN, 0 if LOSS
		outcome := outcomeForHorizon(&s, horizon)
		if outcome == domain.OutcomeWin {
			y[i] = 1.0
		}
	}

	// Initialize weights to zero
	nCols := numFeatures + 1
	w := make([]float64, nCols)

	// IRLS iterations
	maxIter := 25
	lambda := 0.01 // L2 regularization
	converged := false
	var iter int

	for iter = 0; iter < maxIter; iter++ {
		// Compute predictions p = sigmoid(X*w)
		p := make([]float64, n)
		for i := range X {
			z := dotProduct(X[i], w)
			p[i] = sigmoid(z)
			// Clamp to avoid numerical issues
			if p[i] < 1e-6 {
				p[i] = 1e-6
			}
			if p[i] > 1-1e-6 {
				p[i] = 1 - 1e-6
			}
		}

		// Compute gradient: X^T * (y - p) - lambda*w
		grad := make([]float64, nCols)
		for j := 0; j < nCols; j++ {
			for i := 0; i < n; i++ {
				grad[j] += X[i][j] * (y[i] - p[i])
			}
			if j > 0 { // Don't regularize bias
				grad[j] -= lambda * w[j]
			}
		}

		// Compute Hessian diagonal (Newton approximation)
		// H_jj ≈ -Σ x_ij² * p_i * (1-p_i) - lambda
		hDiag := make([]float64, nCols)
		for j := 0; j < nCols; j++ {
			for i := 0; i < n; i++ {
				hDiag[j] -= X[i][j] * X[i][j] * p[i] * (1 - p[i])
			}
			if j > 0 {
				hDiag[j] -= lambda
			}
			if math.Abs(hDiag[j]) < 1e-10 {
				hDiag[j] = -1e-10
			}
		}

		// Newton update: w += -H^{-1} * grad (diagonal approx)
		maxDelta := 0.0
		for j := 0; j < nCols; j++ {
			delta := -grad[j] / hDiag[j]
			// Step size limiting
			if delta > 1.0 {
				delta = 1.0
			} else if delta < -1.0 {
				delta = -1.0
			}
			w[j] += delta
			if math.Abs(delta) > maxDelta {
				maxDelta = math.Abs(delta)
			}
		}

		// Check convergence
		if maxDelta < 1e-5 {
			converged = true
			break
		}
	}

	// Compute training metrics
	predictions := make([]float64, n)
	outcomes := make([]bool, n)
	for i := range X {
		predictions[i] = sigmoid(dotProduct(X[i], w))
		outcomes[i] = y[i] > 0.5
	}

	brier := mathutil.BrierScore(predictions, outcomes)
	auc := approximateAUC(predictions, y)

	return &LogisticModel{
		Weights:     w,
		NumFeatures: numFeatures,
		Horizon:     horizon,
		SampleSize:  n,
		Converged:   converged,
		Iterations:  iter,
		TrainAUC:    roundN(auc, 4),
		BrierScore:  roundN(brier, 4),
	}, nil
}

// logisticPredict computes P(y=1|x) = sigmoid(w^T * [1, x])
func logisticPredict(features []float64, weights []float64) float64 {
	if len(weights) != len(features)+1 {
		return 0.5 // Fallback
	}
	z := weights[0] // bias
	for i, f := range features {
		z += weights[i+1] * f
	}
	return sigmoid(z)
}

// --- Helpers ---

func sigmoid(z float64) float64 {
	if z > 500 {
		return 1.0
	}
	if z < -500 {
		return 0.0
	}
	return 1.0 / (1.0 + math.Exp(-z))
}

func dotProduct(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var sum float64
	for i := 0; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func outcomeForHorizon(s *domain.PersistedSignal, horizon string) string {
	switch horizon {
	case "1W":
		return s.Outcome1W
	case "2W":
		return s.Outcome2W
	case "4W":
		return s.Outcome4W
	default:
		return ""
	}
}

func filterEvaluated(signals []domain.PersistedSignal, horizon string) []domain.PersistedSignal {
	var result []domain.PersistedSignal
	for _, s := range signals {
		outcome := outcomeForHorizon(&s, horizon)
		if outcome == domain.OutcomeWin || outcome == domain.OutcomeLoss {
			result = append(result, s)
		}
	}
	return result
}

// approximateAUC computes an approximate AUC-ROC by sorting predictions.
func approximateAUC(predictions, targets []float64) float64 {
	n := len(predictions)
	if n == 0 {
		return 0.5
	}

	// Count positives and negatives
	var nPos, nNeg int
	for _, t := range targets {
		if t > 0.5 {
			nPos++
		} else {
			nNeg++
		}
	}
	if nPos == 0 || nNeg == 0 {
		return 0.5
	}

	// Mann-Whitney U statistic
	var concordant float64
	for i := 0; i < n; i++ {
		if targets[i] < 0.5 {
			continue
		}
		for j := 0; j < n; j++ {
			if targets[j] > 0.5 {
				continue
			}
			if predictions[i] > predictions[j] {
				concordant++
			} else if predictions[i] == predictions[j] {
				concordant += 0.5
			}
		}
	}

	return concordant / float64(nPos*nNeg)
}

// roundN rounds v to n decimal places.
func roundN(v float64, n int) float64 {
	pow := math.Pow(10, float64(n))
	return math.Round(v*pow) / pow
}
