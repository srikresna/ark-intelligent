package cot

import (
	"context"
	"fmt"
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

var log = logger.Component("cot")

// Analyzer computes all 20+ COT metrics from raw positioning data.
// It processes historical records to derive net positions, ratios,
// momentum, sentiment, concentration, and generates trading signals.
type Analyzer struct {
	cotRepo    ports.COTRepository
	fetcher    *Fetcher
	lastRegime *fred.MacroRegime // Gap B: cached FRED regime for COT adjustments
}

// NewAnalyzer creates a COT analyzer.
func NewAnalyzer(repo ports.COTRepository, fetcher *Fetcher) *Analyzer {
	return &Analyzer{
		cotRepo: repo,
		fetcher: fetcher,
	}
}

// AnalyzeAll fetches latest data, computes metrics for all contracts,
// and stores the results. Returns computed analyses.
func (a *Analyzer) AnalyzeAll(ctx context.Context) ([]domain.COTAnalysis, error) {
	contracts := domain.DefaultCOTContracts // FIX: was domain.DefaultContracts() - it's a var, not func

	// Fetch latest records
	records, err := a.fetcher.FetchLatest(ctx, contracts)
	if err != nil {
		return nil, fmt.Errorf("fetch latest: %w", err)
	}

	// Enrich with options positions
	records, _ = a.fetcher.FetchOptionsPositions(ctx, contracts, records)

	// Save raw records
	if err := a.cotRepo.SaveRecords(ctx, records); err != nil {
		log.Warn().Str("op", "save-records").Err(err).Msg("failed to save records")
	}

	// Gap B — Best-effort FRED regime fetch for RegimeAdjustedScore population
	var cachedRegime *fred.MacroRegime
	if macroData, fredErr := fred.GetCachedOrFetch(ctx); fredErr == nil && macroData != nil {
		r := fred.ClassifyMacroRegime(macroData)
		cachedRegime = &r
		a.lastRegime = cachedRegime
		log.Info().Str("regime", r.Name).Msg("FRED regime loaded for scoring")
	}

	var analyses []domain.COTAnalysis

	for _, record := range records {
		// Get historical data for index calculation (52 weeks)
		history, err := a.cotRepo.GetHistory(ctx, record.ContractCode, 52)
		if err != nil {
			log.Warn().Str("contract", record.ContractCode).Err(err).Msg("failed to get history")
			history = []domain.COTRecord{record} // use just current if no history
		}

		analysis := a.computeMetrics(record, history, cachedRegime)
		analyses = append(analyses, analysis)
	}

	// Save analyses
	if err := a.cotRepo.SaveAnalyses(ctx, analyses); err != nil {
		return analyses, fmt.Errorf("save analyses: %w", err)
	}

	log.Info().Int("contracts", len(analyses)).Msg("analyzed contracts")
	return analyses, nil
}

// SyncHistory fetches a full year of history for all default contracts
// and saves them to the repository. This should be run on first start.
func (a *Analyzer) SyncHistory(ctx context.Context) error {
	contracts := domain.DefaultCOTContracts
	records, err := a.fetcher.FetchAllHistory(ctx, contracts)
	if err != nil {
		return fmt.Errorf("fetch all history: %w", err)
	}

	if err := a.cotRepo.SaveRecords(ctx, records); err != nil {
		return fmt.Errorf("save history records: %w", err)
	}

	log.Info().Int("records", len(records)).Msg("history synced")

	// Run analysis on all synced data — both latest (for live use)
	// and full history (for bootstrap backtest).
	if _, err = a.AnalyzeAll(ctx); err != nil {
		return err
	}
	return a.AnalyzeHistory(ctx)
}

// AnalyzeHistory computes and stores analysis records for ALL historical
// COT records across all contracts. This populates GetAnalysisHistory()
// with 52 weeks of data so the backtest bootstrap can replay signals
// across the full history window.
func (a *Analyzer) AnalyzeHistory(ctx context.Context) error {
	// Best-effort FRED regime (may be nil)
	var cachedRegime *fred.MacroRegime
	if macroData, fredErr := fred.GetCachedOrFetch(ctx); fredErr == nil && macroData != nil {
		r := fred.ClassifyMacroRegime(macroData)
		cachedRegime = &r
	}

	totalAnalyses := 0
	for _, contract := range domain.DefaultCOTContracts {
		history, err := a.cotRepo.GetHistory(ctx, contract.Code, 52)
		if err != nil || len(history) < 2 {
			continue
		}

		// history is newest-first; reverse to oldest-first for windowing
		for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
			history[i], history[j] = history[j], history[i]
		}

		var analyses []domain.COTAnalysis
		for i := range history {
			// Build a history window from the start up to (and including) this record
			window := history[:i+1]
			// computeMetrics expects newest-first history, so reverse the window
			reversed := make([]domain.COTRecord, len(window))
			for k := range window {
				reversed[len(window)-1-k] = window[k]
			}
			analysis := a.computeMetrics(history[i], reversed, cachedRegime)
			analyses = append(analyses, analysis)
		}

		if err := a.cotRepo.SaveAnalyses(ctx, analyses); err != nil {
			log.Warn().Err(err).Str("contract", contract.Code).Msg("failed to save historical analyses")
			continue
		}
		totalAnalyses += len(analyses)
	}

	log.Info().Int("analyses", totalAnalyses).Msg("historical analyses computed for backtest")
	return nil
}

