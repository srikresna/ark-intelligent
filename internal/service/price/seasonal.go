package price

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Advanced Seasonal Pattern Analysis
// Combines statistical rigor, regime awareness, COT alignment,
// event density, volatility regime, and cross-asset confluence.
// ---------------------------------------------------------------------------

// ConfidenceTier classifies the reliability of a seasonal signal.
type ConfidenceTier string

const (
	ConfidenceStrong   ConfidenceTier = "STRONG"
	ConfidenceModerate ConfidenceTier = "MODERATE"
	ConfidenceWeak     ConfidenceTier = "WEAK"
	ConfidenceNone     ConfidenceTier = "NONE"
)

// SeasonalPattern holds monthly return statistics for a contract.
type SeasonalPattern struct {
	ContractCode string
	Currency     string
	Monthly      [12]MonthStats // Index 0 = January
	CurrentMonth int            // 1-12
	CurrentBias  string         // "BULLISH", "BEARISH", "NEUTRAL"

	// Advanced: regime-filtered stats for the current month
	RegimeStats    *RegimeMonthStats    // nil if regime data unavailable
	COTAlignment   *COTAlignmentResult  // nil if COT data unavailable
	EventDensity   *EventDensityResult  // nil if event data unavailable
	VolContext     *SeasonalVolContext   // nil if insufficient data
	CrossAsset     *CrossAssetResult    // nil if cross-asset data unavailable
	EIACtx         *EIAContext          // nil if not an energy asset or EIA unavailable
	Confluence     *ConfluenceResult    // nil if not enough data for scoring
}

// MonthStats holds aggregated return statistics for a single calendar month.
type MonthStats struct {
	Month      string  // "Jan", "Feb", etc.
	AvgReturn  float64 // Average monthly return %
	MedianRet  float64 // Median monthly return %
	StdDev     float64 // Standard deviation of returns
	WinRate    float64 // % of months positive
	SampleSize int     // Number of data points
	Bias       string  // "BULLISH", "BEARISH", "NEUTRAL"
	Confidence ConfidenceTier

	// Recency-weighted statistics
	WeightedAvg float64 // Exponential-decay weighted average return
	WeightedWR  float64 // Exponential-decay weighted win rate

	// Per-year returns for streak analysis
	YearReturns []YearReturn // ordered newest-first
}

// YearReturn holds a single year's return for a calendar month.
type YearReturn struct {
	Year   int
	Return float64
}

// RegimeMonthStats holds seasonal stats filtered to the current macro regime.
type RegimeMonthStats struct {
	RegimeName  string  // e.g., "DISINFLATIONARY"
	AvgReturn   float64
	WinRate     float64
	SampleSize  int
	Bias        string
	Confidence  ConfidenceTier
	PrimaryFREDDriver string // e.g., "Real Yield (10Y-BE5Y): -0.3%"
	DriverAlignment   string // "SUPPORTIVE", "HEADWIND", "NEUTRAL"
}

// COTAlignmentResult shows whether COT positioning historically aligns.
type COTAlignmentResult struct {
	AlignedYears   int    // years where COT direction matched seasonal direction
	TotalYears     int    // total years with both seasonal and COT data
	AlignmentPct   float64 // AlignedYears / TotalYears * 100
	CurrentAligned bool   // is current COT positioning aligned with seasonal bias?
	CurrentCOTBias string // "BULLISH", "BEARISH", "NEUTRAL"
	Interpretation string // e.g., "commercial = contrarian for FX"
}

// EventDensityResult indicates how event-heavy the month typically is.
type EventDensityResult struct {
	HighImpactEvents int    // avg high-impact events in this month for this currency
	Rating           string // "HIGH", "MEDIUM", "LOW"
	KeyEvents        string // e.g., "FOMC, NFP, ECB"
	ReliabilityAdj   float64 // -0.2 to 0 adjustment to confidence (high events = lower reliability)
}

