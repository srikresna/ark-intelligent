package backtest

import (
	"context"
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
	cotsvc "github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// SignalExistenceChecker is the subset of SignalRepository needed for dedup.
type SignalExistenceChecker interface {
	SignalExists(ctx context.Context, contractCode string, reportDate time.Time, signalType string) (bool, error)
}

// COTHistoryProvider provides COT analysis and record history.
// Extends ports.COTRepository with GetAnalysisHistory.
type COTHistoryProvider interface {
	ports.COTRepository
	GetAnalysisHistory(ctx context.Context, contractCode string, weeks int) ([]domain.COTAnalysis, error)
}

// HistoricalDailyPriceStore extends DailyPriceStore with date-relative lookups
// needed for bootstrapping historical signals without look-ahead bias.
type HistoricalDailyPriceStore interface {
	pricesvc.DailyPriceStore
	GetDailyHistoryBefore(ctx context.Context, contractCode string, before time.Time, days int) ([]domain.DailyPrice, error)
}

// historicalDailyAdapter wraps a HistoricalDailyPriceStore to implement
// DailyPriceStore with a fixed "as of" date, so DailyContextBuilder returns
// context for a historical point in time instead of today.
type historicalDailyAdapter struct {
	store HistoricalDailyPriceStore
	asOf  time.Time
}

func (h *historicalDailyAdapter) GetDailyHistory(ctx context.Context, contractCode string, days int) ([]domain.DailyPrice, error) {
	return h.store.GetDailyHistoryBefore(ctx, contractCode, h.asOf, days)
}

// Bootstrapper replays historical COT data against historical prices
// to create a retroactive backtest dataset. Safe to run multiple times
// due to key-based deduplication.
type Bootstrapper struct {
	cotRepo    COTHistoryProvider
	priceRepo  ports.PriceRepository
	signalRepo ports.SignalRepository
	sigChecker SignalExistenceChecker
	detector   *cotsvc.SignalDetector
	dailyRepo  HistoricalDailyPriceStore // optional — enables DailyTrendFilter for bootstrap signals
}

// NewBootstrapper creates a new backtest bootstrapper.
func NewBootstrapper(
	cotRepo COTHistoryProvider,
	priceRepo ports.PriceRepository,
	signalRepo ports.SignalRepository,
	sigChecker SignalExistenceChecker,
	dailyRepo ...HistoricalDailyPriceStore,
) *Bootstrapper {
	b := &Bootstrapper{
		cotRepo:    cotRepo,
		priceRepo:  priceRepo,
		signalRepo: signalRepo,
		sigChecker: sigChecker,
		detector:   cotsvc.NewSignalDetector(),
	}
	if len(dailyRepo) > 0 && dailyRepo[0] != nil {
		b.dailyRepo = dailyRepo[0]
	}
	return b
}

// Run replays historical COT data to generate and persist signal snapshots.
// Returns the number of new signals created.
func (b *Bootstrapper) Run(ctx context.Context) (int, error) {
	log.Info().Msg("Starting backtest bootstrap")

	totalCreated := 0

	// Use COTPriceSymbolMappings to exclude risk-only instruments (VIX, SPX).
	// Bootstrapping VIX/SPX as COT contracts would generate nonsense signals.
	for _, mapping := range domain.COTPriceSymbolMappings() {
		created, err := b.bootstrapContract(ctx, mapping)
		if err != nil {
			log.Warn().Err(err).Str("contract", mapping.Currency).Msg("Bootstrap failed for contract")
			continue
		}
		totalCreated += created
	}

	log.Info().Int("signals_created", totalCreated).Msg("Backtest bootstrap complete")
	return totalCreated, nil
}

// bootstrapContract generates signals for a single contract across its history.
func (b *Bootstrapper) bootstrapContract(ctx context.Context, mapping domain.PriceSymbolMapping) (int, error) {
	// Load full COT history (52 weeks) — returned newest-first from storage.
	cotHistory, err := b.cotRepo.GetHistory(ctx, mapping.ContractCode, 52)
	if err != nil {
		return 0, fmt.Errorf("get COT history: %w", err)
	}
	if len(cotHistory) < 8 {
		return 0, nil // Not enough history for meaningful signals
	}

	// Reverse to oldest-first for buildHistoryWindow chronological scanning.
	for i, j := 0, len(cotHistory)-1; i < j; i, j = i+1, j-1 {
		cotHistory[i], cotHistory[j] = cotHistory[j], cotHistory[i]
	}

	// Load analysis history — also newest-first from storage.
	analyses, err := b.cotRepo.GetAnalysisHistory(ctx, mapping.ContractCode, 52)
	if err != nil {
		return 0, fmt.Errorf("get analysis history: %w", err)
	}
	if len(analyses) == 0 {
		return 0, nil
	}

	// Reverse to oldest-first so we replay chronologically.
	for i, j := 0, len(analyses)-1; i < j; i, j = i+1, j-1 {
		analyses[i], analyses[j] = analyses[j], analyses[i]
	}

	created := 0

	// For each analysis week, simulate signal detection
	for i := range analyses {
		select {
		case <-ctx.Done():
			return created, ctx.Err()
		default:
		}

		analysis := &analyses[i]

		// Build an 8-week history window ending at this analysis's report date
		historyWindow := buildHistoryWindow(cotHistory, analysis.ReportDate, 8)
		if len(historyWindow) < 4 {
			continue // Not enough history context
		}

		// Run signal detection on this single analysis with its history context.
		// DetectAll expects history in newest-first order (consistent with live path
		// where GetHistory returns newest-first). Reverse the window for DetectAll.
		reversedWindow := make([]domain.COTRecord, len(historyWindow))
		for ri, rj := 0, len(historyWindow)-1; ri <= rj; ri, rj = ri+1, rj-1 {
			reversedWindow[ri], reversedWindow[rj] = historyWindow[rj], historyWindow[ri]
		}
		historyMap := map[string][]domain.COTRecord{
			mapping.ContractCode: reversedWindow,
		}
		signals := b.detector.DetectAll([]domain.COTAnalysis{*analysis}, historyMap)
		if len(signals) == 0 {
			continue
		}

		// Get the entry price for this report date — skip if unavailable
		entryPrice, err := b.priceRepo.GetPriceAt(ctx, mapping.ContractCode, analysis.ReportDate)
		if err != nil {
			log.Debug().Err(err).Str("contract", mapping.ContractCode).Msg("No price data for bootstrap")
			continue
		}
		if entryPrice == nil || entryPrice.Close <= 0 {
			log.Debug().
				Str("contract", mapping.ContractCode).
				Time("report_date", analysis.ReportDate).
				Msg("No valid entry price — skipping signal creation")
			continue
		}
		entryClose := entryPrice.Close

		// Convert detected signals to persisted signals
		var toSave []domain.PersistedSignal
		for _, sig := range signals {
			// Check for duplicates
			exists, err := b.sigChecker.SignalExists(ctx, mapping.ContractCode, analysis.ReportDate, string(sig.Type))
			if err != nil {
				continue
			}
			if exists {
				continue
			}

			ps := domain.PersistedSignal{
				ContractCode:   mapping.ContractCode,
				Currency:       mapping.Currency,
				SignalType:     string(sig.Type),
				Direction:      sig.Direction,
				Strength:       sig.Strength,
				Confidence:     sig.Confidence,
				Description:    sig.Description,
				ReportDate:     analysis.ReportDate,
				DetectedAt:     analysis.ReportDate, // Retroactive — use report date
				EntryPrice:     entryClose,
				Inverse:        mapping.Inverse,
				COTIndex:       analysis.COTIndex,
				SentimentScore: analysis.SentimentScore,
			}

			// ConvictionScore: compute simplified score using only COT data.
			// Bootstrap lacks historical FRED data and calendar surprises,
			// so we pass neutral regime/calendar — only COT component contributes.
			cs := cotsvc.ComputeConvictionScoreV3(*analysis, fred.MacroRegime{}, 0, "", nil, nil)
			ps.ConvictionScore = cs.Score

			// FREDRegime: left empty — the BackfillRegimeLabels() mechanism
			// retroactively populates this field from stored FRED snapshots.

			// DailyTrend fields: apply DailyTrendFilter if daily price data is available.
			// Uses historicalDailyAdapter to get data as-of the report date (no look-ahead).
			if b.dailyRepo != nil {
				adapter := &historicalDailyAdapter{store: b.dailyRepo, asOf: analysis.ReportDate}
				dailyBuilder := pricesvc.NewDailyContextBuilder(adapter)
				trendFilter := NewDailyTrendFilter(dailyBuilder)
				adj := trendFilter.Adjust(ctx, mapping.ContractCode, mapping.Currency, sig.Direction, ps.Confidence)
				if adj.Adjustment != 0 {
					ps.RawConfidence = adj.RawConfidence
					ps.Confidence = adj.AdjustedConfidence
					ps.DailyTrend = adj.DailyTrend
					ps.DailyMATrend = adj.MATrend
					ps.DailyTrendAdj = adj.Adjustment
				}
			}

			toSave = append(toSave, ps)
		}

		if len(toSave) > 0 {
			if err := b.signalRepo.SaveSignals(ctx, toSave); err != nil {
				log.Warn().Err(err).Msg("Failed to save bootstrap signals")
				continue
			}
			created += len(toSave)
		}
	}

	return created, nil
}

// buildHistoryWindow extracts COT records up to and including the target date,
// returning at most `maxWeeks` records in oldest-first order.
// IMPORTANT: allRecords MUST be in oldest-first (chronological) order.
func buildHistoryWindow(allRecords []domain.COTRecord, targetDate time.Time, maxWeeks int) []domain.COTRecord {
	// Runtime assertion: verify input is sorted oldest-first.
	// If the caller passes newest-first data, the break-on-After logic
	// would silently return an empty or truncated window, producing
	// incorrect backtest results.
	for i := 1; i < len(allRecords); i++ {
		if allRecords[i].ReportDate.Before(allRecords[i-1].ReportDate) {
			log.Error().
				Time("prev", allRecords[i-1].ReportDate).
				Time("curr", allRecords[i].ReportDate).
				Int("index", i).
				Msg("buildHistoryWindow: allRecords not in chronological order — results may be incorrect")
			break // log once, don't spam
		}
	}

	var window []domain.COTRecord
	for i := range allRecords {
		if allRecords[i].ReportDate.After(targetDate) {
			break
		}
		window = append(window, allRecords[i])
	}

	// Trim to maxWeeks (keep most recent)
	if len(window) > maxWeeks {
		window = window[len(window)-maxWeeks:]
	}
	return window
}

// BackfillCalibration retroactively adjusts stored signal confidence using
// Platt scaling fitted on evaluated signals. This corrects the calibration
// gap for bootstrap signals that were created with raw rule-based confidence.
//
// Call this AFTER evaluation has populated outcomes, so Platt fitting has data.
// Safe to call multiple times — only updates signals whose confidence changes
// by more than 1 percentage point.
func BackfillCalibration(ctx context.Context, signalRepo ports.SignalRepository) (int, error) {
	allSignals, err := signalRepo.GetAllSignals(ctx)
	if err != nil {
		return 0, fmt.Errorf("get all signals: %w", err)
	}

	// Group evaluated signals by type for Platt fitting.
	// Exclude EXPIRED — they have no real outcome and would bias calibration
	// (all EXPIRED would be counted as losses since Outcome1W != OutcomeWin).
	byType := make(map[string][]domain.PersistedSignal)
	for _, s := range allSignals {
		if s.Outcome1W != "" && s.Outcome1W != domain.OutcomePending && s.Outcome1W != domain.OutcomeExpired {
			byType[s.SignalType] = append(byType[s.SignalType], s)
		}
	}

	// Fit Platt scaling per signal type (need >= 20 evaluated signals)
	type plattCoeffs struct{ a, b float64 }
	coeffs := make(map[string]*plattCoeffs)
	for sigType, sigs := range byType {
		if len(sigs) < 20 {
			continue
		}
		var confs []float64
		var outcomes []bool
		for _, s := range sigs {
			confs = append(confs, s.Confidence)
			outcomes = append(outcomes, s.Outcome1W == domain.OutcomeWin)
		}
		a, b := mathutil.PlattScaling(confs, outcomes)
		if a != 0 || b != 0 {
			coeffs[sigType] = &plattCoeffs{a: a, b: b}
		}
	}

	if len(coeffs) == 0 {
		return 0, nil // no types with enough data for Platt
	}

	// Update all signals with calibrated confidence
	updated := 0
	for _, s := range allSignals {
		pc, ok := coeffs[s.SignalType]
		if !ok {
			continue
		}
		calibrated := mathutil.PlattCalibrate(s.Confidence, pc.a, pc.b)
		// Only update if meaningful change (> 1pp)
		diff := calibrated - s.Confidence
		if diff > 1 || diff < -1 {
			s.RawConfidence = s.Confidence
			s.Confidence = calibrated
			if err := signalRepo.UpdateSignal(ctx, s); err != nil {
				continue // skip on error, non-fatal
			}
			updated++
		}
	}

	return updated, nil
}