// BackfillRegimeScores fetches all stored analyses, populates RegimeAdjustedScore
// using the current FRED regime, and re-saves them. This ensures existing records
// written before Gap B was implemented carry the new field.
// Safe to call multiple times — only updates analyses where the field is still zero.
func (a *Analyzer) BackfillRegimeScores(ctx context.Context) error {
	macroData, err := fred.GetCachedOrFetch(ctx)
	if err != nil || macroData == nil {
		return fmt.Errorf("backfill: no FRED data available: %w", err)
	}
	regime := fred.ClassifyMacroRegime(macroData)

	analyses, err := a.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil {
		return fmt.Errorf("backfill: get analyses: %w", err)
	}

	updated := 0
	for i := range analyses {
		if analyses[i].RegimeAdjustedScore == 0 {
			analyses[i].RegimeAdjustedScore = ComputeRegimeAdjustedScore(analyses[i], regime)
			updated++
		}
	}

	if updated == 0 {
		log.Info().Msg("backfill: all analyses already have RegimeAdjustedScore, skipping save")
		return nil
	}

	if err := a.cotRepo.SaveAnalyses(ctx, analyses); err != nil {
		return fmt.Errorf("backfill: save analyses: %w", err)
	}

	log.Info().Int("updated", updated).Int("total", len(analyses)).Msg("backfill: populated RegimeAdjustedScore")
	return nil
}

// AnalyzeContract computes metrics for a single contract.
// Uses the cached FRED regime from the last AnalyzeAll run if available.
func (a *Analyzer) AnalyzeContract(ctx context.Context, contractCode string) (*domain.COTAnalysis, error) {
	latest, err := a.cotRepo.GetLatest(ctx, contractCode)
	if err != nil {
		return nil, fmt.Errorf("get latest: %w", err)
	}

	history, err := a.cotRepo.GetHistory(ctx, contractCode, 52)
	if err != nil {
		history = []domain.COTRecord{*latest}
	}

	analysis := a.computeMetrics(*latest, history, a.lastRegime)
	return &analysis, nil
}

