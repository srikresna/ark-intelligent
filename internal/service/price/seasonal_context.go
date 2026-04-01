package price

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
)

// ---------------------------------------------------------------------------
// Advanced Seasonal Context Engine
// Enriches base seasonal patterns with regime, COT, event, volatility,
// cross-asset, and EIA context for institutional-grade analysis.
// ---------------------------------------------------------------------------

// SeasonalContextDeps holds all optional dependencies for advanced analysis.
type SeasonalContextDeps struct {
	PriceRepo  ports.PriceRepository
	COTRepo    ports.COTRepository
	NewsRepo   ports.NewsRepository
	MacroData  *fred.MacroData   // current FRED data snapshot
	Regimes    map[string]string // date -> regime name (historical)
	VIXPrice   float64           // current VIX level
	EIAData    *EIASeasonalData  // nil if EIA unavailable
}

// EnrichPattern adds advanced context (regime, COT, events, etc.) to a base pattern.
func EnrichPattern(ctx context.Context, pattern *SeasonalPattern, deps *SeasonalContextDeps) {
	if deps == nil {
		return
	}
	curMonth := pattern.CurrentMonth // 1-12
	if curMonth < 1 || curMonth > 12 {
		return // new contract with no data — nothing to enrich
	}
	curIdx := curMonth - 1
	curStats := pattern.Monthly[curIdx]
	driver := GetAssetDriver(pattern.Currency)

	// Phase 2: Regime-aware seasonal
	if deps.Regimes != nil && deps.MacroData != nil {
		pattern.RegimeStats = computeRegimeStats(pattern, curIdx, deps)
	}

	// Phase 3a: COT alignment
	if deps.COTRepo != nil {
		pattern.COTAlignment = computeCOTAlignment(ctx, pattern, curIdx, deps.COTRepo, driver)
	}

	// Phase 3b: Event density
	if deps.NewsRepo != nil {
		pattern.EventDensity = computeEventDensity(ctx, pattern.Currency, curMonth, deps.NewsRepo, driver)
	}

	// Phase 3c: Volatility context
	pattern.VolContext = computeVolatilityContext(pattern, curIdx, deps.VIXPrice, driver)

	// Phase 3d: Cross-asset checks
	if deps.PriceRepo != nil {
		pattern.CrossAsset = computeCrossAsset(ctx, pattern, curIdx, deps.PriceRepo, driver)
	}

	// Phase 4: EIA context for energy assets
	if deps.EIAData != nil {
		pattern.EIACtx = ComputeEIAContext(deps.EIAData, pattern.Currency, curMonth)
	}

	// Phase 5: Confluence scoring
	pattern.Confluence = computeConfluence(pattern, curStats)
}

// ---------------------------------------------------------------------------
// Phase 2: Regime-Aware Seasonal
// ---------------------------------------------------------------------------

func computeRegimeStats(pattern *SeasonalPattern, curIdx int, deps *SeasonalContextDeps) *RegimeMonthStats {
	// Determine current regime
	currentRegime := ""
	if deps.MacroData != nil {
		regime := fred.ClassifyMacroRegime(deps.MacroData)
		currentRegime = regime.Name
	}
	if currentRegime == "" {
		return nil
	}

	curStats := pattern.Monthly[curIdx]
	if len(curStats.YearReturns) == 0 {
		return nil
	}

	// Match each year's return with the regime that was active that month
	var regimeReturns []float64
	for _, yr := range curStats.YearReturns {
		// Look up regime around the middle of that month
		targetDate := time.Date(yr.Year, time.Month(curIdx+1), 15, 0, 0, 0, 0, time.UTC)
		regime := findRegimeAtDate(deps.Regimes, targetDate)
		if regime == currentRegime {
			regimeReturns = append(regimeReturns, yr.Return)
		}
	}

	result := &RegimeMonthStats{
		RegimeName: currentRegime,
		SampleSize: len(regimeReturns),
	}

	if len(regimeReturns) > 0 {
		sum := 0.0
		wins := 0
		for _, r := range regimeReturns {
			sum += r
			if r > 0 {
				wins++
			}
		}
		result.AvgReturn = sum / float64(len(regimeReturns))
		result.WinRate = float64(wins) / float64(len(regimeReturns)) * 100.0
		result.Bias = classifyBias(result.AvgReturn, result.WinRate, len(regimeReturns))

		// Compute stdDev for regime-filtered returns to avoid false STRONG confidence
		regimeStdDev := 0.0
		if len(regimeReturns) >= 2 {
			sumSq := 0.0
			for _, r := range regimeReturns {
				diff := r - result.AvgReturn
				sumSq += diff * diff
			}
			regimeStdDev = math.Sqrt(sumSq / float64(len(regimeReturns)-1))
		}
		result.Confidence = classifyConfidence(result.AvgReturn, result.WinRate, regimeStdDev, len(regimeReturns))
	} else {
		result.Bias = "NEUTRAL"
		result.Confidence = ConfidenceNone
	}

	// Compute primary FRED driver assessment
	result.PrimaryFREDDriver, result.DriverAlignment = assessFREDDriver(pattern.Currency, deps.MacroData)

	return result
}

