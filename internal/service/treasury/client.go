package treasury

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("treasury") //nolint:gochecknoglobals

const (
	baseURL     = "https://www.treasurydirect.gov/TA_WS/securities/search"
	cacheTTL    = 12 * time.Hour
	httpTimeout = 20 * time.Second
	pageSize    = 20 // number of recent auctions to fetch per type
)

// securityTypes lists the Treasury security types we track.
var securityTypes = []string{"Note", "Bond", "Bill", "TIPS"} //nolint:gochecknoglobals

// cache fields (package-level, protected by cacheMu).
var (
	globalCache *TreasuryData //nolint:gochecknoglobals
	cacheMu     sync.RWMutex
	httpClient  = httpclient.New(httpclient.WithTimeout(httpTimeout)) //nolint:gochecknoglobals
)

// GetCachedOrFetch returns cached Treasury data if within TTL, otherwise fetches
// fresh data from the TreasuryDirect API.
func GetCachedOrFetch(ctx context.Context) (*TreasuryData, error) {
	cacheMu.RLock()
	if globalCache != nil && time.Since(globalCache.FetchedAt) < cacheTTL {
		data := globalCache
		cacheMu.RUnlock()
		return data, nil
	}
	cacheMu.RUnlock()

	data, err := FetchTreasuryData(ctx)
	if err != nil {
		// Graceful degradation: return stale cache if available.
		cacheMu.RLock()
		stale := globalCache
		cacheMu.RUnlock()
		if stale != nil {
			log.Warn().Err(err).Msg("Treasury fetch failed; using stale cache")
			return stale, nil
		}
		return nil, fmt.Errorf("treasury fetch failed: %w", err)
	}

	cacheMu.Lock()
	globalCache = data
	cacheMu.Unlock()

	return data, nil
}

// CacheAge returns seconds since last fetch, or -1 if no cache.
func CacheAge() float64 {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if globalCache == nil {
		return -1
	}
	return time.Since(globalCache.FetchedAt).Seconds()
}

// FetchTreasuryData fetches recent auction results for all tracked security types.
func FetchTreasuryData(ctx context.Context) (*TreasuryData, error) {
	var allAuctions []ParsedAuction

	for _, secType := range securityTypes {
		auctions, err := fetchByType(ctx, secType)
		if err != nil {
			log.Warn().Err(err).Str("type", secType).Msg("failed to fetch auctions")
			continue // graceful — skip this type
		}
		allAuctions = append(allAuctions, auctions...)
	}

	if len(allAuctions) == 0 {
		return nil, fmt.Errorf("no auction data retrieved from any security type")
	}

	// Sort by date descending (newest first).
	sort.Slice(allAuctions, func(i, j int) bool {
		return allAuctions[i].AuctionDate.After(allAuctions[j].AuctionDate)
	})

	analyses := analyzeAuctions(allAuctions)

	return &TreasuryData{
		Auctions:  allAuctions,
		Analyses:  analyses,
		FetchedAt: time.Now(),
	}, nil
}

// fetchByType fetches auctions for a single security type.
func fetchByType(ctx context.Context, secType string) ([]ParsedAuction, error) {
	url := fmt.Sprintf("%s?type=%s&pagesize=%d&format=json", baseURL, secType, pageSize)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed for %s: %w", secType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, secType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed for %s: %w", secType, err)
	}

	var raw []AuctionResult
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("JSON parse failed for %s: %w", secType, err)
	}

	var parsed []ParsedAuction
	for _, r := range raw {
		p := parseAuction(r)
		if p.AuctionDate.IsZero() {
			continue
		}
		parsed = append(parsed, p)
	}

	return parsed, nil
}

// parseAuction converts a raw API result to a ParsedAuction.
func parseAuction(r AuctionResult) ParsedAuction {
	p := ParsedAuction{
		SecurityType: r.SecurityType,
		SecurityTerm: r.SecurityTerm,
	}

	// Parse auction date (MM/DD/YYYY format).
	if t, err := time.Parse("01/02/2006", r.AuctionDateStr); err == nil {
		p.AuctionDate = t
	}

	p.HighYield = parseFloat(r.HighYield)
	p.BidToCover = parseFloat(r.BidToCoverRatio)
	p.DirectPct = parseFloat(r.DirectBidder)
	p.IndirectPct = parseFloat(r.IndirectBidder)
	p.AllottedAmt = parseFloat(r.AllottedAmt)
	p.OfferingAmt = parseFloat(r.OfferingAmt)

	return p
}

// parseFloat safely converts a string to float64. Returns 0 on failure.
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Remove commas from numbers like "1,000,000".
	s = strings.ReplaceAll(s, ",", "")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
