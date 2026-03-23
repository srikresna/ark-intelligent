package news

import (
	"context"
	"math"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/adapter/storage"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var bootstrapLog = logger.Component("impact-bootstrap")

// ImpactBootstrapper backfills calendar impact data from MQL5 historical
// events combined with stored weekly price data.  It fetches up to 3 months
// of events (previous + current month), filters for HIGH-impact releases
// that already have an Actual value, and computes a weekly close-to-close
// price change around each event date.
type ImpactBootstrapper struct {
	fetcher    *MQL5Fetcher
	priceRepo  ports.PriceRepository
	impactRepo *storage.ImpactRepo
}

// NewImpactBootstrapper creates a new ImpactBootstrapper.
func NewImpactBootstrapper(fetcher *MQL5Fetcher, priceRepo ports.PriceRepository, impactRepo *storage.ImpactRepo) *ImpactBootstrapper {
	return &ImpactBootstrapper{
		fetcher:    fetcher,
		priceRepo:  priceRepo,
		impactRepo: impactRepo,
	}
}

// Bootstrap fetches historical events and backfills impact records.
// It returns the number of new impact records created.
func (ib *ImpactBootstrapper) Bootstrap(ctx context.Context) (int, error) {
	if ib.fetcher == nil || ib.priceRepo == nil || ib.impactRepo == nil {
		return 0, nil
	}

	// Collect events from previous and current month.
	var allEvents []domain.NewsEvent

	for _, monthType := range []string{"prev", "current"} {
		events, err := ib.fetcher.ScrapeMonth(ctx, monthType)
		if err != nil {
			bootstrapLog.Warn().Str("month", monthType).Err(err).Msg("failed to scrape month, continuing")
			continue
		}
		allEvents = append(allEvents, events...)
	}

	if len(allEvents) == 0 {
		bootstrapLog.Info().Msg("no historical events fetched, nothing to bootstrap")
		return 0, nil
	}

	bootstrapLog.Info().Int("total_events", len(allEvents)).Msg("historical events fetched")

	created := 0
	for _, ev := range allEvents {
		n, err := ib.processEvent(ctx, ev)
		if err != nil {
			bootstrapLog.Warn().Str("event", ev.Event).Str("currency", ev.Currency).Err(err).Msg("failed to process event")
			continue
		}
		created += n
	}

	bootstrapLog.Info().Int("impacts_created", created).Msg("impact bootstrap complete")
	return created, nil
}

// processEvent handles a single event: filters, checks for duplicates,
// looks up prices, and saves the impact record.  Returns 1 if a record
// was created, 0 otherwise.
func (ib *ImpactBootstrapper) processEvent(ctx context.Context, ev domain.NewsEvent) (int, error) {
	// Only HIGH impact events.
	if ev.Impact != "high" {
		return 0, nil
	}

	// Must have an actual value (already released).
	if ev.Actual == "" {
		return 0, nil
	}

	// Find the currency's price mapping.
	mapping := domain.FindPriceMappingByCurrency(ev.Currency)
	if mapping == nil {
		return 0, nil
	}

	// Check for existing impact records to avoid duplicates.
	existing, err := ib.impactRepo.GetEventImpacts(ctx, ev.Event, ev.Currency)
	if err != nil {
		return 0, err
	}
	eventDate := ev.TimeWIB.Truncate(24 * time.Hour)
	for _, imp := range existing {
		if imp.TimeHorizon == "1w" && imp.Timestamp.Truncate(24*time.Hour).Equal(eventDate) {
			// Already have a 1w impact record for this event on this date.
			return 0, nil
		}
	}

	// Get price history — 12 weeks should cover the window around any event
	// in the previous/current month.
	records, err := ib.priceRepo.GetHistory(ctx, mapping.ContractCode, 12)
	if err != nil || len(records) < 2 {
		return 0, nil
	}

	// records are newest-first; find the closest bar before and after the event.
	priceBefore, priceAfter := ib.findSurroundingPrices(records, ev.TimeWIB)
	if priceBefore == nil || priceAfter == nil || priceBefore.Close == 0 {
		return 0, nil
	}

	// Compute sigma bucket from the event's surprise score.
	sigmaBucket := domain.SigmaToBucket(ev.SurpriseScore)

	// Compute price change following the same logic as ImpactRecorder.saveImpactRecord.
	beforePrice := priceBefore.Close
	afterPrice := priceAfter.Close

	pctChange := (afterPrice - beforePrice) / beforePrice * 100
	if mapping.Inverse {
		pctChange = -pctChange
	}

	pipMultiplier := 10000.0
	switch ev.Currency {
	case "JPY":
		pipMultiplier = 100.0
	case "XAU", "OIL", "BOND":
		pipMultiplier = 1.0
	}

	priceChange := (afterPrice - beforePrice) * pipMultiplier
	if mapping.Inverse {
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
		TimeHorizon: "1w",
		Timestamp:   ev.TimeWIB,
	}

	if err := ib.impactRepo.SaveEventImpact(ctx, impact); err != nil {
		return 0, err
	}

	bootstrapLog.Debug().
		Str("event", ev.Event).
		Str("currency", ev.Currency).
		Str("sigma", sigmaBucket).
		Float64("pips", impact.PriceChange).
		Float64("pct", impact.PctChange).
		Msg("backfilled impact record")

	return 1, nil
}

// findSurroundingPrices locates the closest weekly bar before (or on) the
// event date and the closest weekly bar after the event date from newest-first
// price records.
func (ib *ImpactBootstrapper) findSurroundingPrices(records []domain.PriceRecord, eventTime time.Time) (before *domain.PriceRecord, after *domain.PriceRecord) {
	eventDate := eventTime.Truncate(24 * time.Hour)

	for i := range records {
		recDate := records[i].Date.Truncate(24 * time.Hour)
		if recDate.After(eventDate) {
			// This record is after the event — candidate for "after".
			// Keep the closest one (smallest positive delta).
			if after == nil || recDate.Before(after.Date.Truncate(24*time.Hour)) {
				after = &records[i]
			}
		} else {
			// This record is on or before the event — candidate for "before".
			// Keep the closest one (largest date that is <= eventDate).
			if before == nil || recDate.After(before.Date.Truncate(24*time.Hour)) {
				before = &records[i]
			}
		}
	}
	return
}