// SeasonalVolContext provides historical volatility profile for the month.
// Named differently from VolatilityContext in volatility.go to avoid collision.
type SeasonalVolContext struct {
	HistoricalATR    float64 // average ATR for this month (%)
	AvgATR           float64 // overall average ATR across all months
	VolRatio         float64 // HistoricalATR / AvgATR (>1 = above-average vol)
	VIXSensitivity   string  // "HIGH", "MEDIUM", "LOW"
	CurrentVIXRegime string  // "ELEVATED", "NORMAL", "LOW"
	Assessment       string  // e.g., "Below-avg volatility → pattern more reliable"
}

// CrossAssetResult checks for contradictions with correlated assets.
type CrossAssetResult struct {
	Correlations   []CrossCorrelation
	Contradictions int
	Assessment     string // "CONSISTENT", "MIXED", "CONTRADICTORY"
}

// CrossCorrelation describes a seasonal correlation check.
type CrossCorrelation struct {
	Asset      string // e.g., "USD", "OIL"
	Relation   string // e.g., "inverse", "positive"
	TheirBias  string // "BULLISH", "BEARISH", "NEUTRAL"
	IsAligned  bool   // does their bias align with expected relation?
}

// ConfluenceResult is the final multi-factor seasonal score.
type ConfluenceResult struct {
	Score       int    // 0-5 factors aligned
	MaxScore    int    // max possible (usually 5)
	Level       string // "HIGH", "MODERATE-HIGH", "MODERATE", "LOW"
	Factors     []ConfluenceFactor
	Verdict     string // e.g., "MODERATE CONFIDENCE BULLISH"
}

// ConfluenceFactor describes a single confluence factor.
type ConfluenceFactor struct {
	Name    string // "Seasonal", "Regime", "COT", "Events", "Cross-Asset"
	Aligned bool
	Detail  string // short explanation
}

// EIAContext holds energy-specific seasonal context from EIA data.
type EIAContext struct {
	InventoryTrend     string  // "BUILD", "DRAW", "FLAT"
	AvgWeeklyChange    float64 // avg weekly inventory change in this month (millions bbl)
	CurrentVs5YrAvg    string  // "ABOVE", "BELOW", "NEAR" 5-year seasonal average
	RefineryUtil       float64 // avg refinery utilization % in this month
	Assessment         string  // e.g., "Spring build season confirms bearish seasonal"
}

var monthNames = [12]string{
	"Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
}

// ---------------------------------------------------------------------------
// Asset-Specific Driver Configuration
// ---------------------------------------------------------------------------

// AssetDriver defines the primary macro driver and characteristics for an asset.
type AssetDriver struct {
	PrimaryFREDMetric string // which FRED metric matters most
	VIXSensitivity    string // "HIGH", "MEDIUM", "LOW"
	COTInterpretation string // how to read commercial positioning
	CrossAssets       []CrossAssetDef
	AssetClass        string // "FX", "METAL", "ENERGY", "BOND", "EQUITY", "CRYPTO"
}

// CrossAssetDef defines a cross-asset relationship for correlation checks.
type CrossAssetDef struct {
	Currency string // e.g., "USD"
	Relation string // "inverse" or "positive"
}