// computeMetrics calculates all 20+ COT metrics from a record + history.
// regime is optional — when non-nil, RegimeAdjustedScore is populated (Gap B).
func (a *Analyzer) computeMetrics(current domain.COTRecord, history []domain.COTRecord, regime *fred.MacroRegime) domain.COTAnalysis {
	contract := findContractByCode(current.ContractCode)
	rt := contract.ReportType

	analysis := domain.COTAnalysis{
		Contract:       contract,
		ReportDate:     current.ReportDate,
		COTIndex:       50.0,
		COTIndexComm:   50.0,
		SentimentScore: 0.0,
		MomentumDir:    "FLAT",
	}

	// === Core Position Metrics (Modern) ===

	// 1. Smart Money Net (Leveraged Funds or Managed Money)
	analysis.NetPosition = current.GetSmartMoneyNet(rt)
	analysis.CommercialNet = current.GetCommercialNet(rt)
	analysis.SmallSpecNet = current.GetSmallSpecNet()

	// Sync legacy fields
	analysis.NetCommercial = analysis.CommercialNet
	analysis.NetSmallSpec = analysis.SmallSpecNet

	// Specific breakdowns
	if rt == "TFF" {
		analysis.LevFundNet = current.LevFundLong - current.LevFundShort
	} else {
		analysis.ManagedMoneyNet = current.ManagedMoneyLong - current.ManagedMoneyShort
	}

	// 2. Net change (week-over-week)
	// Priority: use API-provided change fields (CFTC-computed, more accurate).
	// Fallback: compute from history diff if API value is zero.
	if current.NetChange != 0 {
		analysis.NetChange = current.NetChange
	} else if len(history) > 1 {
		prev := history[1]
		analysis.NetChange = analysis.NetPosition - prev.GetSmartMoneyNet(rt)
	}
	if len(history) > 1 {
		prev := history[1]
		// CommNetChange: prefer API-provided change fields
		if rt == "TFF" && (current.DealerLongChg != 0 || current.DealerShortChg != 0) {
			analysis.CommNetChange = current.DealerLongChg - current.DealerShortChg
		} else if rt == "DISAGGREGATED" && (current.ProdMercLongChg != 0 || current.ProdMercShortChg != 0 || current.SwapLongChg != 0 || current.SwapShortChg != 0) {
			analysis.CommNetChange = (current.ProdMercLongChg - current.ProdMercShortChg) + (current.SwapLongChg - current.SwapShortChg)
		} else {
			analysis.CommNetChange = analysis.CommercialNet - prev.GetCommercialNet(rt)
		}
	}

	// 3. Long/Short ratios
	if rt == "TFF" {
		analysis.LongShortRatio = safeRatio(current.LevFundLong, current.LevFundShort)
		analysis.CommLSRatio = safeRatio(current.DealerLong, current.DealerShort)
	} else {
		analysis.LongShortRatio = safeRatio(current.ManagedMoneyLong, current.ManagedMoneyShort)
		analysis.CommLSRatio = safeRatio(current.ProdMercLong+current.SwapDealerLong, current.ProdMercShort+current.SwapDealerShort)
	}

	// 4. Percentage of Open Interest
	analysis.OpenInterest = current.OpenInterest
	if current.OpenInterest > 0 {
		oi := current.OpenInterest
		analysis.PctOfOI = analysis.NetPosition / oi * 100
		analysis.CommPctOfOI = analysis.CommercialNet / oi * 100
	}

	// 5. Open Interest change — use API field if available
	if current.OIChangeAPI != 0 {
		analysis.OpenInterestChg = current.OIChangeAPI
		prevOI := current.OpenInterest - current.OIChangeAPI
		if prevOI > 0 {
			analysis.OIPctChange = current.OIChangeAPI / prevOI * 100
		}
	} else if len(history) > 1 {
		prevOI := history[1].OpenInterest
		if prevOI > 0 {
			analysis.OpenInterestChg = current.OpenInterest - prevOI
			analysis.OIPctChange = analysis.OpenInterestChg / prevOI * 100
		}
	}

	// 5b. Spread positions as % of OI — now populated from API
	totalSpread := current.GetTotalSpread(rt)
	if current.OpenInterest > 0 && totalSpread > 0 {
		analysis.SpreadPctOfOI = totalSpread / current.OpenInterest * 100
	}
	// Determine OI Context (Trend)
	analysis.OITrend = "FLAT"
	if analysis.OIPctChange > 1.0 {
		analysis.OITrend = "RISING"
	} else if analysis.OIPctChange < -1.0 {
		analysis.OITrend = "FALLING"
	}

	// === Index Metrics (Larry Williams COT Index) ===

	if len(history) >= 3 {
		// 6. COT Index for smart money (0-100)
		smartNets := extractNets(history, func(r domain.COTRecord) float64 { return r.GetSmartMoneyNet(rt) })
		analysis.COTIndex = computeCOTIndex(smartNets)

		// 7. COT Index for commercials
		commNets := extractNets(history, func(r domain.COTRecord) float64 { return r.GetCommercialNet(rt) })
		analysis.COTIndexComm = computeCOTIndex(commNets)

		// 7b. WillcoIndex — blended smart money + commercial index for cross-confirmation.
		// Values near 0 or 100 confirm extreme positioning; ~50 = no confirmation.
		// Used by signals.detectExtreme as secondary confirmation (|WillcoIndex-50| > 30).
		if len(smartNets) >= 3 && len(commNets) >= 3 {
			analysis.WillcoIndex = (analysis.COTIndex + (100 - analysis.COTIndexComm)) / 2
		} else {
			analysis.WillcoIndex = 50.0 // neutral when insufficient data
		}
	}

	// === Momentum Metrics ===

	if len(history) >= 4 {
		// 8. Smart Money momentum (4-week)
		smartNets4 := extractNets(history[:minInt(5, len(history))], func(r domain.COTRecord) float64 {
			return r.GetSmartMoneyNet(rt)
		})
		analysis.SpecMomentum4W = mathutil.Momentum(reverseFloats(smartNets4), 4)

		// 9. Commercial momentum (4-week)
		commNets4 := extractNets(history[:minInt(5, len(history))], func(r domain.COTRecord) float64 {
			return r.GetCommercialNet(rt)
		})
		analysis.CommMomentum4W = mathutil.Momentum(reverseFloats(commNets4), 4)

		// 10. Momentum direction
		analysis.MomentumDir = classifyMomentumDir(analysis.SpecMomentum4W, analysis.CommMomentum4W)
	}

	if len(history) >= 9 {
		// 10b. SpecMomentum8W — longer-term trend filter (now used in signal logic)
		smartNets8 := extractNets(history[:minInt(9, len(history))], func(r domain.COTRecord) float64 {
			return r.GetSmartMoneyNet(rt)
		})
		analysis.SpecMomentum8W = mathutil.Momentum(reverseFloats(smartNets8), 8)
	}

	// 10c. ConsecutiveWeeks — count weeks spec net has been moving same direction
	if len(history) >= 2 {
		direction := 0
		if analysis.NetChange > 0 {
			direction = 1
		} else if analysis.NetChange < 0 {
			direction = -1
		}
		count := 0
		for i := 0; i < len(history)-1 && i < 26; i++ {
			curr := history[i].GetSmartMoneyNet(rt)
			prev := history[i+1].GetSmartMoneyNet(rt)
			diff := curr - prev
			weekDir := 0
			if diff > 0 {
				weekDir = 1
			} else if diff < 0 {
				weekDir = -1
			}
			if weekDir == direction {
				count++
			} else {
				break
			}
		}
		analysis.ConsecutiveWeeks = count
	}

	// === Sentiment & Signal Metrics ===

	// 11. Overall sentiment score (-100 to +100)
	analysis.SentimentScore = computeSentiment(analysis)

	// === 11b. Institutional Outlier Alerts (TFF) ===
	if rt == "TFF" && len(history) >= 4 {
		// Extract historical AssetMgrNet week-over-week changes.
		// Start at i=2 to exclude the current week's change from the
		// distribution — including it would bias mean/stddev toward the
		// test value and reduce Z-score sensitivity.
		var changes []float64
		for i := 2; i < len(history); i++ {
			currAM := history[i-1].AssetMgrLong - history[i-1].AssetMgrShort
			prevAM := history[i].AssetMgrLong - history[i].AssetMgrShort
			changes = append(changes, currAM-prevAM)
		}

		if len(changes) >= 2 {
			// Current week's change: history[0] (newest) vs history[1] (previous)
			currentChange := (current.AssetMgrLong - current.AssetMgrShort) - (history[1].AssetMgrLong - history[1].AssetMgrShort)
			avg := mathutil.Mean(changes)
			stdDev := mathutil.StdDevSample(changes)

			if stdDev > 0 {
				analysis.AssetMgrZScore = (currentChange - avg) / stdDev
				// Alert if Z-Score exceeds 2.0 (95% confidence outlier)
				if mathutil.Abs(analysis.AssetMgrZScore) >= 2.0 {
					analysis.AssetMgrAlert = true
				}
			}
		}

		// Additional per-category Z-scores (Dealer, LevFund, ManagedMoney, SwapDealer)
		computeAllCategoryZScores(&analysis, history)
	}

	// 12-14. Individual trader signals
	analysis.CommercialSignal = classifySignal(analysis.COTIndexComm, analysis.CommMomentum4W, true)
	analysis.SpeculatorSignal = classifySignal(analysis.COTIndex, analysis.SpecMomentum4W, false)
	analysis.SmallSpecSignal = classifySmallSpec(analysis)

	// === Advanced Metrics ===

	// 15. Divergence flag (commercials vs smart money moving opposite)
	analysis.DivergenceFlag = detectDivergence(analysis.NetChange, analysis.CommNetChange)

	// 15b. Scalper/Intraday Bias — uses 4W momentum + 8W trend filter
	// SpecMomentum8W acts as a higher-timeframe trend filter to reduce noise.
	analysis.ShortTermBias = "NEUTRAL"
	m4 := analysis.SpecMomentum4W
	m8 := analysis.SpecMomentum8W // 0 if not enough history
	trendConfirmed := m8 == 0 || (m4 > 0 && m8 > 0) || (m4 < 0 && m8 < 0)

	if m4 > 0 {
		if analysis.OITrend == "RISING" && trendConfirmed {
			analysis.ShortTermBias = "STRONG BUY (Trend Confirmed)"
		} else if analysis.OITrend == "RISING" && !trendConfirmed {
			analysis.ShortTermBias = "BUY DIPS (8W trend opposing — caution)"
		} else if analysis.OITrend == "FALLING" {
			analysis.ShortTermBias = "WEAK BUY (Short Covering)"
		} else {
			analysis.ShortTermBias = "BUY DIPS"
		}
	} else if m4 < 0 {
		if analysis.OITrend == "RISING" && trendConfirmed {
			analysis.ShortTermBias = "STRONG SELL (Trend Confirmed)"
		} else if analysis.OITrend == "RISING" && !trendConfirmed {
			analysis.ShortTermBias = "SELL RALLIES (8W trend opposing — caution)"
		} else if analysis.OITrend == "FALLING" {
			analysis.ShortTermBias = "WEAK SELL (Long Liquidation)"
		} else {
			analysis.ShortTermBias = "SELL RALLIES"
		}
	}

	// 16. Crowding index
	analysis.CrowdingIndex = computeCrowding(current, rt)

	// 17-18. Concentration (top 4/8 traders)
	analysis.Top4Concentration = (current.Top4Long + current.Top4Short) / 2
	analysis.Top4LongPct = current.Top4Long
	analysis.Top4ShortPct = current.Top4Short
	analysis.Top8Concentration = (current.Top8Long + current.Top8Short) / 2

	// === Extreme Detection ===
	analysis.IsExtremeBull = analysis.COTIndex > 90
	analysis.IsExtremeBear = analysis.COTIndex < 10
	analysis.CommExtremeBull = analysis.COTIndexComm > 90
	analysis.CommExtremeBear = analysis.COTIndexComm < 10

	// === Smart Money vs Dumb Money ===
	analysis.SmartDumbDivergence = (analysis.NetPosition > 0 && analysis.CommercialNet < 0) ||
		(analysis.NetPosition < 0 && analysis.CommercialNet > 0)

	// === Signal Strength ===
	analysis.SignalStrength = classifySignalStrength(analysis)

	// === Trader Concentration Analysis (NEW) ===
	// Populate trader counts from record
	if rt == "TFF" {
		analysis.DealerShortTraders  = current.DealerShortTraders
		analysis.LevFundLongTraders  = current.LevFundLongTraders
		analysis.LevFundShortTraders = current.LevFundShortTraders
		analysis.AssetMgrLongTraders = current.AssetMgrLongTraders
		analysis.TotalTraders        = current.TotalTraders
	} else {
		analysis.MMoneyLongTraders  = current.MMoneyLongTraders
		analysis.MMoneyShortTraders = current.MMoneyShortTraders
		analysis.TotalTraders       = current.TotalTradersDisag
	}

	// Thin market detection: flag when key category has very few traders
	const thinThreshold = 10 // tightened from 15: <10 traders = genuinely thin
	analysis.ThinMarketAlert = false
	if rt == "TFF" {
		switch {
		case current.DealerShortTraders > 0 && current.DealerShortTraders < thinThreshold:
			analysis.ThinMarketAlert = true
			analysis.ThinMarketDesc = fmt.Sprintf("Only %d dealers short — extreme concentration, reversal risk HIGH", current.DealerShortTraders)
		case current.LevFundLongTraders > 0 && current.LevFundLongTraders < thinThreshold:
			analysis.ThinMarketAlert = true
			analysis.ThinMarketDesc = fmt.Sprintf("Only %d lev funds long — thin longs, squeeze risk", current.LevFundLongTraders)
		case current.LevFundShortTraders > 0 && current.LevFundShortTraders < thinThreshold:
			analysis.ThinMarketAlert = true
			analysis.ThinMarketDesc = fmt.Sprintf("Only %d lev funds short — thin shorts, short-squeeze risk", current.LevFundShortTraders)
		}
	} else {
		switch {
		case current.MMoneyLongTraders > 0 && current.MMoneyLongTraders < thinThreshold:
			analysis.ThinMarketAlert = true
			analysis.ThinMarketDesc = fmt.Sprintf("Only %d managed money long — thin longs", current.MMoneyLongTraders)
		case current.MMoneyShortTraders > 0 && current.MMoneyShortTraders < thinThreshold:
			analysis.ThinMarketAlert = true
			analysis.ThinMarketDesc = fmt.Sprintf("Only %d managed money short — thin shorts", current.MMoneyShortTraders)
		}
	}

	// Trader concentration label
	switch {
	case analysis.TotalTraders > 200:
		analysis.TraderConcentration = "DEEP"
	case analysis.TotalTraders > 100:
		analysis.TraderConcentration = "NORMAL"
	case analysis.TotalTraders > 0:
		analysis.TraderConcentration = "THIN"
	}

	// === Gap B — Regime-Adjusted Score ===
	// Populate RegimeAdjustedScore when FRED regime is available.
	// This mathematically links macro regime to the COT sentiment score.
	if regime != nil {
		analysis.RegimeAdjustedScore = ComputeRegimeAdjustedScore(analysis, *regime)
	}

	// Options-derived metrics
	if current.HasOptions {
		optNet := current.OptSmartMoneyLong - current.OptSmartMoneyShort
		analysis.OptionsNetPosition = optNet
		if current.OpenInterest > 0 {
			analysis.OptionsPctOfTotalOI = current.OptionsOI / current.OpenInterest * 100
		}
		if current.OptSmartMoneyLong > current.OptSmartMoneyShort*1.3 {
			analysis.OptionsSmartBias = "CALL-HEAVY"
		} else if current.OptSmartMoneyShort > current.OptSmartMoneyLong*1.3 {
			analysis.OptionsSmartBias = "PUT-HEAVY"
		} else {
			analysis.OptionsSmartBias = "BALANCED"
		}
	}

	return analysis
}

