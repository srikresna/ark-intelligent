package news

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/adapter/storage"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

var bootstrapLog = logger.Component("impact-bootstrap")

// ImpactBootstrapper backfills calendar impact data from MQL5 historical
// events combined with stored weekly price data.  It scrapes up to N months
// of history (configurable, default 12), filters for HIGH-impact releases
// that already have an Actual value, computes SurpriseScore from Actual vs
// Forecast (normalized by per-event stddev), and records weekly price impact.
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
//
// Three-phase approach:
//  1. Scrape: fetch all months from MQL5, collect HIGH-impact events with actuals
//  2. Compute: calculate per-event surprise stddev, then normalize each event's sigma
//  3. Record: match events with weekly price data and save impact records
func (ib *ImpactBootstrapper) Bootstrap(ctx context.Context) (int, error) {
	if ib.fetcher == nil || ib.priceRepo == nil || ib.impactRepo == nil {
		return 0, nil
	}

	bootstrapLog.Info().Int("months", ib.months).Msg("starting impact bootstrap")

	// --- Phase 1: Scrape all months, collect candidate events ---
	candidates := ib.scrapeAllMonths(ctx)
	if len(candidates) == 0 {
		bootstrapLog.Info().Msg("no candidate events found, nothing to bootstrap")
		return 0, nil
	}

	bootstrapLog.Info().Int("candidates", len(candidates)).Msg("candidate events collected")

	// --- Phase 2: Compute SurpriseScore for each event ---
	ib.computeSurpriseScores(candidates)

	// --- Phase 3: Pre-fetch prices, then process events ---
	priceWeeks := ib.months*5 + 4
	ib.ensurePriceHistory(ctx, priceWeeks)

	var totalCreated int
	for i := range candidates {
		if ctx.Err() != nil {
			bootstrapLog.Warn().Int("created_so_far", totalCreated).Msg("bootstrap cancelled")
			break
		}
		n, err := ib.processEvent(ctx, &candidates[i])
		if err != nil {
			bootstrapLog.Warn().Str("event", candidates[i].ev.Event).Err(err).Msg("failed to process event")
			continue
		}
		totalCreated += n
	}

	bootstrapLog.Info().Int("impacts_created", totalCreated).Msg("impact bootstrap complete")
	return totalCreated, nil
}

// bootstrapCandidate pairs a scraped event with its computed surprise sigma.
type bootstrapCandidate struct {
	ev            domain.NewsEvent
	actualVal     float64
	forecastVal   float64
	parsedOK      bool
	surpriseSigma float64
}

// scrapeAllMonths fetches events from MQL5 for each month and returns
// filtered HIGH-impact candidates with parseable Actual/Forecast values.
func (ib *ImpactBootstrapper) scrapeAllMonths(ctx context.Context) []bootstrapCandidate {
	var candidates []bootstrapCandidate
	now := time.Now().In(wibLocation)

	for i := 0; i < ib.months; i++ {
		if ctx.Err() != nil {
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
			sleepCtx(ctx, 2*time.Second)
			continue
		}

		for _, ev := range events {
			if ev.Impact != "high" || ev.Actual == "" {
				continue
			}
			if domain.FindPriceMappingByCurrency(ev.Currency) == nil {
				continue
			}

			actualVal, okA := ParseNumericValue(ev.Actual)
			forecastVal, okF := ParseNumericValue(ev.Forecast)

			candidates = append(candidates, bootstrapCandidate{
				ev:          ev,
				actualVal:   actualVal,
				forecastVal: forecastVal,
				parsedOK:    okA && okF,
			})
		}

		sleepCtx(ctx, 1*time.Second)
	}

	// Sort oldest-first so the surprise history accumulation is chronological.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ev.TimeWIB.Before(candidates[j].ev.TimeWIB)
	})

	return candidates
}

