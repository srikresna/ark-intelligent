package backtest

import (
	"context"
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
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

// Bootstrapper replays historical COT data against historical prices
// to create a retroactive backtest dataset. Safe to run multiple times
// due to key-based deduplication.
type Bootstrapper struct {
	cotRepo    COTHistoryProvider
	priceRepo  ports.PriceRepository
	signalRepo ports.SignalRepository
	sigChecker SignalExistenceChecker
	detector   *cot.SignalDetector
}

// NewBootstrapper creates a new backtest bootstrapper.
func NewBootstrapper(
	cotRepo COTHistoryProvider,
	priceRepo ports.PriceRepository,
	signalRepo ports.SignalRepository,
	sigChecker SignalExistenceChecker,
) *Bootstrapper {
	return &Bootstrapper{
		cotRepo:    cotRepo,
		priceRepo:  priceRepo,
		signalRepo: signalRepo,
		sigChecker: sigChecker,
		detector:   cot.NewSignalDetector(),
	}
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

		// Run signal detection on this single analysis with its history context
		historyMap := map[string][]domain.COTRecord{
			mapping.ContractCode: historyWindow,
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
				ContractCode: mapping.ContractCode,
				Currency:     mapping.Currency,
				SignalType:   string(sig.Type),
				Direction:    sig.Direction,
				Strength:     sig.Strength,
				Confidence:   sig.Confidence,
				Description:  sig.Description,
				ReportDate:   analysis.ReportDate,
				DetectedAt:   analysis.ReportDate, // Retroactive — use report date
				EntryPrice:   entryClose,
				Inverse:      mapping.Inverse,
				COTIndex:     analysis.COTIndex,
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