// --- computation helpers ---

// findContractByCode looks up a COTContract from DefaultCOTContracts by code.
func findContractByCode(code string) domain.COTContract {
	for _, c := range domain.DefaultCOTContracts {
		if c.Code == code {
			return c
		}
	}
	// Return a minimal contract if not found
	return domain.COTContract{Code: code}
}

// computeCOTIndex implements the Larry Williams COT Index formula:
// Index = (Current Net - Min Net) / (Max Net - Min Net) * 100
// FIX: Uses float64 throughout (was int64)
func computeCOTIndex(nets []float64) float64 {
	if len(nets) < 3 {
		return 50.0 // neutral if insufficient data
	}

	current := nets[0]
	minVal, maxVal := nets[0], nets[0]

	for _, n := range nets {
		if n < minVal {
			minVal = n
		}
		if n > maxVal {
			maxVal = n
		}
	}

	span := maxVal - minVal
	if span == 0 {
		return 50.0
	}

	return mathutil.Clamp((current-minVal)/span*100, 0, 100)
}

// computeSentiment creates a composite score from all factors.
func computeSentiment(a domain.COTAnalysis) float64 {
	score := 0.0

	// COT Index contribution (40% weight)
	indexScore := (a.COTIndex - 50) * 2
	score += indexScore * 0.40

	// Commercial positioning (30% weight).
	// For TFF (forex/indices/bonds): dealers are counterparties, not smart money.
	//   High dealer COTIndex → forced long by client selling → bearish → invert.
	// For DISAGGREGATED (commodities): commercials are producers/hedgers (smart money).
	//   High commercial COTIndex → producers net long → bullish → same direction.
	var commScore float64
	if a.Contract.ReportType == "DISAGGREGATED" {
		commScore = (a.COTIndexComm - 50) * 2 // same direction: high = bullish
	} else {
		commScore = (50 - a.COTIndexComm) * 2 // inverted: high = bearish (contrarian to dealers)
	}
	score += commScore * 0.30

	// Momentum contribution (20% weight) — continuous scaling
	// Scale factor: 5000 contracts momentum → ±20 points (full weight)
	momScale := 5000.0
	if momScale > 0 {
		momContrib := mathutil.Clamp(a.SpecMomentum4W/momScale*20, -20, 20)
		score += momContrib
	}

	// Crowding penalty (10% weight) — proportional to crowding degree
	// CrowdingIndex 50 = neutral, >50 = crowded (penalty), <50 = uncrowded (bonus)
	crowdingContrib := mathutil.Clamp((50-a.CrowdingIndex)*0.2, -10, 10)
	score += crowdingContrib

	return mathutil.Clamp(score, -100, 100)
}