// assetDrivers maps currency codes to their fundamental drivers.
var assetDrivers = map[string]AssetDriver{
	// FX Majors
	"EUR": {PrimaryFREDMetric: "rate_diff_ECB", VIXSensitivity: "LOW", COTInterpretation: "contrarian", AssetClass: "FX",
		CrossAssets: []CrossAssetDef{{Currency: "USD", Relation: "inverse"}, {Currency: "GBP", Relation: "positive"}}},
	"GBP": {PrimaryFREDMetric: "rate_diff_BOE", VIXSensitivity: "LOW", COTInterpretation: "contrarian", AssetClass: "FX",
		CrossAssets: []CrossAssetDef{{Currency: "USD", Relation: "inverse"}, {Currency: "EUR", Relation: "positive"}}},
	"JPY": {PrimaryFREDMetric: "yield_spread+NFCI", VIXSensitivity: "HIGH", COTInterpretation: "contrarian", AssetClass: "FX",
		CrossAssets: []CrossAssetDef{{Currency: "USD", Relation: "inverse"}, {Currency: "XAU", Relation: "positive"}}},
	"CHF": {PrimaryFREDMetric: "NFCI", VIXSensitivity: "HIGH", COTInterpretation: "contrarian", AssetClass: "FX",
		CrossAssets: []CrossAssetDef{{Currency: "USD", Relation: "inverse"}, {Currency: "JPY", Relation: "positive"}}},
	"AUD": {PrimaryFREDMetric: "GDP+labor", VIXSensitivity: "MEDIUM", COTInterpretation: "contrarian", AssetClass: "FX",
		CrossAssets: []CrossAssetDef{{Currency: "USD", Relation: "inverse"}, {Currency: "COPPER", Relation: "positive"}, {Currency: "NZD", Relation: "positive"}}},
	"CAD": {PrimaryFREDMetric: "GDP+OIL", VIXSensitivity: "MEDIUM", COTInterpretation: "contrarian", AssetClass: "FX",
		CrossAssets: []CrossAssetDef{{Currency: "USD", Relation: "inverse"}, {Currency: "OIL", Relation: "positive"}}},
	"NZD": {PrimaryFREDMetric: "GDP+labor", VIXSensitivity: "MEDIUM", COTInterpretation: "contrarian", AssetClass: "FX",
		CrossAssets: []CrossAssetDef{{Currency: "USD", Relation: "inverse"}, {Currency: "AUD", Relation: "positive"}}},
	"USD": {PrimaryFREDMetric: "DXY+FEDFUNDS", VIXSensitivity: "MEDIUM", COTInterpretation: "contrarian", AssetClass: "FX",
		CrossAssets: []CrossAssetDef{{Currency: "EUR", Relation: "inverse"}, {Currency: "XAU", Relation: "inverse"}}},

	// Metals
	"XAU": {PrimaryFREDMetric: "real_yield", VIXSensitivity: "HIGH", COTInterpretation: "contrarian", AssetClass: "METAL",
		CrossAssets: []CrossAssetDef{{Currency: "USD", Relation: "inverse"}, {Currency: "BOND", Relation: "positive"}}},
	"XAG": {PrimaryFREDMetric: "GDP+real_yield", VIXSensitivity: "MEDIUM", COTInterpretation: "contrarian", AssetClass: "METAL",
		CrossAssets: []CrossAssetDef{{Currency: "USD", Relation: "inverse"}, {Currency: "XAU", Relation: "positive"}, {Currency: "COPPER", Relation: "positive"}}},
	"COPPER": {PrimaryFREDMetric: "GDP+ISM", VIXSensitivity: "MEDIUM", COTInterpretation: "contrarian", AssetClass: "METAL",
		CrossAssets: []CrossAssetDef{{Currency: "AUD", Relation: "positive"}, {Currency: "USD", Relation: "inverse"}}},

	// Energy
	"OIL": {PrimaryFREDMetric: "GDP+EIA", VIXSensitivity: "MEDIUM", COTInterpretation: "directional", AssetClass: "ENERGY",
		CrossAssets: []CrossAssetDef{{Currency: "CAD", Relation: "positive"}, {Currency: "USD", Relation: "inverse"}}},
	"ULSD": {PrimaryFREDMetric: "GDP+EIA", VIXSensitivity: "LOW", COTInterpretation: "directional", AssetClass: "ENERGY",
		CrossAssets: []CrossAssetDef{{Currency: "OIL", Relation: "positive"}}},
	"RBOB": {PrimaryFREDMetric: "GDP+EIA", VIXSensitivity: "LOW", COTInterpretation: "directional", AssetClass: "ENERGY",
		CrossAssets: []CrossAssetDef{{Currency: "OIL", Relation: "positive"}}},

	// Bonds
	"BOND": {PrimaryFREDMetric: "FEDFUNDS+inflation", VIXSensitivity: "MEDIUM", COTInterpretation: "directional", AssetClass: "BOND",
		CrossAssets: []CrossAssetDef{{Currency: "XAU", Relation: "positive"}, {Currency: "SPX500", Relation: "inverse"}}},
	"BOND30": {PrimaryFREDMetric: "FEDFUNDS+inflation", VIXSensitivity: "MEDIUM", COTInterpretation: "directional", AssetClass: "BOND",
		CrossAssets: []CrossAssetDef{{Currency: "BOND", Relation: "positive"}}},
	"BOND5": {PrimaryFREDMetric: "FEDFUNDS+inflation", VIXSensitivity: "MEDIUM", COTInterpretation: "directional", AssetClass: "BOND",
		CrossAssets: []CrossAssetDef{{Currency: "BOND", Relation: "positive"}}},
	"BOND2": {PrimaryFREDMetric: "FEDFUNDS+inflation", VIXSensitivity: "LOW", COTInterpretation: "directional", AssetClass: "BOND",
		CrossAssets: []CrossAssetDef{{Currency: "BOND", Relation: "positive"}}},

	// Equity Indices
	"SPX500": {PrimaryFREDMetric: "NFCI+GDP", VIXSensitivity: "HIGH", COTInterpretation: "momentum", AssetClass: "EQUITY",
		CrossAssets: []CrossAssetDef{{Currency: "NDX", Relation: "positive"}, {Currency: "BOND", Relation: "inverse"}}},
	"NDX": {PrimaryFREDMetric: "NFCI+GDP", VIXSensitivity: "HIGH", COTInterpretation: "momentum", AssetClass: "EQUITY",
		CrossAssets: []CrossAssetDef{{Currency: "SPX500", Relation: "positive"}}},
	"DJI": {PrimaryFREDMetric: "NFCI+GDP", VIXSensitivity: "HIGH", COTInterpretation: "momentum", AssetClass: "EQUITY",
		CrossAssets: []CrossAssetDef{{Currency: "SPX500", Relation: "positive"}}},
	"RUT": {PrimaryFREDMetric: "NFCI+GDP", VIXSensitivity: "HIGH", COTInterpretation: "momentum", AssetClass: "EQUITY",
		CrossAssets: []CrossAssetDef{{Currency: "SPX500", Relation: "positive"}}},

	// Crypto
	"BTC": {PrimaryFREDMetric: "NFCI", VIXSensitivity: "MEDIUM", COTInterpretation: "momentum", AssetClass: "CRYPTO",
		CrossAssets: []CrossAssetDef{{Currency: "ETH", Relation: "positive"}, {Currency: "SPX500", Relation: "positive"}}},
	"ETH": {PrimaryFREDMetric: "NFCI", VIXSensitivity: "MEDIUM", COTInterpretation: "momentum", AssetClass: "CRYPTO",
		CrossAssets: []CrossAssetDef{{Currency: "BTC", Relation: "positive"}}},
}

