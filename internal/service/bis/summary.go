// BIS Summary — aggregates WS_CBPOL, WS_CREDIT_GAP, and WS_GLI datasets
// into a single BISSummaryData struct. Caches result in BadgerDB for 24h.
package bis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	badger "github.com/dgraph-io/badger/v4"
	"golang.org/x/sync/singleflight"
)

const (
	summaryTTL      = 24 * time.Hour
	summaryCacheKey = "bis:summary:v1:latest"
)

// cbConfig maps BIS REF_AREA codes to human-readable central bank labels.
var cbConfig = []struct { //nolint:gochecknoglobals
	Code  string
	Label string
}{
	{"US", "Fed"},
	{"XM", "ECB"},
	{"GB", "BOE"},
	{"JP", "BOJ"},
	{"CH", "SNB"},
	{"AU", "RBA"},
	{"CA", "BOC"},
	{"NZ", "RBNZ"},
}

// creditGapConfig maps REF_AREA codes to country display names for WS_CREDIT_GAP.
var creditGapConfig = []struct { //nolint:gochecknoglobals
	Code  string
	Label string
}{
	{"US", "United States"},
	{"GB", "United Kingdom"},
	{"JP", "Japan"},
	{"AU", "Australia"},
	{"CA", "Canada"},
	{"NZ", "New Zealand"},
	{"CH", "Switzerland"},
	{"XM", "Eurozone"},
}

// gliConfig maps REF_AREA codes to display labels for WS_GLI.
var gliConfig = []struct { //nolint:gochecknoglobals
	Code  string
	Label string
}{
	{"5J", "USD to Non-US"},
	{"5R", "EUR Credit"},
	{"5A", "All Currencies"},
}

// Package-level cache state.
var (
	summaryCache    *BISSummaryData //nolint:gochecknoglobals
	summaryCacheMu  sync.RWMutex
	summaryCacheExp time.Time
	summaryDB       *badger.DB //nolint:gochecknoglobals
	summaryFG       singleflight.Group
)

// InitBISSummaryCache injects a BadgerDB instance for 24h persistence.
// If db is nil, the cache falls back to pure in-memory mode.
func InitBISSummaryCache(db *badger.DB) {
	summaryCacheMu.Lock()
	summaryDB = db
	summaryCacheMu.Unlock()
	log.Debug().Bool("persistence", db != nil).Msg("BIS summary cache initialized")
}

// GetBISSummary returns cached BISSummaryData or fetches fresh data.
// Load order: in-memory → BadgerDB → live BIS API.
func GetBISSummary(ctx context.Context) (*BISSummaryData, error) {
	// Fast path: in-memory.
	summaryCacheMu.RLock()
	if summaryCache != nil && time.Now().Before(summaryCacheExp) {
		d := summaryCache
		summaryCacheMu.RUnlock()
		return d, nil
	}
	summaryCacheMu.RUnlock()

	v, err, _ := summaryFG.Do("bis-summary", func() (any, error) {
		// Re-check under read lock.
		summaryCacheMu.RLock()
		if summaryCache != nil && time.Now().Before(summaryCacheExp) {
			d := summaryCache
			summaryCacheMu.RUnlock()
			return d, nil
		}
		db := summaryDB
		summaryCacheMu.RUnlock()

		// Try BadgerDB.
		if db != nil {
			if d := loadSummaryFromBadger(db); d != nil {
				log.Debug().Msg("BIS summary: BadgerDB hit")
				summaryCacheMu.Lock()
				summaryCache = d
				summaryCacheExp = d.FetchedAt.Add(summaryTTL)
				summaryCacheMu.Unlock()
				return d, nil
			}
		}

		// Live fetch.
		log.Debug().Msg("BIS summary: cache miss, fetching live data")
		d, err := fetchBISSummary(ctx)
		if err != nil {
			return nil, err
		}

		summaryCacheMu.Lock()
		summaryCache = d
		summaryCacheExp = time.Now().Add(summaryTTL)
		summaryCacheMu.Unlock()

		if db != nil {
			go saveSummaryToBadger(db, d)
		}
		return d, nil
	})

	if err != nil {
		// Graceful degradation: return stale cache if available.
		summaryCacheMu.RLock()
		stale := summaryCache
		summaryCacheMu.RUnlock()
		if stale != nil {
			log.Warn().Err(err).Msg("BIS summary fetch failed; using stale cache")
			return stale, nil
		}
		return nil, err
	}
	return v.(*BISSummaryData), nil
}