// findRegimeAtDate finds the regime closest to a target date from the regime map.
func findRegimeAtDate(regimes map[string]string, target time.Time) string {
	if len(regimes) == 0 {
		return ""
	}

	bestRegime := ""
	bestDist := time.Duration(math.MaxInt64)

	for dateStr, regime := range regimes {
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		dist := target.Sub(d)
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			bestDist = dist
			bestRegime = regime
		}
	}

	// Only use if within 30 days
	if bestDist > 30*24*time.Hour {
		return ""
	}
	return bestRegime
}

// assessFREDDriver evaluates the primary FRED driver for a currency.
func assessFREDDriver(currency string, data *fred.MacroData) (driver, alignment string) {
	if data == nil {
		return "N/A", "NEUTRAL"
	}

	switch currency {
	case "XAU":
		realYield := data.Yield10Y - data.Breakeven5Y
		driver = fmt.Sprintf("Real Yield (10Y-BE5Y): %.2f%%", realYield)
		if realYield < 0 {
			alignment = "SUPPORTIVE" // negative real yield = gold bullish
		} else if realYield > 1.5 {
			alignment = "HEADWIND"
		} else {
			alignment = "NEUTRAL"
		}

	case "XAG", "COPPER":
		driver = fmt.Sprintf("GDP Growth: %.1f%%, ISM: %.1f", data.GDPGrowth, data.ISMNewOrders)
		if data.GDPGrowth > 2.0 && data.ISMNewOrders > 50 {
			alignment = "SUPPORTIVE"
		} else if data.GDPGrowth < 0 || data.ISMNewOrders < 45 {
			alignment = "HEADWIND"
		} else {
			alignment = "NEUTRAL"
		}

	case "JPY", "CHF":
		driver = fmt.Sprintf("NFCI: %.2f, VIX: %.1f", data.NFCI, data.VIX)
		if data.NFCI > 0.3 || data.VIX > 25 {
			alignment = "SUPPORTIVE" // stress = safe haven bullish
		} else if data.NFCI < -0.5 && data.VIX < 15 {
			alignment = "HEADWIND"
		} else {
			alignment = "NEUTRAL"
		}

	case "AUD", "NZD":
		driver = fmt.Sprintf("GDP: %.1f%%, Claims: %.0fK", data.GDPGrowth, data.InitialClaims/1000)
		if data.GDPGrowth > 2.0 && data.InitialClaims < 230000 {
			alignment = "SUPPORTIVE"
		} else if data.GDPGrowth < 1.0 || data.InitialClaims > 280000 {
			alignment = "HEADWIND"
		} else {
			alignment = "NEUTRAL"
		}

	case "CAD":
		driver = fmt.Sprintf("GDP: %.1f%% (+ OIL correlation)", data.GDPGrowth)
		if data.GDPGrowth > 2.0 {
			alignment = "SUPPORTIVE"
		} else if data.GDPGrowth < 1.0 {
			alignment = "HEADWIND"
		} else {
			alignment = "NEUTRAL"
		}

	case "EUR", "GBP":
		rateDiff := data.FedFundsRate // higher FFR = stronger USD = weaker EUR/GBP
		driver = fmt.Sprintf("Fed Funds: %.2f%%, DXY: %.1f", rateDiff, data.DXY)
		if data.DXY < 100 {
			alignment = "SUPPORTIVE"
		} else if data.DXY > 106 {
			alignment = "HEADWIND"
		} else {
			alignment = "NEUTRAL"
		}

	case "USD":
		driver = fmt.Sprintf("DXY: %.1f, FFR: %.2f%%", data.DXY, data.FedFundsRate)
		if data.DXY > 106 && data.FedFundsRate > 4.0 {
			alignment = "SUPPORTIVE"
		} else if data.DXY < 100 {
			alignment = "HEADWIND"
		} else {
			alignment = "NEUTRAL"
		}

	case "OIL", "RBOB", "ULSD":
		driver = fmt.Sprintf("GDP: %.1f%% (demand proxy)", data.GDPGrowth)
		if data.GDPGrowth > 2.5 {
			alignment = "SUPPORTIVE"
		} else if data.GDPGrowth < 1.0 {
			alignment = "HEADWIND"
		} else {
			alignment = "NEUTRAL"
		}

	case "BOND", "BOND30", "BOND5", "BOND2":
		driver = fmt.Sprintf("FFR: %.2f%%, PCE: %.1f%%", data.FedFundsRate, data.CorePCE)
		if data.CorePCE < 2.5 && data.FedFundsRate > data.CorePCE {
			alignment = "SUPPORTIVE" // easing expectations = bond bullish
		} else if data.CorePCE > 3.5 {
			alignment = "HEADWIND"
		} else {
			alignment = "NEUTRAL"
		}

	case "SPX500", "NDX", "DJI", "RUT":
		driver = fmt.Sprintf("NFCI: %.2f, GDP: %.1f%%", data.NFCI, data.GDPGrowth)
		if data.NFCI < -0.3 && data.GDPGrowth > 1.5 {
			alignment = "SUPPORTIVE"
		} else if data.NFCI > 0.3 || data.GDPGrowth < 0 {
			alignment = "HEADWIND"
		} else {
			alignment = "NEUTRAL"
		}

	case "BTC", "ETH":
		driver = fmt.Sprintf("NFCI: %.2f (risk proxy)", data.NFCI)
		if data.NFCI < -0.3 {
			alignment = "SUPPORTIVE" // loose financial conditions = risk-on
		} else if data.NFCI > 0.3 {
			alignment = "HEADWIND"
		} else {
			alignment = "NEUTRAL"
		}

	default:
		driver = "N/A"
		alignment = "NEUTRAL"
	}

	return driver, alignment
}