// computeSurpriseScores calculates normalized SurpriseScore for each candidate.
// Groups events by name, computes raw diffs, then normalizes by per-event stddev.
func (ib *ImpactBootstrapper) computeSurpriseScores(candidates []bootstrapCandidate) {
	// First pass: collect all raw surprise diffs per event name.
	type eventKey string
	rawDiffs := make(map[eventKey][]float64)

	for i := range candidates {
		c := &candidates[i]
		if !c.parsedOK {
			continue
		}
		raw := c.actualVal - c.forecastVal
		// Adjust sign based on ImpactDirection (same logic as ComputeSurpriseWithDirection).
		if c.ev.ImpactDirection == 2 && raw > 0 {
			raw = -raw
		} else if c.ev.ImpactDirection == 1 && raw < 0 {
			raw = -raw
		}
		key := eventKey(c.ev.Event)
		rawDiffs[key] = append(rawDiffs[key], raw)
	}

	// Compute stddev per event name.
	stddevs := make(map[eventKey]float64)
	for key, diffs := range rawDiffs {
		if len(diffs) >= 3 {
			stddevs[key] = mathutil.StdDevSample(diffs)
		}
	}

	// Second pass: normalize each candidate's surprise.
	for i := range candidates {
		c := &candidates[i]
		if !c.parsedOK {
			// Will be skipped by processEvent; leave sigma at zero.
			continue
		}

		raw := c.actualVal - c.forecastVal
		if c.ev.ImpactDirection == 2 && raw > 0 {
			raw = -raw
		} else if c.ev.ImpactDirection == 1 && raw < 0 {
			raw = -raw
		}

		key := eventKey(c.ev.Event)
		sd := stddevs[key]
		if sd > 0 {
			c.surpriseSigma = raw / sd
		} else {
			c.surpriseSigma = raw // no normalization possible
		}
	}

	// Log distribution stats.
	var withSigma, withoutSigma int
	for _, c := range candidates {
		if c.parsedOK {
			withSigma++
		} else {
			withoutSigma++
		}
	}
	bootstrapLog.Info().
		Int("with_sigma", withSigma).
		Int("without_sigma", withoutSigma).
		Int("unique_events", len(rawDiffs)).
		Msg("surprise scores computed")
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

		existing, err := ib.priceRepo.GetHistory(ctx, mapping.ContractCode, weeks)
		if err == nil && len(existing) >= weeks/2 {
			continue
		}

		bootstrapLog.Debug().
			Str("contract", mapping.ContractCode).
			Str("currency", mapping.Currency).
			Int("weeks", weeks).
			Msg("fetching historical prices for backfill")

		fetched, fetchErr := ib.priceFetcher.FetchWeekly(ctx, mapping, weeks)
		if fetchErr != nil {
			bootstrapLog.Warn().Err(fetchErr).Str("contract", mapping.ContractCode).Msg("price fetch failed")
			continue
		}
		if len(fetched) > 0 {
			if saveErr := ib.priceRepo.SavePrices(ctx, fetched); saveErr != nil {
				bootstrapLog.Warn().Err(saveErr).Str("contract", mapping.ContractCode).Msg("failed to persist prices")
			}
		}

		sleepCtx(ctx, 500*time.Millisecond)
	}
}

// processEvent matches a candidate with weekly price data and saves the impact record.
func (ib *ImpactBootstrapper) processEvent(ctx context.Context, c *bootstrapCandidate) (int, error) {
	// Skip events where surprise could not be computed (unparseable Actual/Forecast).
	// Recording these with sigma=0 would pollute the "-1σ to +1σ" bucket with
	// unknown-surprise events, misleading the statistical analysis.
	if !c.parsedOK {
		return 0, nil
	}

	ev := c.ev
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
			return 0, nil
		}
	}

	// Get price history.
	weeksNeeded := int(time.Since(ev.TimeWIB).Hours()/168) + 4
	if weeksNeeded < 12 {
		weeksNeeded = 12
	}

	records, err := ib.priceRepo.GetHistory(ctx, mapping.ContractCode, weeksNeeded)
	if err != nil || len(records) < 2 {
		if ib.priceFetcher != nil {
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

	priceBefore, priceAfter := ib.findSurroundingPrices(records, ev.TimeWIB)
	if priceBefore == nil || priceAfter == nil || priceBefore.Close == 0 {
		return 0, nil
	}

	// Use the pre-computed surprise sigma.
	sigmaBucket := domain.SigmaToBucket(c.surpriseSigma)

	beforePrice := priceBefore.Close
	afterPrice := priceAfter.Close

	pctChange := (afterPrice - beforePrice) / beforePrice * 100
	if mapping.Inverse {
		pctChange = -pctChange
	}

	pipMultiplier := pipMultiplierForCurrency(ev.Currency)
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
		Float64("sigma_val", c.surpriseSigma).
		Float64("pips", impact.PriceChange).
		Msg("backfilled impact record")

	return 1, nil
}

// findSurroundingPrices locates the closest weekly bar before (or on) the
// event date and the closest weekly bar after the event date.
func (ib *ImpactBootstrapper) findSurroundingPrices(records []domain.PriceRecord, eventTime time.Time) (before *domain.PriceRecord, after *domain.PriceRecord) {
	eventDate := eventTime.Truncate(24 * time.Hour)

	for i := range records {
		recDate := records[i].Date.Truncate(24 * time.Hour)
		if recDate.After(eventDate) {
			if after == nil || recDate.Before(after.Date.Truncate(24*time.Hour)) {
				after = &records[i]
			}
		} else {
			if before == nil || recDate.After(before.Date.Truncate(24*time.Hour)) {
				before = &records[i]
			}
		}
	}
	return
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// pipMultiplierForCurrency returns the pip multiplier for a given currency.
// Shared between bootstrap and recorder to ensure consistent computation.
func pipMultiplierForCurrency(currency string) float64 {
	switch currency {
	case "JPY":
		return 100.0
	case "XAU", "XAG", "OIL", "BOND":
		return 1.0
	default:
		return 10000.0
	}
}

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
