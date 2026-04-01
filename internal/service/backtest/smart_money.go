package backtest

import (
	"context"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

// SmartMoneyAccuracy holds per-contract correlation between smart money
// position changes and subsequent price moves.
type SmartMoneyAccuracy struct {
	ContractCode string
	Currency     string
	TotalWeeks   int     // Weeks with both COT + price data
	Correct1W    int     // Weeks where net change direction matched price at +1W
	Correct2W    int
	Correct4W    int
	Accuracy1W   float64 // Correct1W / TotalWeeks * 100
	Accuracy2W   float64
	Accuracy4W   float64
	AvgReturnWhenFollow1W float64 // Avg % return when following smart money direction at 1W
	AvgReturnWhenFollow4W float64
	Correlation  float64 // Pearson correlation: net change vs 1W price change
	BestHorizon  string  // "1W", "2W", or "4W"
	BestAccuracy float64
	Edge         string  // "YES", "NO", "INSUFFICIENT"
}

// SmartMoneyAnalyzer computes smart money predictive accuracy per contract.
type SmartMoneyAnalyzer struct {
	cotRepo   ports.COTRepository
	priceRepo ports.PriceRepository
}

func NewSmartMoneyAnalyzer(cotRepo ports.COTRepository, priceRepo ports.PriceRepository) *SmartMoneyAnalyzer {
	return &SmartMoneyAnalyzer{cotRepo: cotRepo, priceRepo: priceRepo}
}

// Analyze computes SmartMoneyAccuracy for all 11 COT contracts.
func (a *SmartMoneyAnalyzer) Analyze(ctx context.Context) ([]SmartMoneyAccuracy, error) {
	var results []SmartMoneyAccuracy
	for _, contract := range domain.DefaultCOTContracts {
		acc, err := a.AnalyzeContract(ctx, contract)
		if err != nil {
			continue // skip contracts with insufficient data
		}
		if acc.TotalWeeks > 0 {
			results = append(results, *acc)
		}
	}
	// Sort by best accuracy descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].BestAccuracy > results[j].BestAccuracy
	})
	return results, nil
}