// ---------------------------------------------------------------------------
// Phase 3a: COT Alignment
// ---------------------------------------------------------------------------

func computeCOTAlignment(ctx context.Context, pattern *SeasonalPattern, curIdx int, cotRepo ports.COTRepository, driver AssetDriver) *COTAlignmentResult {
	curStats := pattern.Monthly[curIdx]
	if curStats.SampleSize == 0 || curStats.Bias == "NEUTRAL" {
		return nil
	}

	// Get current COT analysis
	analyses, err := cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil || len(analyses) == 0 {
		return nil
	}

	// Find COT for this currency
	var currentCOT *domain.COTAnalysis
	for i := range analyses {
		if analyses[i].Contract.Currency == pattern.Currency {
			currentCOT = &analyses[i]
			break
		}
	}
	if currentCOT == nil {
		return nil
	}

	result := &COTAlignmentResult{
		TotalYears: curStats.SampleSize,
	}

	// Determine current COT directional bias
	switch {
	case currentCOT.SentimentScore > 20:
		result.CurrentCOTBias = "BULLISH"
	case currentCOT.SentimentScore < -20:
		result.CurrentCOTBias = "BEARISH"
	default:
		result.CurrentCOTBias = "NEUTRAL"
	}

	// COT interpretation depends on asset class
	result.Interpretation = fmt.Sprintf("COT %s (%s)", driver.COTInterpretation, driver.AssetClass)

	// Check alignment: does current COT agree with seasonal bias?
	// For contrarian interpretation, use commercial signal when available (overrides speculative)
	if driver.COTInterpretation == "contrarian" && currentCOT.CommercialSignal != "" {
		// Contrarian: commercial positioning is the primary signal
		if curStats.Bias == "BULLISH" && strings.Contains(currentCOT.CommercialSignal, "BULLISH") {
			result.CurrentAligned = true
		} else if curStats.Bias == "BEARISH" && strings.Contains(currentCOT.CommercialSignal, "BEARISH") {
			result.CurrentAligned = true
		}
		// else: CurrentAligned stays false
	} else {
		// Directional/momentum: use speculative sentiment score direction
		if curStats.Bias == "BULLISH" && result.CurrentCOTBias == "BULLISH" {
			result.CurrentAligned = true
		} else if curStats.Bias == "BEARISH" && result.CurrentCOTBias == "BEARISH" {
			result.CurrentAligned = true
		}
	}

	// Historical alignment is not available — we don't store per-month COT snapshots.
	// Only current alignment is computed. Set to -1 to indicate N/A.
	result.AlignedYears = -1
	result.AlignmentPct = -1

	return result
}

