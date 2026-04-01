package fred

import (
	"math"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ComputeComposites derives all composite scores from raw MacroData.
//
// Returns nil if data is nil.
// Always returns a fully-populated (non-nil) struct for non-nil input; all
// score fields default to 0 when the underlying data is missing or zero-valued.
// This function never panics regardless of the input state.
func ComputeComposites(data *MacroData) *domain.MacroComposites {
	if data == nil {
		return nil
	}

	c := &domain.MacroComposites{
		ComputedAt: time.Now(),
	}

	c.LaborHealth = computeLaborHealth(data)
	c.LaborLabel = domain.LaborHealthLabel(c.LaborHealth)

	c.InflationMomentum = computeInflationMomentum(data)
	c.InflationLabel = domain.InflationMomentumLabel(c.InflationMomentum)

	c.YieldCurveSignal = computeYieldCurveSignal(data)

	c.CreditStress = computeCreditStress(data)
	c.CreditLabel = domain.CreditStressLabel(c.CreditStress)

	c.HousingPulse = computeHousingPulse(data)
	c.FinConditions = computeFinancialConditions(data)

	// Per-country macro scores (-100 to +100).
	// Each sub-function returns 0 when all input data is missing.
	c.USScore = computeUSMacroScore(data)
	ezRate := data.EZ_Rate
	if data.EZ_10Y != 0 {
		ezRate = data.EZ_10Y // prefer 10Y as it's more comparable across countries
	}
	c.EZScore = computeCountryScore(data.EZ_CPI, data.EZ_GDP, data.EZ_Unemployment, ezRate, 2.0, 0.3, 6.5)
	c.UKScore = computeCountryScore(data.UK_CPI, data.UK_IndustrialProd, data.UK_Unemployment, 0, 2.0, 2.0, 4.5)
	c.JPScore = computeCountryScore(data.JP_CPI, data.JP_IndustrialProd, data.JP_Unemployment, data.JP_10Y, 2.0, 1.5, 2.8)
	c.AUScore = computeCountryScore(data.AU_CPI, 0, data.AU_Unemployment, 0, 2.5, 0.5, 4.5)
	c.CAScore = computeCountryScore(data.CA_CPI, 0, data.CA_Unemployment, 0, 2.0, 0.3, 6.0)
	c.NZScore = computeCountryScore(data.NZ_CPI, 0, 0, 0, 2.0, 0.5, 4.5)

	// Defensive NaN/Inf guard: score functions should never produce non-finite
	// values, but guard here to prevent downstream scoring math from propagating
	// NaN/Inf into conviction scores or regime classification.
	sanitizeCompositeScores(c)

	// Sentiment composite
	c.SentimentComposite = computeSentimentComposite(data)
	c.SentimentLabel = domain.SentimentCompositeLabel(c.SentimentComposite)

	// VIX term structure
	c.VIXTermRatio = data.VIXTermRatio
	c.VIXTermRegime = data.VIXTermRegime
	if c.VIXTermRegime == "" {
		c.VIXTermRegime = "N/A"
	}

	return c
}

// sanitizeCompositeScores clamps all numeric composite fields to finite values.
// Any NaN or Inf produced by upstream math is zeroed out to prevent propagation.
func sanitizeCompositeScores(c *domain.MacroComposites) {
	c.USScore = sanitizeScore(c.USScore)
	c.EZScore = sanitizeScore(c.EZScore)
	c.UKScore = sanitizeScore(c.UKScore)
	c.JPScore = sanitizeScore(c.JPScore)
	c.AUScore = sanitizeScore(c.AUScore)
	c.CAScore = sanitizeScore(c.CAScore)
	c.NZScore = sanitizeScore(c.NZScore)
	c.LaborHealth = sanitizeScore(c.LaborHealth)
	c.CreditStress = sanitizeScore(c.CreditStress)
	c.InflationMomentum = sanitizeScore(c.InflationMomentum)
	c.FinConditions = sanitizeScore(c.FinConditions)
	c.SentimentComposite = sanitizeScore(c.SentimentComposite)
}

// sanitizeScore returns 0 for any non-finite (NaN, ±Inf) value, otherwise
// returns the value unchanged. All composite scores should be finite.
func sanitizeScore(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// computeLaborHealth returns a 0-100 score for US labor market health.
func computeLaborHealth(data *MacroData) float64 {
	var total, weights float64

	// Initial Claims (20% weight) — lower = better
	if data.InitialClaims > 0 {
		claimsScore := mapRange(data.InitialClaims, 350_000, 180_000, 0, 100)
		total += claimsScore * 20
		weights += 20
	}

	// Continuing Claims (10% weight) — lower = better
	if data.ContinuingClaims > 0 {
		ccScore := mapRange(data.ContinuingClaims, 2_500_000, 1_500_000, 0, 100)
		total += ccScore * 10
		weights += 10
	}

	// Unemployment U3 (15% weight) — lower = better
	if data.UnemployRate > 0 {
		uScore := mapRange(data.UnemployRate, 7.0, 3.0, 0, 100)
		total += uScore * 15
		weights += 15
	}

	// U6 Unemployment (10% weight) — lower = better
	if data.U6Unemployment > 0 {
		u6Score := mapRange(data.U6Unemployment, 12.0, 6.0, 0, 100)
		total += u6Score * 10
		weights += 10
	}

	// JOLTS Openings (20% weight) — higher = better
	if data.JOLTSOpenings > 0 {
		joltsScore := mapRange(data.JOLTSOpenings, 6_000, 12_000, 0, 100)
		total += joltsScore * 20
		weights += 20
	}

	// JOLTS Quit Rate (15% weight) — higher = better (workers confident)
	if data.JOLTSQuitRate > 0 {
		quitScore := mapRange(data.JOLTSQuitRate, 1.5, 3.0, 0, 100)
		total += quitScore * 15
		weights += 15
	}

	// Employment-Population Ratio (10% weight) — higher = better
	if data.EmpPopRatio > 0 {
		empScore := mapRange(data.EmpPopRatio, 55.0, 62.0, 0, 100)
		total += empScore * 10
		weights += 10
	}

	// Sahm Rule override: if triggered, cap at 20
	if data.SahmRule >= 0.5 {
		if weights > 0 {
			score := total / weights
			if score > 20 {
				return 20
			}
			return score
		}
		return 20
	}

	if weights == 0 {
		return 50 // no data, assume neutral
	}
	return clamp(total/weights, 0, 100)
}

// computeInflationMomentum returns -1.0 to +1.0 (positive = accelerating).
func computeInflationMomentum(data *MacroData) float64 {
	var total, weights float64

	// Core PCE vs target (25%)
	if data.CorePCE > 0 {
		pceScore := mapRange(data.CorePCE, 1.5, 3.5, -1.0, 1.0)
		total += pceScore * 25
		weights += 25
	}

	// Median CPI (20%)
	if data.MedianCPI > 0 {
		medScore := mapRange(data.MedianCPI, 2.0, 5.0, -1.0, 1.0)
		total += medScore * 20
		weights += 20
	}

	// Sticky CPI (15%)
	if data.StickyCPI > 0 {
		stickyScore := mapRange(data.StickyCPI, 2.0, 6.0, -1.0, 1.0)
		total += stickyScore * 15
		weights += 15
	}

	// PPI Commodities (5%)
	if data.PPICommodities != 0 {
		ppiScore := mapRange(data.PPICommodities, -5.0, 10.0, -1.0, 1.0)
		total += ppiScore * 5
		weights += 5
	}

	// PPI Finished Goods core (5% — secondary confirmation)
	if data.PPIFinished != 0 {
		ppiFinScore := mapRange(data.PPIFinished, -3.0, 8.0, -1.0, 1.0)
		total += ppiFinScore * 5
		weights += 5
	}

	// Market breakevens average (15%)
	be := data.Breakeven5Y
	if be > 0 {
		beScore := mapRange(be, 1.5, 3.0, -1.0, 1.0)
		total += beScore * 15
		weights += 15
	}

	// Consumer expectations (10%)
	if data.MichInflExp1Y > 0 {
		expScore := mapRange(data.MichInflExp1Y, 2.0, 5.0, -1.0, 1.0)
		total += expScore * 10
		weights += 10
	}

	// Cleveland Fed Expected Inflation 1Y (5%)
	if data.ClevelandInfExp1Y > 0 {
		clevScore := mapRange(data.ClevelandInfExp1Y, 1.5, 4.0, -1.0, 1.0)
		total += clevScore * 5
		weights += 5
	}

	if weights == 0 {
		return 0
	}
	return clamp(total/weights, -1.0, 1.0)
}

// computeYieldCurveSignal classifies the yield curve state.
func computeYieldCurveSignal(data *MacroData) string {
	spread2Y10Y := data.YieldSpread
	spread3M10Y := data.Spread3M10Y

	// Use FRED pre-computed spreads if available (more accurate)
	if data.Spread10Y2Y != 0 {
		spread2Y10Y = data.Spread10Y2Y
	}
	if data.Spread10Y3M != 0 {
		spread3M10Y = data.Spread10Y3M
	}

	bothInverted := spread2Y10Y < 0 && spread3M10Y < 0
	eitherInverted := spread2Y10Y < 0 || spread3M10Y < 0

	// Detect steepening: spread was recently negative (trend direction DOWN→UP transition)
	// Use the trend direction as a proxy — if spread is now positive but small,
	// and the trend was recently coming from inversion, classify as STEEPENING.
	recentlyInverted := data.YieldSpreadTrend.Previous < 0 && spread2Y10Y > 0

	switch {
	case bothInverted && (spread2Y10Y < -0.5 && spread3M10Y < -0.5):
		return "DEEP_INVERSION"
	case eitherInverted:
		return "INVERTED"
	case recentlyInverted && spread2Y10Y < 0.50:
		// Curve just un-inverted — this transition historically precedes recession by 3-6 months
		return "STEEPENING"
	case spread2Y10Y >= 0 && spread2Y10Y < 0.25:
		return "FLAT"
	case spread2Y10Y > 1.5 || spread3M10Y > 1.5:
		return "STEEP"
	default:
		return "NORMAL"
	}
}

// computeCreditStress returns 0-100 credit stress level.
func computeCreditStress(data *MacroData) float64 {
	var total, weights float64

	// HY OAS spread (30%) — stored in TedSpread field
	if data.TedSpread > 0 {
		hyScore := mapRange(data.TedSpread, 2.5, 8.0, 0, 100)
		total += hyScore * 30
		weights += 30
	}

	// BBB spread (15%)
	if data.BBBSpread > 0 {
		bbbScore := mapRange(data.BBBSpread, 1.0, 4.0, 0, 100)
		total += bbbScore * 15
		weights += 15
	}

	// AAA spread (10%)
	if data.AAASpread > 0 {
		aaaScore := mapRange(data.AAASpread, 0.3, 2.0, 0, 100)
		total += aaaScore * 10
		weights += 10
	}

	// NFCI (20%) — 0 is a valid value (neutral conditions), only skip if we have no FRED data.
	// NFCI=0 maps to ~40/100 (normal credit), which is a safe neutral default even if unfetched.
	{
		nfciScore := mapRange(data.NFCI, -0.5, 0.7, 0, 100)
		total += nfciScore * 20
		weights += 20
	}

	// St. Louis Stress Index (15%) — 0 is also a valid neutral value.
	// St. Louis Stress Index (15%) — 0 is also a valid neutral value.
	{
		stlScore := mapRange(data.StLouisStress, -1.0, 2.0, 0, 100)
		total += stlScore * 15
		weights += 15
	}

	// SOFR-IORB spread (5%)
	if data.SOFR > 0 && data.IORB > 0 {
		sofrSpread := data.SOFR - data.IORB
		sofrScore := mapRange(sofrSpread, -0.05, 0.2, 0, 100)
		total += sofrScore * 5
		weights += 5
	}

	// Senior Loan Officer Survey (5% — credit cycle leading indicator)
	// Positive values = net tightening of lending standards
	if data.SeniorLoanSurvey != 0 {
		loanScore := mapRange(data.SeniorLoanSurvey, -10, 50, 0, 100)
		total += loanScore * 5
		weights += 5
	}

	if weights == 0 {
		return 30 // no data, assume normal
	}
	return clamp(total/weights, 0, 100)
}

// computeHousingPulse classifies the housing market state.
func computeHousingPulse(data *MacroData) string {
	signals := 0 // positive = expanding, negative = contracting
	dataPoints := 0

	if data.BuildingPermits > 0 {
		dataPoints++
		if data.BuildingPermitsTrend.Direction == "UP" {
			signals++
		} else if data.BuildingPermitsTrend.Direction == "DOWN" {
			signals--
		}
	}

	if data.HousingStarts > 0 {
		dataPoints++
		if data.HousingStartsTrend.Direction == "UP" {
			signals++
		} else if data.HousingStartsTrend.Direction == "DOWN" {
			signals--
		}
	}

	if data.MortgageRate30Y > 0 {
		dataPoints++
		// Rising rates = negative for housing
		if data.MortgageRate30Y > 7.0 {
			signals--
		} else if data.MortgageRate30Y < 5.0 {
			signals++
		}
	}

	if dataPoints == 0 {
		return "N/A"
	}

	switch {
	case signals >= 2:
		return "EXPANDING"
	case signals <= -2:
		if data.BuildingPermitsTrend.Direction == "DOWN" && data.HousingStartsTrend.Direction == "DOWN" {
			return "COLLAPSING"
		}
		return "CONTRACTING"
	case signals <= -1:
		return "CONTRACTING"
	default:
		return "STABLE"
	}
}

// computeFinancialConditions returns -1.0 to +1.0 (positive = loose).
func computeFinancialConditions(data *MacroData) float64 {
	var total, weights float64

	// NFCI (primary, 40%) — negative = loose, 0 = neutral, positive = tight
	// NFCI=0 is valid (neutral conditions); always include.
	{
		nfciNorm := mapRange(data.NFCI, -0.8, 1.0, 1.0, -1.0)
		total += nfciNorm * 40
		weights += 40
	}

	// VIX (20%)
	if data.VIX > 0 {
		vixNorm := mapRange(data.VIX, 12, 35, 1.0, -1.0)
		total += vixNorm * 20
		weights += 20
	}

	// HY Spread (20%)
	if data.TedSpread > 0 {
		hyNorm := mapRange(data.TedSpread, 2.5, 6.0, 1.0, -1.0)
		total += hyNorm * 20
		weights += 20
	}

	// Real yield restrictiveness (20%)
	if data.RealYield10Y != 0 {
		// High real yield = tight, low/negative = loose
		ryNorm := mapRange(data.RealYield10Y, -1.0, 3.0, 1.0, -1.0)
		total += ryNorm * 20
		weights += 20
	}

	if weights == 0 {
		return 0
	}
	return clamp(total/weights, -1.0, 1.0)
}

// computeUSMacroScore returns -100 to +100 for US macro health.
func computeUSMacroScore(data *MacroData) float64 {
	var total, weights float64

	// GDP growth (25%)
	if data.GDPGrowth != 0 {
		gdpScore := mapRange(data.GDPGrowth, -2.0, 4.0, -100, 100)
		total += gdpScore * 25
		weights += 25
	}

	// Inflation vs target (30%) — closer to 2% = better
	if data.CorePCE > 0 {
		// Ideal = 2.0%. Score drops for both too high and too low.
		deviation := math.Abs(data.CorePCE - 2.0)
		infScore := mapRange(deviation, 0, 3.0, 100, -100)
		total += infScore * 30
		weights += 30
	}

	// Labor (20%)
	if data.UnemployRate > 0 {
		laborScore := mapRange(data.UnemployRate, 6.0, 3.0, -100, 100)
		total += laborScore * 20
		weights += 20
	}

	// Rate level (25%) — higher = more attractive for currency
	if data.FedFundsRate > 0 {
		rateScore := mapRange(data.FedFundsRate, 0, 6.0, -100, 100)
		total += rateScore * 25
		weights += 25
	}

	if weights == 0 {
		return 0
	}
	return clamp(total/weights, -100, 100)
}

// computeCountryScore returns -100 to +100 for a country's macro health.
// Parameters: CPI (YoY%), GDP (QoQ%), unemployment (%), rate (%),
// and target values for normalization.
func computeCountryScore(cpi, gdp, unemployment, rate, cpiTarget, gdpGood, unempNeutral float64) float64 {
	var total, weights float64

	// CPI vs target (30%) — cpi != 0 allows deflation as valid data
	if cpi != 0 {
		deviation := math.Abs(cpi - cpiTarget)
		infScore := mapRange(deviation, 0, 3.0, 100, -100)
		total += infScore * 30
		weights += 30
	}

	// GDP (25%) — gdp == 0 treated as missing data (exact 0% stagnation is rare)
	if gdp != 0 {
		gdpScore := mapRange(gdp, -1.0, gdpGood*3, -100, 100)
		total += gdpScore * 25
		weights += 25
	}

	// Unemployment (20%)
	if unemployment > 0 {
		unempScore := mapRange(unemployment, unempNeutral+3, unempNeutral-2, -100, 100)
		total += unempScore * 20
		weights += 20
	}

	// Rate level (25%) — rate can be 0 (ZIRP) or negative (NIRP), both valid
	if rate != 0 {
		rateScore := mapRange(rate, 0, 6.0, -100, 100)
		total += rateScore * 25
		weights += 25
	}

	if weights == 0 {
		return 0
	}
	return clamp(total/weights, -100, 100)
}

// computeSentimentComposite returns -100 (extreme greed) to +100 (extreme fear).
// Contrarian-adjusted: high fear = positive (bullish signal).
func computeSentimentComposite(data *MacroData) float64 {
	var total, weights float64

	// CNN Fear & Greed (25%) — contrarian: invert
	if data.CNNFearGreed > 0 {
		// CNN 0=extreme fear, 100=extreme greed
		// Invert: 0->+100, 100->-100
		cnnScore := mapRange(data.CNNFearGreed, 0, 100, 100, -100)
		total += cnnScore * 25
		weights += 25
	}

	// AAII Bull/Bear (25%) — contrarian: invert
	if data.AAIIBullBear > 0 {
		// Ratio >1.5 = very bullish crowd (bearish contrarian), <0.5 = very bearish crowd (bullish contrarian)
		aaiScore := mapRange(data.AAIIBullBear, 0.3, 2.0, 100, -100)
		total += aaiScore * 25
		weights += 25
	}

	// CBOE Put/Call (20%) — high P/C = fear = contrarian bullish
	if data.PutCallTotal > 0 {
		pcScore := mapRange(data.PutCallTotal, 0.6, 1.3, -100, 100)
		total += pcScore * 20
		weights += 20
	}

	// VIX percentile (15%) — high VIX = contrarian bullish
	if data.VIX > 0 {
		vixScore := mapRange(data.VIX, 12, 35, -100, 100)
		total += vixScore * 15
		weights += 15
	}

	// Michigan Consumer Sentiment (15%) — contrarian: low sentiment = bullish
	if data.ConsumerSentiment > 0 {
		csScore := mapRange(data.ConsumerSentiment, 50, 100, 100, -100)
		total += csScore * 15
		weights += 15
	}

	if weights == 0 {
		return 0
	}
	return clamp(total/weights, -100, 100)
}

// --- Utility functions ---

// mapRange linearly maps value from [inMin, inMax] to [outMin, outMax], clamped.
func mapRange(value, inMin, inMax, outMin, outMax float64) float64 {
	if inMax == inMin {
		return (outMin + outMax) / 2
	}
	// Clamp input
	if inMin < inMax {
		if value < inMin {
			value = inMin
		}
		if value > inMax {
			value = inMax
		}
	} else {
		// inverted range (inMin > inMax)
		if value > inMin {
			value = inMin
		}
		if value < inMax {
			value = inMax
		}
	}
	ratio := (value - inMin) / (inMax - inMin)
	return outMin + ratio*(outMax-outMin)
}

// clamp restricts v to [min, max].
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