// GetAssetDriver returns the driver config for a currency, with a safe default.
func GetAssetDriver(currency string) AssetDriver {
	if d, ok := assetDrivers[currency]; ok {
		return d
	}
	return AssetDriver{
		PrimaryFREDMetric: "unknown",
		VIXSensitivity:    "MEDIUM",
		COTInterpretation: "contrarian",
		AssetClass:        "OTHER",
	}
}

// ---------------------------------------------------------------------------
// Seasonal Analyzer
// ---------------------------------------------------------------------------

// SeasonalAnalyzer computes historical seasonal tendencies from stored price data.
type SeasonalAnalyzer struct {
	priceRepo ports.PriceRepository
}

// NewSeasonalAnalyzer creates a new SeasonalAnalyzer.
func NewSeasonalAnalyzer(priceRepo ports.PriceRepository) *SeasonalAnalyzer {
	return &SeasonalAnalyzer{priceRepo: priceRepo}
}

// Analyze computes seasonal patterns for all COT contracts.
func (sa *SeasonalAnalyzer) Analyze(ctx context.Context) ([]SeasonalPattern, error) {
	mappings := domain.COTPriceSymbolMappings()
	patterns := make([]SeasonalPattern, 0, len(mappings))

	for _, m := range mappings {
		p, err := sa.AnalyzeContract(ctx, m.ContractCode, m.Currency)
		if err != nil {
			continue
		}
		patterns = append(patterns, *p)
	}

	if len(patterns) == 0 {
		return nil, fmt.Errorf("no seasonal data available for any contract")
	}
	return patterns, nil
}

