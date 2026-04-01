package cryptocompare

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/rs/zerolog/log"
)

const (
	baseURL     = "https://min-api.cryptocompare.com"
	httpTimeout = 15 * time.Second
	cacheTTL    = 4 * time.Hour
)

// TrackedExchanges lists the top exchanges to monitor.
var TrackedExchanges = []string{"Binance", "Coinbase", "OKX", "Bybit"}

// package-level cache.
var (
	globalCache *VolumeSummary //nolint:gochecknoglobals
	cacheMu     sync.RWMutex  //nolint:gochecknoglobals
	client      = httpclient.NewClient(httpTimeout) //nolint:gochecknoglobals
)

// GetCachedOrFetch returns cached data if within TTL, otherwise fetches fresh.
// Gracefully degrades: returns stale cache on failure.
func GetCachedOrFetch(ctx context.Context) *VolumeSummary {
	cacheMu.RLock()
	if globalCache != nil && time.Since(globalCache.FetchedAt) < cacheTTL {
		data := globalCache
		cacheMu.RUnlock()
		return data
	}
	cacheMu.RUnlock()

	fresh := fetchAll(ctx)

	cacheMu.Lock()
	if fresh.Available {
		globalCache = fresh
	} else if globalCache != nil {
		stale := globalCache
		cacheMu.Unlock()
		log.Warn().Msg("cryptocompare: fetch failed, returning stale cache")
		return stale
	}
	cacheMu.Unlock()

	return fresh
}

// fetchAll gathers exchange volumes and top assets concurrently.
func fetchAll(ctx context.Context) *VolumeSummary {
	result := &VolumeSummary{FetchedAt: time.Now()}

	type exResult struct {
		exchange string
		vol      *ExchangeVolume
	}

	var wg sync.WaitGroup
	exCh := make(chan exResult, len(TrackedExchanges))

	// Fetch exchange volumes concurrently
	for _, ex := range TrackedExchanges {
		wg.Add(1)
		go func(exchange string) {
			defer wg.Done()
			ev := fetchExchangeVolume(ctx, exchange)
			if ev != nil {
				exCh <- exResult{exchange: exchange, vol: ev}
			}
		}(ex)
	}

	// Fetch top assets concurrently
	var topAssets []TopAsset
	wg.Add(1)
	go func() {
		defer wg.Done()
		topAssets = fetchTopAssets(ctx, 20)
	}()

	wg.Wait()
	close(exCh)

	// Collect exchange results
	var totalVol float64
	for er := range exCh {
		result.Exchanges = append(result.Exchanges, *er.vol)
		totalVol += er.vol.VolumeUSD
	}

	if len(result.Exchanges) == 0 {
		log.Warn().Msg("cryptocompare: no exchange data retrieved")
		return result
	}

	// Sort by volume descending
	sort.Slice(result.Exchanges, func(i, j int) bool {
		return result.Exchanges[i].VolumeUSD > result.Exchanges[j].VolumeUSD
	})

	// Compute market share
	result.TotalUSD = totalVol
	if totalVol > 0 {
		for i := range result.Exchanges {
			result.Exchanges[i].Share = result.Exchanges[i].VolumeUSD / totalVol * 100
		}
	}

	result.TopAssets = topAssets
	result.Available = true

	// Compute divergence
	computeDivergence(result)

	log.Info().
		Int("exchanges", len(result.Exchanges)).
		Int("top_assets", len(result.TopAssets)).
		Float64("total_vol_b", totalVol/1e9).
		Str("divergence", result.Divergence).
		Msg("cryptocompare: data fetched")

	return result
}

