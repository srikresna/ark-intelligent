package backtest

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("backtest")

// Evaluator fills in outcome fields on persisted signals by looking up
// future prices at +1W, +2W, and +4W from the signal's report date.
type Evaluator struct {
	signalRepo ports.SignalRepository
	priceRepo  ports.PriceRepository
}

// NewEvaluator creates a new signal outcome evaluator.
func NewEvaluator(signalRepo ports.SignalRepository, priceRepo ports.PriceRepository) *Evaluator {
	return &Evaluator{
		signalRepo: signalRepo,
		priceRepo:  priceRepo,
	}
}

// EvaluatePending finds all signals that need outcome evaluation and fills
// in price/return/outcome fields. Returns the number of signals evaluated.
func (e *Evaluator) EvaluatePending(ctx context.Context) (int, error) {
	pending, err := e.signalRepo.GetPendingSignals(ctx)
	if err != nil {
		return 0, fmt.Errorf("get pending signals: %w", err)
	}

	if len(pending) == 0 {
		log.Info().Msg("No pending signals to evaluate")
		return 0, nil
	}

	log.Info().Int("pending", len(pending)).Msg("Evaluating pending signals")

	evaluated := 0
	skippedNoPrice := 0
	for i := range pending {
		updated, err := e.evaluateSignal(ctx, &pending[i])
		if err != nil {
			log.Warn().Err(err).
				Str("contract", pending[i].ContractCode).
				Str("type", pending[i].SignalType).
				Msg("Failed to evaluate signal")
			continue
		}
		if !updated {
			skippedNoPrice++
			continue
		}

		if err := e.signalRepo.UpdateSignal(ctx, pending[i]); err != nil {
			log.Warn().Err(err).Msg("Failed to update evaluated signal")
			continue
		}
		evaluated++
	}

	log.Info().
		Int("evaluated", evaluated).
		Int("pending", len(pending)).
		Int("skipped_no_price", skippedNoPrice).
		Int("total_scanned", len(pending)).
		Msg("Signal evaluation complete")

	if evaluated == 0 && len(pending) > 0 {
		sample := pending[0]
		log.Debug().
			Str("contract", sample.ContractCode).
			Time("report_date", sample.ReportDate).
			Float64("entry_price", sample.EntryPrice).
			Str("outcome_1w", sample.Outcome1W).
			Dur("age", time.Since(sample.ReportDate)).
			Msg("sample pending signal (0 evaluated — possible price data gap)")
	}

	return evaluated, nil
}

// evaluateSignal looks up future prices and fills outcome fields.
// Returns true if any field was updated. Evaluates each horizon independently
// so that a transient price lookup failure on one horizon doesn't discard
// successful evaluations on other horizons.
func (e *Evaluator) evaluateSignal(ctx context.Context, sig *domain.PersistedSignal) (bool, error) {
	if sig.EntryPrice == 0 {
		log.Debug().
			Str("contract", sig.ContractCode).
			Time("report_date", sig.ReportDate).
			Msg("Skipping signal with zero entry price")
		return false, nil // Cannot evaluate without entry price
	}

	now := time.Now()
	updated := false

	// Evaluate 1-week outcome
	if (sig.Outcome1W == "" || sig.Outcome1W == domain.OutcomePending) &&
		now.Sub(sig.ReportDate) >= 7*24*time.Hour {
		targetDate := sig.ReportDate.AddDate(0, 0, 7)
		price, err := e.priceRepo.GetPriceAt(ctx, sig.ContractCode, targetDate)
		if err != nil {
			log.Warn().Err(err).Str("contract", sig.ContractCode).Msg("price lookup failed at +1W")
		} else if price != nil && price.Close > 0 {
			sig.Price1W = price.Close
			sig.Return1W = computeReturn(sig.EntryPrice, price.Close, sig.Inverse)
			sig.Outcome1W = classifyOutcome(sig.Direction, sig.Return1W)
			updated = true
		} else {
			sig.Outcome1W = domain.OutcomePending
			log.Warn().
				Str("contract", sig.ContractCode).
				Time("target", targetDate).
				Msg("no price record found at +1W")
		}
	}

	// Evaluate 2-week outcome
	if (sig.Outcome2W == "" || sig.Outcome2W == domain.OutcomePending) &&
		now.Sub(sig.ReportDate) >= 14*24*time.Hour {
		targetDate := sig.ReportDate.AddDate(0, 0, 14)
		price, err := e.priceRepo.GetPriceAt(ctx, sig.ContractCode, targetDate)
		if err != nil {
			log.Warn().Err(err).Str("contract", sig.ContractCode).Msg("price lookup failed at +2W")
		} else if price != nil && price.Close > 0 {
			sig.Price2W = price.Close
			sig.Return2W = computeReturn(sig.EntryPrice, price.Close, sig.Inverse)
			sig.Outcome2W = classifyOutcome(sig.Direction, sig.Return2W)
			updated = true
		} else {
			sig.Outcome2W = domain.OutcomePending
			log.Warn().
				Str("contract", sig.ContractCode).
				Time("target", targetDate).
				Msg("no price record found at +2W")
		}
	}

	// Evaluate 4-week outcome
	if (sig.Outcome4W == "" || sig.Outcome4W == domain.OutcomePending) &&
		now.Sub(sig.ReportDate) >= 28*24*time.Hour {
		targetDate := sig.ReportDate.AddDate(0, 0, 28)
		price, err := e.priceRepo.GetPriceAt(ctx, sig.ContractCode, targetDate)
		if err != nil {
			log.Warn().Err(err).Str("contract", sig.ContractCode).Msg("price lookup failed at +4W")
		} else if price != nil && price.Close > 0 {
			sig.Price4W = price.Close
			sig.Return4W = computeReturn(sig.EntryPrice, price.Close, sig.Inverse)
			sig.Outcome4W = classifyOutcome(sig.Direction, sig.Return4W)
			updated = true
		} else {
			sig.Outcome4W = domain.OutcomePending
			log.Warn().
				Str("contract", sig.ContractCode).
				Time("target", targetDate).
				Msg("no price record found at +4W")
		}
	}

	if updated {
		sig.EvaluatedAt = now
	}

	return updated, nil
}

// computeReturn calculates the percentage return from entry to exit price.
// For inverse pairs (USD/JPY, USD/CHF, USD/CAD, DXY), a price increase
// means the base currency (USD) strengthened, which is bearish for the
// foreign currency — so the return is negated.
func computeReturn(entryPrice, exitPrice float64, inverse bool) float64 {
	if entryPrice == 0 {
		return 0
	}
	ret := ((exitPrice - entryPrice) / entryPrice) * 100
	if inverse {
		ret = -ret
	}
	// Round to 4 decimal places
	return math.Round(ret*10000) / 10000
}

// classifyOutcome determines WIN or LOSS based on direction and return.
// A BULLISH signal wins if return > 0, BEARISH wins if return < 0.
// A return of exactly 0.0 is treated as LOSS — the signal produced no movement
// and offered no value. This avoids false-positive wins on illiquid/holiday weeks
// where price data shows no change.
func classifyOutcome(direction string, returnPct float64) string {
	switch direction {
	case "BULLISH":
		if returnPct > 0 {
			return domain.OutcomeWin
		}
		return domain.OutcomeLoss // includes returnPct == 0: no movement = no edge
	case "BEARISH":
		if returnPct < 0 {
			return domain.OutcomeWin
		}
		return domain.OutcomeLoss // includes returnPct == 0: no movement = no edge
	default:
		return domain.OutcomePending
	}
}