// ---------------------------------------------------------------------------
// Phase 3b: Event Density
// ---------------------------------------------------------------------------

func computeEventDensity(ctx context.Context, currency string, curMonth int, newsRepo ports.NewsRepository, driver AssetDriver) *EventDensityResult {
	// Fetch events for the current month using GetByMonth
	now := time.Now()
	yearMonth := fmt.Sprintf("%d%02d", now.Year(), curMonth)
	events, err := newsRepo.GetByMonth(ctx, yearMonth)
	if err != nil {
		return nil
	}

	// Count high-impact events for this currency and for USD (affects all FX pairs)
	highCount := 0
	var keyEvents []string
	seenEvents := make(map[string]bool)

	for _, ev := range events {

		// Count events for this currency OR USD (for FX pairs)
		isRelevant := ev.Currency == currency
		if driver.AssetClass == "FX" && ev.Currency == "USD" {
			isRelevant = true
		}
		if !isRelevant {
			continue
		}

		if ev.Impact == "high" && !seenEvents[ev.Event] {
			highCount++
			seenEvents[ev.Event] = true
			if len(keyEvents) < 3 {
				keyEvents = append(keyEvents, ev.Event)
			}
		}
	}

	result := &EventDensityResult{
		HighImpactEvents: highCount,
	}

	switch {
	case highCount >= 4:
		result.Rating = "HIGH"
		result.ReliabilityAdj = -0.15
	case highCount >= 2:
		result.Rating = "MEDIUM"
		result.ReliabilityAdj = -0.05
	default:
		result.Rating = "LOW"
		result.ReliabilityAdj = 0
	}

	if len(keyEvents) > 0 {
		result.KeyEvents = strings.Join(keyEvents, ", ")
	}

	return result
}

// ---------------------------------------------------------------------------
// Phase 3c: Volatility Context
// ---------------------------------------------------------------------------

func computeVolatilityContext(pattern *SeasonalPattern, curIdx int, vixPrice float64, driver AssetDriver) *SeasonalVolContext {
	// Compute historical ATR proxy: use StdDev of returns per month as vol measure
	curStdDev := pattern.Monthly[curIdx].StdDev

	// Average StdDev across all months for comparison
	totalStdDev := 0.0
	validMonths := 0
	for i := 0; i < 12; i++ {
		if pattern.Monthly[i].SampleSize >= 2 {
			totalStdDev += pattern.Monthly[i].StdDev
			validMonths++
		}
	}

	result := &SeasonalVolContext{
		VIXSensitivity: driver.VIXSensitivity,
	}

	if validMonths > 0 {
		result.AvgATR = totalStdDev / float64(validMonths)
		result.HistoricalATR = curStdDev
		if result.AvgATR > 0 {
			result.VolRatio = curStdDev / result.AvgATR
		}
	}

	// Current VIX regime
	switch {
	case vixPrice > 25:
		result.CurrentVIXRegime = "ELEVATED"
	case vixPrice > 18:
		result.CurrentVIXRegime = "NORMAL"
	case vixPrice > 0:
		result.CurrentVIXRegime = "LOW"
	default:
		result.CurrentVIXRegime = "N/A"
	}

	// Assessment
	switch {
	case result.VolRatio > 1.3:
		result.Assessment = "Above-avg volatility → wider outcome range"
	case result.VolRatio < 0.7 && result.VolRatio > 0:
		result.Assessment = "Below-avg volatility → pattern more reliable"
	default:
		result.Assessment = "Normal volatility for this month"
	}

	return result
}

// ---------------------------------------------------------------------------
// Phase 3d: Cross-Asset Checks
// ---------------------------------------------------------------------------