// fetchBISSummary fetches all three datasets concurrently.
func fetchBISSummary(ctx context.Context) (*BISSummaryData, error) {
	type result struct {
		rates  []PolicyRate
		gaps   []CreditGap
		gli    []GlobalLiquidity
		err    error
		kind   int // 0=rates, 1=gaps, 2=gli
	}

	ch := make(chan result, 3)

	go func() {
		rates, err := fetchPolicyRates(ctx)
		ch <- result{rates: rates, err: err, kind: 0}
	}()
	go func() {
		gaps, err := fetchCreditGaps(ctx)
		ch <- result{gaps: gaps, err: err, kind: 1}
	}()
	go func() {
		gli, err := fetchGlobalLiquidity(ctx)
		ch <- result{gli: gli, err: err, kind: 2}
	}()

	summary := &BISSummaryData{FetchedAt: time.Now()}
	for i := 0; i < 3; i++ {
		r := <-ch
		if r.err != nil {
			log.Warn().Err(r.err).Int("kind", r.kind).Msg("BIS dataset fetch error (non-fatal)")
		}
		switch r.kind {
		case 0:
			summary.PolicyRates = r.rates
		case 1:
			summary.CreditGaps = r.gaps
		case 2:
			summary.GLIndicators = r.gli
		}
	}

	// Populate fallbacks for missing entries.
	if len(summary.PolicyRates) == 0 {
		summary.PolicyRates = makeFallbackRates()
	}
	if len(summary.CreditGaps) == 0 {
		summary.CreditGaps = makeFallbackGaps()
	}

	return summary, nil
}

// ---------------------------------------------------------------------------
// WS_CBPOL — Central Bank Policy Rates
// ---------------------------------------------------------------------------

func fetchPolicyRates(ctx context.Context) ([]PolicyRate, error) {
	// Build series key: Q.US+Q.XM+Q.GB+... for all configured CBs
	parts := make([]string, 0, len(cbConfig))
	for _, cb := range cbConfig {
		parts = append(parts, "Q."+cb.Code)
	}
	seriesKey := joinPlus(parts)

	body, err := fetchCSV(ctx, "WS_CBPOL", seriesKey, 4)
	if err != nil {
		return nil, fmt.Errorf("WS_CBPOL: %w", err)
	}

	rows, err := ParseBISCSV(body)
	if err != nil {
		return nil, fmt.Errorf("WS_CBPOL parse: %w", err)
	}

	latest := LatestByRefArea(rows)
	rates := make([]PolicyRate, 0, len(cbConfig))
	for _, cb := range cbConfig {
		row, ok := latest[cb.Code]
		if !ok {
			rates = append(rates, PolicyRate{Country: cb.Code, Label: cb.Label})
			continue
		}
		rates = append(rates, PolicyRate{
			Country:   cb.Code,
			Label:     cb.Label,
			Rate:      row.OBSValue,
			Period:    row.TimePeriod,
			Available: true,
		})
	}
	return rates, nil
}

// ---------------------------------------------------------------------------
// WS_CREDIT_GAP — Credit-to-GDP Gaps
// ---------------------------------------------------------------------------

func fetchCreditGaps(ctx context.Context) ([]CreditGap, error) {
	// WS_CREDIT_GAP series key: FREQ.BORROWER_CTY.LEND_TYPE.CREDIT_MEASURE
	// Total credit, all sectors: Q.{country}.P.A (private non-financial sector, all instruments)
	parts := make([]string, 0, len(creditGapConfig))
	for _, cg := range creditGapConfig {
		parts = append(parts, "Q."+cg.Code+".P.A")
	}
	seriesKey := joinPlus(parts)

	body, err := fetchCSV(ctx, "WS_CREDIT_GAP", seriesKey, 8)
	if err != nil {
		return nil, fmt.Errorf("WS_CREDIT_GAP: %w", err)
	}

	rows, err := ParseBISCSV(body)
	if err != nil {
		return nil, fmt.Errorf("WS_CREDIT_GAP parse: %w", err)
	}

	latest := LatestByRefArea(rows)
	gaps := make([]CreditGap, 0, len(creditGapConfig))
	for _, cg := range creditGapConfig {
		row, ok := latest[cg.Code]
		if !ok {
			gaps = append(gaps, CreditGap{Country: cg.Code, Label: cg.Label})
			continue
		}
		gaps = append(gaps, CreditGap{
			Country:   cg.Code,
			Label:     cg.Label,
			Gap:       row.OBSValue,
			Signal:    classifyCreditGap(row.OBSValue),
			Period:    row.TimePeriod,
			Available: true,
		})
	}
	return gaps, nil
}