// AnalyzeContract computes the enhanced seasonal pattern for a single contract.
func (sa *SeasonalAnalyzer) AnalyzeContract(ctx context.Context, contractCode, currency string) (*SeasonalPattern, error) {
	const maxWeeks = 260 // 5 years

	records, err := sa.priceRepo.GetHistory(ctx, contractCode, maxWeeks)
	if err != nil {
		return nil, fmt.Errorf("get price history for %s: %w", currency, err)
	}
	if len(records) < 4 {
		return nil, fmt.Errorf("insufficient price data for %s (%d records)", currency, len(records))
	}

	// Records come newest-first; reverse to oldest-first.
	sort.Slice(records, func(i, j int) bool {
		return records[i].Date.Before(records[j].Date)
	})

	// Group weekly closes by year-month.
	type monthBound struct {
		firstClose float64
		lastClose  float64
		firstDate  time.Time
		lastDate   time.Time
	}
	months := make(map[string]*monthBound)
	for _, r := range records {
		if r.Close == 0 {
			continue
		}
		key := r.Date.Format("2006-01")
		mb, ok := months[key]
		if !ok {
			mb = &monthBound{
				firstClose: r.Close, lastClose: r.Close,
				firstDate: r.Date, lastDate: r.Date,
			}
			months[key] = mb
		} else {
			if r.Date.Before(mb.firstDate) {
				mb.firstClose = r.Close
				mb.firstDate = r.Date
			}
			if r.Date.After(mb.lastDate) {
				mb.lastClose = r.Close
				mb.lastDate = r.Date
			}
		}
	}

	// Compute monthly returns, EXCLUDING incomplete current month.
	now := time.Now()
	currentKey := now.Format("2006-01")

	type monthReturn struct {
		year   int
		month  time.Month
		retPct float64
	}

	var returns []monthReturn
	for key, mb := range months {
		if mb.firstClose == 0 {
			continue
		}
		// Phase 1 fix: skip the current incomplete month
		if key == currentKey {
			continue
		}
		ret := (mb.lastClose - mb.firstClose) / mb.firstClose * 100.0
		returns = append(returns, monthReturn{
			year:   mb.firstDate.Year(),
			month:  mb.firstDate.Month(),
			retPct: ret,
		})
	}

	// Aggregate per calendar month with full statistics.
	var (
		perMonth [12][]float64   // raw returns per month
		perYear  [12][]YearReturn // year-tagged returns
	)

	for _, r := range returns {
		idx := int(r.month) - 1
		perMonth[idx] = append(perMonth[idx], r.retPct)
		perYear[idx] = append(perYear[idx], YearReturn{Year: r.year, Return: r.retPct})
	}

	pattern := &SeasonalPattern{
		ContractCode: contractCode,
		Currency:     currency,
		CurrentMonth: int(now.Month()),
	}

	maxYear := now.Year()
	for i := 0; i < 12; i++ {
		ms := computeMonthStats(i, perMonth[i], perYear[i], maxYear)
		pattern.Monthly[i] = ms
	}

	curIdx := int(now.Month()) - 1
	pattern.CurrentBias = pattern.Monthly[curIdx].Bias

	return pattern, nil
}

