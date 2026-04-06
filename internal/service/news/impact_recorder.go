package news

import (
	"context"
	"math"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var impactLog = logger.Component("impact-recorder")

// ImpactStore defines the storage interface used by the impact recorder.
type ImpactStore interface {
	SaveEventImpact(ctx context.Context, impact domain.EventImpact) error
}

// PriceProvider defines the price lookup interface used by the impact recorder.
type PriceProvider interface {
	GetPriceAt(ctx context.Context, contractCode string, date time.Time) (*domain.PriceRecord, error)
	GetLatest(ctx context.Context, contractCode string) (*domain.PriceRecord, error)
}

// SpotPriceFetcher fetches real-time spot prices for impact recording.
type SpotPriceFetcher interface {
	FetchSpotPrice(ctx context.Context, contractCode string) (float64, error)
}

// ImpactRecorder captures price impact after economic event releases.
// It records the price at release time and schedules follow-up checks
// at various horizons (15m, 30m, 1h, 4h) to measure the actual market impact.
type ImpactRecorder struct {
	impactStore ImpactStore
	priceRepo   PriceProvider
	spotFetcher SpotPriceFetcher
}

// NewImpactRecorder creates a new ImpactRecorder.
func NewImpactRecorder(impactStore ImpactStore, priceRepo PriceProvider, spotFetcher SpotPriceFetcher) *ImpactRecorder {
	return &ImpactRecorder{
		impactStore: impactStore,
		priceRepo:   priceRepo,
		spotFetcher: spotFetcher,
	}
}

// RecordImpact records the price impact of an event release.
// It fetches the price at release time and at release+horizon, then
// computes the change in pips and percentage.
// horizons: e.g., []string{"1h", "4h"}
func (r *ImpactRecorder) RecordImpact(ctx context.Context, ev domain.NewsEvent, surpriseSigma float64, horizons []string) {
	if r.impactStore == nil || r.priceRepo == nil {
		return
	}

	// Find the contract code for this currency
	mapping := domain.FindPriceMappingByCurrency(ev.Currency)
	if mapping == nil {
		impactLog.Debug().Str("currency", ev.Currency).Msg("no price mapping for currency, skipping impact recording")
		return
	}

	// Get price at release time — try real-time spot price first
	releaseTime := ev.TimeWIB
	var beforePrice float64
	if r.spotFetcher != nil {
		spotPrice, err := r.spotFetcher.FetchSpotPrice(ctx, mapping.ContractCode)
		if err == nil && spotPrice > 0 {
			beforePrice = spotPrice
		}
	}
	if beforePrice == 0 {
		// Fallback to stored price
		priceBefore, err := r.priceRepo.GetPriceAt(ctx, mapping.ContractCode, releaseTime)
		if err != nil || priceBefore == nil {
			priceBefore, err = r.priceRepo.GetLatest(ctx, mapping.ContractCode)
			if err != nil || priceBefore == nil {
				impactLog.Debug().Str("currency", ev.Currency).Msg("no price data available for impact recording")
				return
			}
		}
		beforePrice = priceBefore.Close
	}

	sigmaBucket := domain.SigmaToBucket(surpriseSigma)

	for _, horizon := range horizons {
		var duration time.Duration
		switch horizon {
		case "15m":
			duration = 15 * time.Minute
		case "30m":
			duration = 30 * time.Minute
		case "1h":
			duration = 1 * time.Hour
		case "4h":
			duration = 4 * time.Hour
		default:
			continue
		}

		afterTime := releaseTime.Add(duration)

		// If the after-time is in the future, schedule a delayed recording
		// Use a background context so it survives the parent request lifecycle.
		if afterTime.After(time.Now()) {
			recordCtx, recordCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer recordCancel()
			go r.delayedRecord(recordCtx, ev, mapping.ContractCode, beforePrice, surpriseSigma, sigmaBucket, horizon, duration)
			continue
		}

		// After-time is in the past — look up price directly
		priceAfter, err := r.priceRepo.GetPriceAt(ctx, mapping.ContractCode, afterTime)
		if err != nil || priceAfter == nil {
			impactLog.Debug().Str("currency", ev.Currency).Str("horizon", horizon).Msg("no after-price for impact recording")
			continue
		}

		r.saveImpactRecord(ctx, ev, beforePrice, priceAfter.Close, sigmaBucket, horizon, releaseTime, mapping.Inverse)
	}
}

// delayedRecord waits for the specified duration then records the impact.
func (r *ImpactRecorder) delayedRecord(
	ctx context.Context,
	ev domain.NewsEvent,
	contractCode string,
	beforePrice float64,
	surpriseSigma float64,
	sigmaBucket string,
	horizon string,
	delay time.Duration,
) {
	// Panic recovery — an unrecovered panic in a goroutine crashes the entire process.
	defer func() {
		if rec := recover(); rec != nil {
			impactLog.Error().Interface("panic", rec).
				Str("event", ev.Event).Str("horizon", horizon).
				Msg("PANIC in delayedRecord goroutine")
		}
	}()

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	// Try real-time spot price first
	if r.spotFetcher != nil {
		spotPrice, spotErr := r.spotFetcher.FetchSpotPrice(ctx, contractCode)
		if spotErr == nil && spotPrice > 0 {
			mapping := domain.FindPriceMappingByCurrency(ev.Currency)
			inverse := mapping != nil && mapping.Inverse
			r.saveImpactRecord(ctx, ev, beforePrice, spotPrice, sigmaBucket, horizon, ev.TimeWIB, inverse)
			return
		}
	}

	// Fallback to stored price
	priceAfter, err := r.priceRepo.GetLatest(ctx, contractCode)
	if err != nil || priceAfter == nil {
		impactLog.Warn().Str("currency", ev.Currency).Str("horizon", horizon).Msg("delayed impact: no price available")
		return
	}

	mapping := domain.FindPriceMappingByCurrency(ev.Currency)
	inverse := mapping != nil && mapping.Inverse

	r.saveImpactRecord(ctx, ev, beforePrice, priceAfter.Close, sigmaBucket, horizon, ev.TimeWIB, inverse)
}

// saveImpactRecord computes pip/pct changes and persists the impact record.
func (r *ImpactRecorder) saveImpactRecord(
	ctx context.Context,
	ev domain.NewsEvent,
	beforePrice, afterPrice float64,
	sigmaBucket, horizon string,
	releaseTime time.Time,
	inverse bool,
) {
	if beforePrice == 0 {
		return
	}

	pctChange := (afterPrice - beforePrice) / beforePrice * 100
	// For inverse pairs (USD/JPY, USD/CHF, USD/CAD, DXY), flip the sign
	// so that positive = good for the quoted currency
	if inverse {
		pctChange = -pctChange
	}

	// Compute pip change using shared multiplier (consistent with bootstrap).
	pipMultiplier := pipMultiplierForCurrency(ev.Currency)

	priceChange := (afterPrice - beforePrice) * pipMultiplier
	if inverse {
		priceChange = -priceChange
	}

	impact := domain.EventImpact{
		EventTitle:  ev.Event,
		Currency:    ev.Currency,
		SigmaLevel:  sigmaBucket,
		PriceBefore: beforePrice,
		PriceAfter:  afterPrice,
		PriceChange: math.Round(priceChange*10) / 10,
		PctChange:   math.Round(pctChange*1000) / 1000,
		TimeHorizon: horizon,
		Timestamp:   releaseTime,
	}

	if err := r.impactStore.SaveEventImpact(ctx, impact); err != nil {
		impactLog.Error().
			Str("event", ev.Event).
			Str("currency", ev.Currency).
			Str("horizon", horizon).
			Err(err).
			Msg("failed to save event impact")
	} else {
		impactLog.Info().
			Str("event", ev.Event).
			Str("currency", ev.Currency).
			Str("horizon", horizon).
			Str("sigma", sigmaBucket).
			Float64("pips", impact.PriceChange).
			Float64("pct", impact.PctChange).
			Msg("event impact recorded")
	}
}
