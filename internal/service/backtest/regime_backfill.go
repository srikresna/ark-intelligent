package backtest

import (
	"context"
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// BackfillRegimeLabels assigns the given FRED regime label to all persisted
// signals that currently have an empty FREDRegime field. This bootstraps the
// macro matrix by approximating historical regimes with the current one —
// not perfect, but far better than leaving them blank (which causes the
// regime performance matrix to show "No data available").
//
// Safe to call multiple times: only touches signals where FREDRegime == "".
// Returns the number of signals updated.
func BackfillRegimeLabels(ctx context.Context, signalRepo ports.SignalRepository, regime string) (int, error) {
	if regime == "" {
		return 0, fmt.Errorf("regime label is empty, nothing to backfill")
	}

	allSignals, err := signalRepo.GetAllSignals(ctx)
	if err != nil {
		return 0, fmt.Errorf("get all signals for regime backfill: %w", err)
	}

	updated := 0
	for _, sig := range allSignals {
		select {
		case <-ctx.Done():
			return updated, ctx.Err()
		default:
		}

		if sig.FREDRegime != "" {
			continue // already labelled
		}

		sig.FREDRegime = regime
		if err := signalRepo.UpdateSignal(ctx, sig); err != nil {
			log.Warn().Err(err).
				Str("contract", sig.ContractCode).
				Str("signal_type", sig.SignalType).
				Msg("Failed to backfill regime label on signal")
			continue
		}
		updated++
	}

	return updated, nil
}