// computeMonthStats calculates all statistics for a calendar month.
func computeMonthStats(monthIdx int, rets []float64, yearRets []YearReturn, maxYear int) MonthStats {
	ms := MonthStats{
		Month:      monthNames[monthIdx],
		SampleSize: len(rets),
	}

	if len(rets) == 0 {
		ms.Bias = "NEUTRAL"
		ms.Confidence = ConfidenceNone
		return ms
	}

	// Sort year returns newest-first
	sort.Slice(yearRets, func(i, j int) bool {
		return yearRets[i].Year > yearRets[j].Year
	})
	ms.YearReturns = yearRets

	// Basic statistics
	sum := 0.0
	wins := 0
	for _, r := range rets {
		sum += r
		if r > 0 {
			wins++
		}
	}
	ms.AvgReturn = sum / float64(len(rets))
	ms.WinRate = float64(wins) / float64(len(rets)) * 100.0

	// Median
	sorted := make([]float64, len(rets))
	copy(sorted, rets)
	sort.Float64s(sorted)
	if len(sorted)%2 == 0 {
		ms.MedianRet = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	} else {
		ms.MedianRet = sorted[len(sorted)/2]
	}

	// Standard deviation
	if len(rets) >= 2 {
		sumSq := 0.0
		for _, r := range rets {
			diff := r - ms.AvgReturn
			sumSq += diff * diff
		}
		ms.StdDev = math.Sqrt(sumSq / float64(len(rets)-1))
	}

	// Recency-weighted stats (exponential decay: weight = 0.8^(yearsAgo))
	ms.WeightedAvg, ms.WeightedWR = computeRecencyWeighted(yearRets, maxYear)

	// Classification with minimum sample size
	ms.Bias = classifyBias(ms.AvgReturn, ms.WinRate, ms.SampleSize)
	ms.Confidence = classifyConfidence(ms.AvgReturn, ms.WinRate, ms.StdDev, ms.SampleSize)

	return ms
}

// computeRecencyWeighted calculates exponential-decay weighted average and win rate.
func computeRecencyWeighted(yearRets []YearReturn, maxYear int) (weightedAvg, weightedWR float64) {
	if len(yearRets) == 0 {
		return 0, 0
	}

	const decayFactor = 0.8
	totalWeight := 0.0
	weightedSum := 0.0
	weightedWins := 0.0

	for _, yr := range yearRets {
		yearsAgo := maxYear - yr.Year
		if yearsAgo < 0 {
			yearsAgo = 0
		}
		weight := math.Pow(decayFactor, float64(yearsAgo))
		totalWeight += weight
		weightedSum += yr.Return * weight
		if yr.Return > 0 {
			weightedWins += weight
		}
	}

	if totalWeight > 0 {
		weightedAvg = weightedSum / totalWeight
		weightedWR = weightedWins / totalWeight * 100.0
	}
	return
}

// classifyBias determines seasonal bias with minimum sample size of 3.
// Requires both direction agreement (avg + WR) and a minimum effect size.
func classifyBias(avgReturn, winRate float64, sampleSize int) string {
	if sampleSize < 3 {
		return "NEUTRAL"
	}
	// Minimum effect size: at least 0.1% average return to avoid noise-driven classification
	if avgReturn > 0.1 && winRate > 55.0 {
		return "BULLISH"
	}
	if avgReturn < -0.1 && winRate < 45.0 {
		return "BEARISH"
	}
	return "NEUTRAL"
}

// classifyConfidence assigns a confidence tier based on statistical quality.
func classifyConfidence(avgReturn, winRate, stdDev float64, sampleSize int) ConfidenceTier {
	if sampleSize < 3 {
		return ConfidenceNone
	}

	absAvg := math.Abs(avgReturn)

	// STRONG: high win rate divergence + low noise + decent sample
	if sampleSize >= 4 && (winRate > 65 || winRate < 35) && (stdDev == 0 || absAvg/stdDev > 0.3) {
		return ConfidenceStrong
	}

	// MODERATE: meets bias threshold + reasonable sample
	if sampleSize >= 3 && (winRate > 55 || winRate < 45) {
		return ConfidenceModerate
	}

	// WEAK: has directional bias but noisy
	if (avgReturn > 0 && winRate > 50) || (avgReturn < 0 && winRate < 50) {
		return ConfidenceWeak
	}

	return ConfidenceNone
}