// fetchExchangeVolume fetches 30 days of daily volume for one exchange.
func fetchExchangeVolume(ctx context.Context, exchange string) *ExchangeVolume {
	url := fmt.Sprintf("%s/data/exchange/histoday?tsym=USD&e=%s&limit=30", baseURL, exchange)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Warn().Err(err).Str("exchange", exchange).Msg("cryptocompare: request creation failed")
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).Str("exchange", exchange).Msg("cryptocompare: fetch failed")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Warn().Int("status", resp.StatusCode).Str("exchange", exchange).Msg("cryptocompare: non-200 response")
		return nil
	}

	var apiResp apiExchangeHistoResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		log.Warn().Err(err).Str("exchange", exchange).Msg("cryptocompare: decode failed")
		return nil
	}

	if apiResp.Response != "Success" || len(apiResp.Data) < 2 {
		log.Warn().Str("exchange", exchange).Str("msg", apiResp.Message).Msg("cryptocompare: API error or no data")
		return nil
	}

	data := apiResp.Data
	latest := data[len(data)-1]

	ev := &ExchangeVolume{
		Exchange:  exchange,
		VolumeUSD: latest.Volume,
		FetchedAt: time.Now(),
	}

	// 24h change (last vs second-to-last)
	if len(data) >= 2 {
		prev := data[len(data)-2]
		if prev.Volume > 0 {
			ev.Change24H = (latest.Volume - prev.Volume) / prev.Volume * 100
		}
	}

	// 7d change
	if len(data) >= 8 {
		p7 := data[len(data)-8]
		if p7.Volume > 0 {
			ev.Change7D = (latest.Volume - p7.Volume) / p7.Volume * 100
		}
	}

	return ev
}

// fetchTopAssets fetches top N assets by total volume.
func fetchTopAssets(ctx context.Context, limit int) []TopAsset {
	url := fmt.Sprintf("%s/data/top/totalvolfull?limit=%d&tsym=USD", baseURL, limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Warn().Err(err).Msg("cryptocompare: top assets request failed")
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("cryptocompare: top assets fetch failed")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Warn().Int("status", resp.StatusCode).Msg("cryptocompare: top assets non-200")
		return nil
	}

	var apiResp apiTopVolResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		log.Warn().Err(err).Msg("cryptocompare: top assets decode failed")
		return nil
	}

	if apiResp.Response != "Success" {
		log.Warn().Str("msg", apiResp.Message).Msg("cryptocompare: top assets API error")
		return nil
	}

	var assets []TopAsset
	for _, coin := range apiResp.Data {
		usd, ok := coin.RAW["USD"]
		if !ok {
			continue
		}
		assets = append(assets, TopAsset{
			Symbol:    coin.CoinInfo.Name,
			FullName:  coin.CoinInfo.FullName,
			VolumeUSD: usd.VOLUME24HOURTO,
			Change24H: usd.CHANGEPCT24HOUR,
			Price:     usd.PRICE,
		})
	}

	return assets
}

// computeDivergence detects volume share shifts between exchanges.
func computeDivergence(s *VolumeSummary) {
	s.Divergence = "NONE"
	s.DivDetail = ""

	if len(s.Exchanges) < 2 {
		return
	}

	// Check 7d volume change divergence between exchanges
	maxGain := -math.MaxFloat64
	maxLoss := math.MaxFloat64
	var gainer, loser string

	for _, ex := range s.Exchanges {
		if ex.Change7D > maxGain {
			maxGain = ex.Change7D
			gainer = ex.Exchange
		}
		if ex.Change7D < maxLoss {
			maxLoss = ex.Change7D
			loser = ex.Exchange
		}
	}

	spread := maxGain - maxLoss
	switch {
	case spread > 50:
		s.Divergence = "SIGNIFICANT"
		s.DivDetail = fmt.Sprintf("%s gaining (%+.0f%% 7d) while %s losing (%+.0f%% 7d) — major volume rotation",
			gainer, maxGain, loser, maxLoss)
	case spread > 20:
		s.Divergence = "MODERATE"
		s.DivDetail = fmt.Sprintf("%s gaining (%+.0f%% 7d) vs %s (%+.0f%% 7d) — some volume shift",
			gainer, maxGain, loser, maxLoss)
	}
}

// FormatVolumeUSD formats volume as human-readable string.
func FormatVolumeUSD(vol float64) string {
	switch {
	case vol >= 1e9:
		return fmt.Sprintf("$%.1fB", vol/1e9)
	case vol >= 1e6:
		return fmt.Sprintf("$%.1fM", vol/1e6)
	case vol >= 1e3:
		return fmt.Sprintf("$%.1fK", vol/1e3)
	default:
		return fmt.Sprintf("$%.0f", vol)
	}
}