func classifyCreditGap(gap float64) string {
	switch {
	case gap > 2.0:
		return "WARNING"
	case gap > 0:
		return "ELEVATED"
	default:
		return "NEUTRAL"
	}
}

// ---------------------------------------------------------------------------
// WS_GLI — Global Liquidity Indicators
// ---------------------------------------------------------------------------

func fetchGlobalLiquidity(ctx context.Context) ([]GlobalLiquidity, error) {
	// WS_GLI: Q.{currency_group}.USD.A.I (total USD credit to non-residents)
	// We fetch the aggregated series for USD, EUR, and total.
	// Series key dimensions: FREQ.CURRENCY.DENOM.LEND_TYPE.BORROWER_CTY
	// Total: Q.U.A.A.5J (USD, all, all sectors, all non-US countries)
	// Use "all" and filter to known aggregates.
	body, err := fetchCSV(ctx, "WS_GLI", "all", 4)
	if err != nil {
		return nil, fmt.Errorf("WS_GLI: %w", err)
	}

	rows, err := ParseBISCSV(body)
	if err != nil {
		return nil, fmt.Errorf("WS_GLI parse: %w", err)
	}

	latest := LatestByRefArea(rows)
	var gli []GlobalLiquidity
	for _, g := range gliConfig {
		row, ok := latest[g.Code]
		if !ok {
			continue
		}
		gli = append(gli, GlobalLiquidity{
			Label:     g.Label,
			ValueBn:   row.OBSValue,
			Period:    row.TimePeriod,
			Available: true,
		})
	}
	return gli, nil
}

// ---------------------------------------------------------------------------
// Fallback helpers
// ---------------------------------------------------------------------------

func makeFallbackRates() []PolicyRate {
	r := make([]PolicyRate, len(cbConfig))
	for i, cb := range cbConfig {
		r[i] = PolicyRate{Country: cb.Code, Label: cb.Label}
	}
	return r
}

func makeFallbackGaps() []CreditGap {
	r := make([]CreditGap, len(creditGapConfig))
	for i, cg := range creditGapConfig {
		r[i] = CreditGap{Country: cg.Code, Label: cg.Label}
	}
	return r
}

// SummaryCacheAge returns how old the cached summary is, or -1 if none exists.
func SummaryCacheAge() time.Duration {
	summaryCacheMu.RLock()
	defer summaryCacheMu.RUnlock()
	if summaryCache == nil {
		return -1
	}
	return time.Since(summaryCache.FetchedAt)
}

func joinPlus(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "+"
		}
		result += p
	}
	return result
}

// ---------------------------------------------------------------------------
// BadgerDB helpers
// ---------------------------------------------------------------------------

type summaryBadgerEntry struct {
	Data      *BISSummaryData `json:"data"`
	ExpiresAt time.Time       `json:"expires_at"`
}

func loadSummaryFromBadger(db *badger.DB) *BISSummaryData {
	var entry summaryBadgerEntry
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(summaryCacheKey))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &entry)
		})
	})
	if err != nil {
		if err != badger.ErrKeyNotFound {
			log.Debug().Err(err).Msg("BIS summary: BadgerDB read error (non-fatal)")
		}
		return nil
	}
	if time.Now().After(entry.ExpiresAt) || entry.Data == nil {
		return nil
	}
	return entry.Data
}

func saveSummaryToBadger(db *badger.DB, data *BISSummaryData) {
	entry := summaryBadgerEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(summaryTTL),
	}
	val, err := json.Marshal(&entry)
	if err != nil {
		log.Warn().Err(err).Msg("BIS summary: marshal for BadgerDB failed (skipped)")
		return
	}
	err = db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(summaryCacheKey), val).WithTTL(summaryTTL)
		return txn.SetEntry(e)
	})
	if err != nil {
		log.Warn().Err(err).Msg("BIS summary: BadgerDB save failed (non-fatal)")
		return
	}
	log.Debug().Msg("BIS summary: saved to BadgerDB")
}
