package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
)

// regimeSnapshot pairs a date with its classified regime name.
type regimeSnapshot struct {
	date   time.Time
	regime string
}

// BackfillRegimeLabels fetches historical FRED data, builds a timeline of
// weekly regime snapshots, and stamps each unlabelled signal with the regime
// that was active at its DetectedAt date.
//
// This replaces the naive approach of stamping ALL signals with the current
// regime — each signal now gets the historically accurate regime.
//
// Safe to call multiple times: only touches signals where FREDRegime == "".
// Returns the number of signals updated.
func BackfillRegimeLabels(ctx context.Context, signalRepo ports.SignalRepository) (int, error) {
	// Fetch ~52 weeks of historical regime snapshots from FRED.
	regimeMap, err := fred.FetchHistoricalRegimes(ctx, 52)
	if err != nil {
		return 0, fmt.Errorf("fetch historical regimes: %w", err)
	}
	if len(regimeMap) == 0 {
		return 0, fmt.Errorf("no historical regime snapshots available")
	}

	// Parse the date keys and sort them for closest-date lookup.
	snapshots := make([]regimeSnapshot, 0, len(regimeMap))
	for dateStr, regime := range regimeMap {
		t, parseErr := time.Parse("2006-01-02", dateStr)
		if parseErr != nil {
			continue
		}
		snapshots = append(snapshots, regimeSnapshot{date: t, regime: regime})
	}
	if len(snapshots) == 0 {
		return 0, fmt.Errorf("failed to parse any regime snapshot dates")
	}

	// Sort ascending by date.
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].date.Before(snapshots[j].date)
	})

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

		// Find the closest regime snapshot to this signal's DetectedAt date.
		regime := findClosestRegime(snapshots, sig.DetectedAt)
		if regime == "" {
			continue
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

// findClosestRegime returns the regime name from the snapshot whose date is
// closest to (but not after) the target, to avoid look-ahead bias.
// Only searches BACKWARD: snapshots on or before the target date.
// Returns "" if no snapshot is within 30 days before the target.
//
// Snapshots must be sorted ascending by date.
func findClosestRegime(snapshots []regimeSnapshot, target time.Time) string {
	bestRegime := ""
	bestDist := time.Duration(math.MaxInt64)

	for _, s := range snapshots {
		if s.date.After(target) {
			// Snapshots are sorted ascending — all remaining are in the future.
			// Stop here to prevent look-ahead bias.
			break
		}
		dist := target.Sub(s.date) // always >= 0 since s.date <= target
		if dist < bestDist {
			bestDist = dist
			bestRegime = s.regime
		}
	}

	// Only match if within 30 days backward from the target.
	if bestDist > 30*24*time.Hour {
		return ""
	}
	return bestRegime
}
