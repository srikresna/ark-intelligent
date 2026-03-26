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
// events combined with stored weekly price data.  It scrapes up to N months
// of history (configurable, default 12), filters for HIGH-impact releases
// that already have an Actual value, and computes a weekly close-to-close
// price change around each event date.
//
// When stored price data is insufficient (e.g. fresh bot with empty DB),
// the bootstrapper falls back to fetching historical weekly prices on-demand
// via the optional PriceFetcher.
type ImpactBootstrapper struct {
	fetcher      *MQL5Fetcher
	priceRepo    ports.PriceRepository
	priceFetcher ports.PriceFetcher
	impactRepo   *storage.ImpactRepo
	months       int // how many months to backfill (default 12)
}

// NewImpactBootstrapper creates a new ImpactBootstrapper.
// priceFetcher is optional — if non-nil it is used as a fallback when stored
// price data is insufficient to compute impact for a given contract.
func NewImpactBootstrapper(fetcher *MQL5Fetcher, priceRepo ports.PriceRepository, impactRepo *storage.ImpactRepo, priceFetcher ports.PriceFetcher) *ImpactBootstrapper {
	return &ImpactBootstrapper{
		fetcher:      fetcher,
		priceRepo:    priceRepo,
		priceFetcher: priceFetcher,
		impactRepo:   impactRepo,
		months:       12,
	}
}

// SetMonths configures how many months of history to backfill.
func (ib *ImpactBootstrapper) SetMonths(m int) {
	if m < 1 {
		m = 1
	}
	if m > 24 {
		m = 24
	}
	ib.months = m
}

// Bootstrap fetches historical events and backfills impact records.
// It scrapes MQL5 month by month going back ib.months months,
// then matches each HIGH-impact event with weekly price data to
// compute the price reaction. Returns the number of new impact records created.
func (ib *ImpactBootstrapper) Bootstrap(ctx context.Context) (int, error) {
	if ib.fetcher == nil || ib.priceRepo == nil || ib.impactRepo == nil {
		return 0, nil
	}

	bootstrapLog.Info().Int("months", ib.months).Msg("starting impact bootstrap")

	// Pre-fetch price data for all contracts to cover the full backfill window.
	// Add a few extra weeks as buffer for surrounding-price lookup.
	priceWeeks := ib.months*5 + 4 // ~5 weeks per month + buffer
	ib.ensurePriceHistory(ctx, priceWeeks)

	// Scrape events month by month, newest first.
	var totalCreated int
	now := time.Now().In(wibLocation)

	for i := 0; i < ib.months; i++ {
		if ctx.Err() != nil {
			bootstrapLog.Warn().Int("created_so_far", totalCreated).Msg("bootstrap cancelled")
			break
		}

		targetMonth := now.AddDate(0, -i, 0)
		from, to := monthBounds(targetMonth)

		bootstrapLog.Info().
			Int("month_offset", i).
			Str("from", from).
			Str("to", to).
			Msg("scraping month for impact backfill")

		events, err := ib.fetcher.ScrapeRange(ctx, "1", from, to)
		if err != nil {
			bootstrapLog.Warn().Err(err).Int("offset", i).Msg("failed to scrape month, continuing")
			// Brief pause before retrying next month to avoid hammering MQL5.
			sleepCtx(ctx, 2*time.Second)
			continue
		}

		bootstrapLog.Debug().Int("events", len(events)).Int("offset", i).Msg("month scraped")

		for _, ev := range events {
			n, procErr := ib.processEvent(ctx, ev)
			if procErr != nil {
				bootstrapLog.Warn().Str("event", ev.Event).Str("currency", ev.Currency).Err(procErr).Msg("failed to process event")
				continue
			}
			totalCreated += n
		}

		// Be respectful to MQL5 — brief pause between months.
		sleepCtx(ctx, 1*time.Second)
	}

	bootstrapLog.Info().Int("impacts_created", totalCreated).Msg("impact bootstrap complete")
	return totalCreated, nil
}

// ensurePriceHistory pre-fetches weekly price data for all contracts
// so that processEvent doesn't need to fetch on-demand for each event.
func (ib *ImpactBootstrapper) ensurePriceHistory(ctx context.Context, weeks int) {
	if ib.priceFetcher == nil {
		return
	}

	for _, mapping := range domain.DefaultPriceSymbolMappings {
		if ctx.Err() != nil {
			return
		}

		// Check if we already have enough data.
		existing, err := ib.priceRepo.GetHistory(ctx, mapping.ContractCode, weeks)
		if err == nil && len(existing) >= weeks/2 {
			continue // sufficient data already stored
		}

		bootstrapLog.Debug().
			Str("contract", mapping.ContractCode).
			Str("currency", mapping.Currency).
			Int("weeks", weeks).
			Msg("fetching historical prices for backfill")

		fetched, fetchErr := ib.priceFetcher.FetchWeekly(ctx, mapping, weeks)
		if fetchErr != nil {
			bootstrapLog.Warn().Err(fetchErr).Str("contract", mapping.ContractCode).Msg("price fetch failed, will try on-demand")
			continue
		}
		if len(fetched) > 0 {
			if saveErr := ib.priceRepo.SavePrices(ctx, fetched); saveErr != nil {
				bootstrapLog.Warn().Err(saveErr).Str("contract", mapping.ContractCode).Msg("failed to persist prices")
			}
		}

		// Brief pause between price API calls.
		sleepCtx(ctx, 500*time.Millisecond)
	}
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

	// Get price history — use enough weeks to cover the event's date.
	weeksNeeded := int(time.Since(ev.TimeWIB).Hours()/168) + 4 // weeks since event + buffer
	if weeksNeeded < 12 {
		weeksNeeded = 12
	}

	records, err := ib.priceRepo.GetHistory(ctx, mapping.ContractCode, weeksNeeded)
	if err != nil || len(records) < 2 {
		// Fallback: fetch historical prices on-demand if repo is empty.
		if ib.priceFetcher != nil {
			bootstrapLog.Debug().
				Str("contract", mapping.ContractCode).
				Str("currency", mapping.Currency).
				Msg("stored price data insufficient, fetching on-demand")
			fetched, fetchErr := ib.priceFetcher.FetchWeekly(ctx, *mapping, weeksNeeded)
			if fetchErr != nil || len(fetched) < 2 {
				return 0, nil
			}
			if saveErr := ib.priceRepo.SavePrices(ctx, fetched); saveErr != nil {
				bootstrapLog.Warn().Err(saveErr).Str("contract", mapping.ContractCode).Msg("failed to persist on-demand prices")
			}
			records = fetched
		} else {
			return 0, nil
		}
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// monthBounds returns the first and last second of a given month in WIB,
// formatted as UTC ISO strings for ScrapeRange.
func monthBounds(t time.Time) (from, to string) {
	y, m, _ := t.In(wibLocation).Date()
	firstDay := time.Date(y, m, 1, 0, 0, 0, 0, wibLocation)
	lastDay := firstDay.AddDate(0, 1, -1)
	endOfDay := time.Date(lastDay.Year(), lastDay.Month(), lastDay.Day(), 23, 59, 59, 0, wibLocation)

	from = firstDay.UTC().Format("2006-01-02T15:04:05")
	to = endOfDay.UTC().Format("2006-01-02T15:04:05")
	return
}

// sleepCtx sleeps for the given duration, aborting early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