// classifySignal generates a directional signal from COT index + momentum.
// classifySignal generates a directional signal from COT index + momentum.
//
// For speculators (isCommercial=false):
//   - High COT index (>75) + positive momentum = STRONG_BULLISH
//   - Low COT index (<25) + negative momentum = STRONG_BEARISH
//
// For commercials (isCommercial=true), the interpretation is CONTRARIAN:
//   - Commercials at extreme HIGH (>75) = they are fully long = BULLISH (they are smart money at turning points)
//   - Commercials at extreme LOW (<25)  = they are fully short = BEARISH (they know something is overvalued)
//   - Momentum confirms: commercial increasing their extreme = stronger signal
func classifySignal(cotIndex, momentum float64, isCommercial bool) string {
	if !isCommercial {
		// Speculator: directional signal
		switch {
		case cotIndex >= 75 && momentum > 0:
			return "STRONG_BULLISH"
		case cotIndex >= 75:
			return "BULLISH"
		case cotIndex <= 25 && momentum < 0:
			return "STRONG_BEARISH"
		case cotIndex <= 25:
			return "BEARISH"
		default:
			return "NEUTRAL"
		}
	}

	// Commercial: contrarian signal
	// High commercial index = commercials net long = bullish for price (contrarian)
	// Low commercial index = commercials net short = bearish for price (contrarian)
	switch {
	case cotIndex >= 75 && momentum > 0:
		return "STRONG_BULLISH"
	case cotIndex >= 75:
		return "BULLISH"
	case cotIndex <= 25 && momentum < 0:
		return "STRONG_BEARISH"
	case cotIndex <= 25:
		return "BEARISH"
	default:
		return "NEUTRAL"
	}
}