func computeCrossAsset(ctx context.Context, pattern *SeasonalPattern, curIdx int, priceRepo ports.PriceRepository, driver AssetDriver) *CrossAssetResult {
	if len(driver.CrossAssets) == 0 {
		return nil
	}

	result := &CrossAssetResult{}

	for _, ca := range driver.CrossAssets {
		mapping := domain.FindPriceMappingByCurrency(ca.Currency)
		if mapping == nil {
			continue
		}

		// Compute seasonal bias for the cross-asset's current month
		analyzer := NewSeasonalAnalyzer(priceRepo)
		crossPattern, err := analyzer.AnalyzeContract(ctx, mapping.ContractCode, ca.Currency)
		if err != nil {
			continue
		}

		theirBias := crossPattern.Monthly[curIdx].Bias

		cc := CrossCorrelation{
			Asset:     ca.Currency,
			Relation:  ca.Relation,
			TheirBias: theirBias,
		}

		// Check alignment based on relationship type
		myBias := pattern.Monthly[curIdx].Bias
		if ca.Relation == "inverse" {
			// My bullish + their bearish = aligned
			cc.IsAligned = (myBias == "BULLISH" && theirBias == "BEARISH") ||
				(myBias == "BEARISH" && theirBias == "BULLISH") ||
				myBias == "NEUTRAL" || theirBias == "NEUTRAL"
		} else {
			// positive correlation: same direction = aligned
			cc.IsAligned = myBias == theirBias || myBias == "NEUTRAL" || theirBias == "NEUTRAL"
		}

		if !cc.IsAligned {
			result.Contradictions++
		}

		result.Correlations = append(result.Correlations, cc)
	}

	switch {
	case result.Contradictions == 0:
		result.Assessment = "CONSISTENT"
	case result.Contradictions == 1:
		result.Assessment = "MIXED"
	default:
		result.Assessment = "CONTRADICTORY"
	}

	return result
}

// ---------------------------------------------------------------------------
// Phase 5: Confluence Scoring
// ---------------------------------------------------------------------------

func computeConfluence(pattern *SeasonalPattern, curStats MonthStats) *ConfluenceResult {
	if curStats.SampleSize < 3 && curStats.Bias == "NEUTRAL" {
		return nil
	}

	result := &ConfluenceResult{
		MaxScore: 5,
	}

	// Factor 1: Base seasonal
	f1 := ConfluenceFactor{Name: "Seasonal"}
	if curStats.Bias != "NEUTRAL" && curStats.Confidence != ConfidenceNone {
		f1.Aligned = true
		f1.Detail = fmt.Sprintf("%s (%.1f%% avg, %.0f%% WR)", curStats.Bias, curStats.AvgReturn, curStats.WinRate)
		result.Score++
	} else {
		f1.Detail = "Neutral or insufficient data"
	}
	result.Factors = append(result.Factors, f1)

	// Factor 2: Regime
	f2 := ConfluenceFactor{Name: "Regime"}
	if pattern.RegimeStats != nil && pattern.RegimeStats.SampleSize > 0 {
		if pattern.RegimeStats.DriverAlignment == "SUPPORTIVE" {
			f2.Aligned = true
			f2.Detail = fmt.Sprintf("%s regime — %s", pattern.RegimeStats.RegimeName, pattern.RegimeStats.PrimaryFREDDriver)
			result.Score++
		} else if pattern.RegimeStats.DriverAlignment == "HEADWIND" {
			f2.Detail = fmt.Sprintf("HEADWIND — %s", pattern.RegimeStats.PrimaryFREDDriver)
		} else {
			f2.Detail = fmt.Sprintf("%s regime — neutral driver", pattern.RegimeStats.RegimeName)
		}
	} else {
		f2.Detail = "No regime data"
	}
	result.Factors = append(result.Factors, f2)

	// Factor 3: COT
	f3 := ConfluenceFactor{Name: "COT"}
	if pattern.COTAlignment != nil {
		if pattern.COTAlignment.CurrentAligned {
			f3.Aligned = true
			f3.Detail = fmt.Sprintf("COT %s — aligned with seasonal", pattern.COTAlignment.CurrentCOTBias)
			result.Score++
		} else {
			f3.Detail = fmt.Sprintf("COT %s — diverges from seasonal", pattern.COTAlignment.CurrentCOTBias)
		}
	} else {
		f3.Detail = "No COT data"
	}
	result.Factors = append(result.Factors, f3)

	// Factor 4: Events (aligned = low event density)
	f4 := ConfluenceFactor{Name: "Events"}
	if pattern.EventDensity != nil {
		if pattern.EventDensity.Rating == "LOW" {
			f4.Aligned = true
			f4.Detail = "Low event density → cleaner seasonals"
			result.Score++
		} else {
			f4.Detail = fmt.Sprintf("%s event density (%s)", pattern.EventDensity.Rating, pattern.EventDensity.KeyEvents)
		}
	} else {
		f4.Detail = "No event data"
		// Don't assume alignment when data is missing — consistent with other factors
	}
	result.Factors = append(result.Factors, f4)

	// Factor 5: Cross-asset
	f5 := ConfluenceFactor{Name: "Cross-Asset"}
	if pattern.CrossAsset != nil {
		if pattern.CrossAsset.Assessment == "CONSISTENT" {
			f5.Aligned = true
			f5.Detail = "No contradictions with correlated assets"
			result.Score++
		} else {
			f5.Detail = fmt.Sprintf("%d contradiction(s) detected", pattern.CrossAsset.Contradictions)
		}
	} else {
		f5.Detail = "No cross-asset data"
	}
	result.Factors = append(result.Factors, f5)

	// Determine overall level
	switch {
	case result.Score >= 5:
		result.Level = "HIGH"
	case result.Score >= 4:
		result.Level = "MODERATE-HIGH"
	case result.Score >= 3:
		result.Level = "MODERATE"
	case result.Score >= 2:
		result.Level = "LOW-MODERATE"
	default:
		result.Level = "LOW"
	}

	// Verdict
	biasWord := "NEUTRAL"
	if curStats.Bias != "NEUTRAL" {
		biasWord = curStats.Bias
	} else if pattern.RegimeStats != nil && pattern.RegimeStats.Bias != "NEUTRAL" {
		biasWord = pattern.RegimeStats.Bias
	}
	result.Verdict = fmt.Sprintf("%s CONFIDENCE %s", result.Level, biasWord)

	return result
}