// AnalyzeContract computes SmartMoneyAccuracy for a single contract.
func (a *SmartMoneyAnalyzer) AnalyzeContract(ctx context.Context, contract domain.COTContract) (*SmartMoneyAccuracy, error) {
	// Get 52 weeks of COT records (newest-first)
	records, err := a.cotRepo.GetHistory(ctx, contract.Code, 52)
	if err != nil || len(records) < 4 {
		return nil, err
	}

	// Find price mapping for inverse flag
	mapping := domain.FindPriceMapping(contract.Code)
	if mapping == nil {
		return nil, nil
	}

	acc := &SmartMoneyAccuracy{
		ContractCode: contract.Code,
		Currency:     contract.Currency,
	}

	var netChanges []float64
	var priceChanges1W []float64
	var sumFollowReturn1W, sumFollowReturn4W float64
	var countFollow1W, countFollow4W int

	// Records are newest-first; iterate from oldest to newest for chronological order.
	// We need consecutive weeks to compute net change.
	for i := len(records) - 1; i >= 1; i-- {
		current := records[i-1] // newer
		// prev := records[i]     // older

		// Smart money net change this week (use API change fields)
		netChg := current.GetSmartMoneyNetChangeAPI(contract.ReportType)
		if netChg == 0 {
			// Fallback: compute manually from consecutive records
			curNet := current.GetSmartMoneyNet(contract.ReportType)
			prevNet := records[i].GetSmartMoneyNet(contract.ReportType)
			netChg = curNet - prevNet
		}

		if netChg == 0 {
			continue // no position change, skip
		}

		// Look up price at report date and at +1W, +2W, +4W
		entryPrice, err := a.priceRepo.GetPriceAt(ctx, contract.Code, current.ReportDate)
		if err != nil || entryPrice == nil || entryPrice.Close == 0 {
			continue
		}

		acc.TotalWeeks++
		smDirection := 1.0 // positive = smart money adding longs
		if netChg < 0 {
			smDirection = -1.0
		}

		// Check each horizon
		for _, horizon := range []struct {
			days    int
			correct *int
			label   string
		}{
			{7, &acc.Correct1W, "1W"},
			{14, &acc.Correct2W, "2W"},
			{28, &acc.Correct4W, "4W"},
		} {
			futureDate := current.ReportDate.AddDate(0, 0, horizon.days)
			futurePrice, err := a.priceRepo.GetPriceAt(ctx, contract.Code, futureDate)
			if err != nil || futurePrice == nil || futurePrice.Close == 0 {
				continue
			}

			priceChg := (futurePrice.Close - entryPrice.Close) / entryPrice.Close * 100
			// For inverse pairs, negate
			if mapping.Inverse {
				priceChg = -priceChg
			}

			priceDir := 1.0
			if priceChg < 0 {
				priceDir = -1.0
			}

			if smDirection == priceDir {
				*horizon.correct++
			}

			// Collect data for correlation and avg return
			if horizon.label == "1W" {
				netChanges = append(netChanges, netChg)
				priceChanges1W = append(priceChanges1W, priceChg)
				// "Following" smart money means going in their direction
				sumFollowReturn1W += priceChg * smDirection // positive if aligned
				countFollow1W++
			}
			if horizon.label == "4W" {
				sumFollowReturn4W += priceChg * smDirection
				countFollow4W++
			}
		}
	}

	if acc.TotalWeeks == 0 {
		acc.Edge = "INSUFFICIENT"
		return acc, nil
	}

	// Compute accuracy percentages
	acc.Accuracy1W = float64(acc.Correct1W) / float64(acc.TotalWeeks) * 100
	acc.Accuracy2W = float64(acc.Correct2W) / float64(acc.TotalWeeks) * 100
	acc.Accuracy4W = float64(acc.Correct4W) / float64(acc.TotalWeeks) * 100

	// Avg return when following
	if countFollow1W > 0 {
		acc.AvgReturnWhenFollow1W = sumFollowReturn1W / float64(countFollow1W)
	}
	if countFollow4W > 0 {
		acc.AvgReturnWhenFollow4W = sumFollowReturn4W / float64(countFollow4W)
	}

	// Pearson correlation
	if len(netChanges) >= 5 {
		r := pearsonCorrelation(netChanges, priceChanges1W)
		if !math.IsNaN(r) {
			acc.Correlation = r
		}
	}

	// Best horizon
	acc.BestHorizon = "1W"
	acc.BestAccuracy = acc.Accuracy1W
	if acc.Accuracy2W > acc.BestAccuracy {
		acc.BestHorizon = "2W"
		acc.BestAccuracy = acc.Accuracy2W
	}
	if acc.Accuracy4W > acc.BestAccuracy {
		acc.BestHorizon = "4W"
		acc.BestAccuracy = acc.Accuracy4W
	}

	// Edge determination
	if acc.TotalWeeks < 10 {
		acc.Edge = "INSUFFICIENT"
	} else if acc.BestAccuracy >= 55 {
		acc.Edge = "YES"
	} else {
		acc.Edge = "NO"
	}

	return acc, nil
}

// pearsonCorrelation computes Pearson correlation coefficient between two slices.
func pearsonCorrelation(x, y []float64) float64 {
	n := len(x)
	if n != len(y) || n < 5 {
		return math.NaN()
	}

	meanX := mathutil.Mean(x)
	meanY := mathutil.Mean(y)

	var sumXY, sumX2, sumY2 float64
	for i := 0; i < n; i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		sumXY += dx * dy
		sumX2 += dx * dx
		sumY2 += dy * dy
	}

	denom := math.Sqrt(sumX2 * sumY2)
	if denom == 0 {
		return 0
	}
	return sumXY / denom
}