// classifySmallSpec generates small speculator signal (contrarian indicator).
func classifySmallSpec(a domain.COTAnalysis) string {
	if a.NetSmallSpec > 0 && a.CrowdingIndex > 65 {
		return "CROWD_LONG"
	}
	if a.NetSmallSpec < 0 && a.CrowdingIndex > 65 {
		return "CROWD_SHORT"
	}
	return "NEUTRAL"
}

// classifyMomentumDir determines the overall positioning momentum direction.
// FIX: Returns domain.MomentumDirection type (was string)
func classifyMomentumDir(specMom, commMom float64) domain.MomentumDirection {
	if specMom > 0 && commMom < 0 {
		return domain.MomentumBuilding // spec building, comm unwinding
	}
	if specMom < 0 && commMom > 0 {
		return domain.MomentumReversing // spec reducing, comm adding
	}
	if math.Abs(specMom) < 100 && math.Abs(commMom) < 100 {
		return domain.MomentumStable
	}
	if specMom < 0 {
		return domain.MomentumUnwinding
	}
	return domain.MomentumBuilding
}

// classifySignalStrength rates the conviction level of the COT signal.
func classifySignalStrength(a domain.COTAnalysis) domain.SignalStrength {
	absSentiment := math.Abs(a.SentimentScore)
	switch {
	case absSentiment > 60 && (a.IsExtremeBull || a.IsExtremeBear):
		return domain.SignalStrong
	case absSentiment > 40:
		return domain.SignalModerate
	case absSentiment > 20:
		return domain.SignalWeak
	default:
		return domain.SignalNeutral
	}
}