// ---------------------------------------------------------------------------
// AnalyzeContractAdvanced — Full pipeline: base + context enrichment
// ---------------------------------------------------------------------------

// AnalyzeContractAdvanced runs the full seasonal analysis pipeline with all
// available context layers. Any nil dependency gracefully degrades to basic stats.
func (sa *SeasonalAnalyzer) AnalyzeContractAdvanced(ctx context.Context, contractCode, currency string, deps *SeasonalContextDeps) (*SeasonalPattern, error) {
	pattern, err := sa.AnalyzeContract(ctx, contractCode, currency)
	if err != nil {
		return nil, err
	}

	EnrichPattern(ctx, pattern, deps)
	return pattern, nil
}

// AnalyzeAllAdvanced runs the full pipeline for all contracts.
func (sa *SeasonalAnalyzer) AnalyzeAllAdvanced(ctx context.Context, deps *SeasonalContextDeps) ([]SeasonalPattern, error) {
	mappings := domain.COTPriceSymbolMappings()
	patterns := make([]SeasonalPattern, 0, len(mappings))

	// Cache cross-asset patterns to avoid redundant computation
	for _, m := range mappings {
		p, err := sa.AnalyzeContract(ctx, m.ContractCode, m.Currency)
		if err != nil {
			continue
		}
		// Enrich each pattern
		EnrichPattern(ctx, p, deps)
		patterns = append(patterns, *p)
	}

	if len(patterns) == 0 {
		return nil, fmt.Errorf("no seasonal data available for any contract")
	}

	// Sort: FX first, then metals, energy, bonds, equities, crypto
	classOrder := map[string]int{"FX": 0, "METAL": 1, "ENERGY": 2, "BOND": 3, "EQUITY": 4, "CRYPTO": 5, "OTHER": 6}
	sort.Slice(patterns, func(i, j int) bool {
		di := GetAssetDriver(patterns[i].Currency)
		dj := GetAssetDriver(patterns[j].Currency)
		return classOrder[di.AssetClass] < classOrder[dj.AssetClass]
	})

	return patterns, nil
}
