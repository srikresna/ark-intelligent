package price

import (
	"context"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// RiskContextBuilder computes VIX + S&P 500 risk sentiment context from stored prices.
type RiskContextBuilder struct {
	priceRepo ports.PriceRepository
}

// NewRiskContextBuilder creates a new risk context builder.
func NewRiskContextBuilder(priceRepo ports.PriceRepository) *RiskContextBuilder {
	return &RiskContextBuilder{priceRepo: priceRepo}
}

// Build computes the current risk context from stored VIX and SPX price records.
// Returns nil, nil if VIX data is unavailable — callers should treat nil RiskContext
// as "no adjustment" (multiplier = 1.0). SPX data is optional — missing SPX only
// disables the SPX trend modifier, not the whole risk context.
func (rb *RiskContextBuilder) Build(ctx context.Context) (*domain.RiskContext, error) {
	vixRecords, err := rb.priceRepo.GetHistory(ctx, "risk_VIX", 4)
	if err != nil || len(vixRecords) == 0 {
		// VIX is required — without it we cannot compute the regime.
		// Return nil, nil (not an error) so callers skip adjustment gracefully.
		return nil, nil
	}

	// SPX is optional — fetch best-effort, missing data just disables SPX modifier.
	spxRecords, _ := rb.priceRepo.GetHistory(ctx, "risk_SPX", 5)

	rc := &domain.RiskContext{}

	// --- VIX ---
	rc.VIXLevel = vixRecords[0].Close

	// 4-week VIX average
	var sumVIX float64
	for _, r := range vixRecords {
		sumVIX += r.Close
	}
	rc.VIX4WAvg = roundN(sumVIX/float64(len(vixRecords)), 2)

	// VIX trend: compare current vs 4W avg
	if rc.VIXLevel > rc.VIX4WAvg*1.05 {
		rc.VIXTrend = "RISING"
	} else if rc.VIXLevel < rc.VIX4WAvg*0.95 {
		rc.VIXTrend = "FALLING"
	} else {
		rc.VIXTrend = "STABLE"
	}

	rc.Regime = domain.ClassifyRiskRegime(rc.VIXLevel)

	// --- SPX (optional) ---
	if len(spxRecords) >= 2 && spxRecords[1].Close > 0 {
		rc.SPXWeeklyChg = roundN((spxRecords[0].Close-spxRecords[1].Close)/spxRecords[1].Close*100, 4)
	}
	if len(spxRecords) >= 5 && spxRecords[4].Close > 0 {
		rc.SPXMonthlyChg = roundN((spxRecords[0].Close-spxRecords[4].Close)/spxRecords[4].Close*100, 4)
	}
	if len(spxRecords) >= 4 {
		var sumSPX float64
		for i := 0; i < 4; i++ {
			sumSPX += spxRecords[i].Close
		}
		ma4w := sumSPX / 4
		rc.SPXAboveMA4W = spxRecords[0].Close > ma4w
	}

	return rc, nil
}