// detectDivergence checks if commercials and speculators are moving in opposite directions.
// FIX: Takes float64 params instead of accessing non-existent fields
func detectDivergence(specNetChange, commNetChange float64) bool {
	specDir := signF(specNetChange)
	commDir := signF(commNetChange)

	if math.Abs(specNetChange) < 1000 || math.Abs(commNetChange) < 1000 {
		return false
	}

	return specDir != 0 && commDir != 0 && specDir != commDir
}

// computeCrowding measures how one-sided speculative positioning is (0-100).
// It focuses on the speculative trader category (leveraged funds for TFF,
// managed money for DISAGG) because total market longs always equal shorts
// in futures markets — making aggregate crowding meaningless.
//
// 0 = speculators perfectly balanced (50/50 long/short)
// 100 = speculators fully one-sided (all long or all short)
func computeCrowding(r domain.COTRecord, reportType string) float64 {
	var specLong, specShort float64

	if reportType == "TFF" {
		// Leveraged funds = speculative/hedge fund positioning
		specLong = r.LevFundLong
		specShort = r.LevFundShort
	} else {
		// Managed money = speculative/CTA positioning
		specLong = r.ManagedMoneyLong
		specShort = r.ManagedMoneyShort
	}

	specTotal := specLong + specShort
	if specTotal == 0 {
		return 50.0 // No speculative positions → neutral
	}

	longPct := specLong / specTotal * 100
	deviation := math.Abs(longPct - 50)

	return mathutil.Clamp(deviation*2, 0, 100)
}

// --- utility helpers ---

// FIX: extractNets returns []float64 (was []int64)
func extractNets(history []domain.COTRecord, fn func(domain.COTRecord) float64) []float64 {
	nets := make([]float64, len(history))
	for i, r := range history {
		nets[i] = fn(r)
	}
	return nets
}

// safeRatio computes a/b safely, returning 0 or 999.99 on edge cases.
// FIX: Parameters are float64 (were cast from int64)
func safeRatio(a, b float64) float64 {
	if b == 0 {
		if a > 0 {
			return 999.99
		}
		return 0
	}
	return math.Round(a/b*100) / 100
}

// signF returns the sign of a float64 value.
// FIX: Renamed from sign() and takes float64 (was int64)
func signF(v float64) float64 {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

// minInt returns the minimum of two ints.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// reverseFloats returns a new slice with elements in reverse order.
// Used to convert newest-first data (from GetHistory) to oldest-first
// order required by mathutil.Momentum.
func reverseFloats(s []float64) []float64 {
	out := make([]float64, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return out
}
