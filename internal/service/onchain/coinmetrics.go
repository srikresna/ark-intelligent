package onchain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/rs/zerolog/log"
)

const (
	baseURL     = "https://community-api.coinmetrics.io/v4/timeseries/asset-metrics"
	httpTimeout = 20 * time.Second
	cacheTTL    = 6 * time.Hour
)

// tracked assets.
var trackedAssets = []string{"btc", "eth"}

// metrics to fetch from CoinMetrics.
var metricsToFetch = []string{"FlowInExNtv", "FlowOutExNtv", "AdrActCnt", "TxCnt"}

// coinMetricsResponse represents the API response structure.
type coinMetricsResponse struct {
	Data []coinMetricsDataPoint `json:"data"`
}

type coinMetricsDataPoint struct {
	Asset        string `json:"asset"`
	Time         string `json:"time"`
	FlowInExNtv  string `json:"FlowInExNtv"`
	FlowOutExNtv string `json:"FlowOutExNtv"`
	AdrActCnt    string `json:"AdrActCnt"`
	TxCnt        string `json:"TxCnt"`
}

// package-level cache.
var (
	globalReport *OnChainReport                      //nolint:gochecknoglobals
	cacheMu      sync.RWMutex                        //nolint:gochecknoglobals
	client       = httpclient.NewClient(httpTimeout) //nolint:gochecknoglobals
)

// GetCachedOrFetch returns cached on-chain data if within TTL, otherwise
// fetches fresh data from CoinMetrics. Gracefully degrades with stale cache.
func GetCachedOrFetch(ctx context.Context) *OnChainReport {
	cacheMu.RLock()
	if globalReport != nil && time.Since(globalReport.FetchedAt) < cacheTTL {
		rpt := globalReport
		cacheMu.RUnlock()
		return rpt
	}
	cacheMu.RUnlock()

	fresh := fetchAllAssets(ctx)

	cacheMu.Lock()
	if fresh.Available {
		globalReport = fresh
	} else if globalReport != nil {
		stale := globalReport
		cacheMu.Unlock()
		log.Warn().Msg("onchain/coinmetrics: fetch failed, returning stale cache")
		return stale
	}
	cacheMu.Unlock()

	return fresh
}

func fetchAllAssets(ctx context.Context) *OnChainReport {
	report := &OnChainReport{
		Assets:    make(map[string]*AssetOnChainSummary),
		FetchedAt: time.Now(),
	}

	assets := strings.Join(trackedAssets, ",")
	metrics := strings.Join(metricsToFetch, ",")
	url := fmt.Sprintf("%s?assets=%s&metrics=%s&frequency=1d&page_size=60",
		baseURL, assets, metrics)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Error().Err(err).Msg("onchain/coinmetrics: failed to create request")
		return report
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("onchain/coinmetrics: request failed")
		return report
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Msg("onchain/coinmetrics: non-200 response")
		return report
	}

	var apiResp coinMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		log.Error().Err(err).Msg("onchain/coinmetrics: failed to decode response")
		return report
	}

	// Group data by asset.
	assetData := make(map[string][]coinMetricsDataPoint)
	for _, dp := range apiResp.Data {
		assetData[dp.Asset] = append(assetData[dp.Asset], dp)
	}

	for _, asset := range trackedAssets {
		points, ok := assetData[asset]
		if !ok || len(points) == 0 {
			continue
		}
		summary := analyzeAsset(asset, points)
		report.Assets[asset] = summary
	}

	report.Available = len(report.Assets) > 0
	return report
}
