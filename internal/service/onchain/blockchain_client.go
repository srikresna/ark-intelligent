package onchain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/rs/zerolog/log"
)

const (
	blockchainChartsURL = "https://api.blockchain.info/charts"
	blockchainStatsURL  = "https://api.blockchain.info/stats"
	blockchainTimeout   = 15 * time.Second
	blockchainCacheTTL  = 4 * time.Hour
	mempoolCongestedBytes = 100 * 1024 * 1024 // 100 MB
)

var (
	bcGlobalHealth *BTCNetworkHealth  //nolint:gochecknoglobals
	bcCacheMu      sync.RWMutex      //nolint:gochecknoglobals
	bcClient       = httpclient.NewClient(blockchainTimeout) //nolint:gochecknoglobals
)

// GetBTCHealth returns cached BTC network health data, or fetches fresh data
// from Blockchain.com if the cache has expired (TTL: 4h).
func GetBTCHealth(ctx context.Context) *BTCNetworkHealth {
	bcCacheMu.RLock()
	if bcGlobalHealth != nil && time.Since(bcGlobalHealth.FetchedAt) < blockchainCacheTTL {
		h := bcGlobalHealth
		bcCacheMu.RUnlock()
		return h
	}
	bcCacheMu.RUnlock()

	fresh := fetchBTCHealth(ctx)

	bcCacheMu.Lock()
	if fresh.Available {
		bcGlobalHealth = fresh
	} else if bcGlobalHealth != nil {
		stale := bcGlobalHealth
		bcCacheMu.Unlock()
		log.Warn().Msg("onchain/blockchain: fetch failed, returning stale cache")
		return stale
	}
	bcCacheMu.Unlock()

	return fresh
}

func fetchBTCHealth(ctx context.Context) *BTCNetworkHealth {
	health := &BTCNetworkHealth{
		FetchedAt: time.Now(),
	}

	// 1. Fetch aggregate stats.
	stats, err := fetchBlockchainStats(ctx)
	if err != nil {
		log.Error().Err(err).Msg("onchain/blockchain: stats fetch failed")
		return health
	}

	health.MarketPriceUSD = stats.MarketPriceUSD
	health.NTx24H = stats.NTx
	health.Difficulty = stats.Difficulty
	health.MempoolBytes = stats.MempoolSize
	health.MempoolCongested = stats.MempoolSize > mempoolCongestedBytes

	// Hash rate from /stats is in GH/s, convert to TH/s.
	health.HashRate = stats.HashRate / 1000.0

	// 2. Fetch hash-rate chart (30 days) for trend analysis.
	hashChart, err := fetchChart(ctx, "hash-rate", "30days")
	if err != nil {
		log.Warn().Err(err).Msg("onchain/blockchain: hash-rate chart fetch failed")
	} else {
		analyzeHashRate(health, hashChart.Values)
	}

	// 3. Fetch transaction-fees chart (30 days) for spike detection.
	feeChart, err := fetchChart(ctx, "transaction-fees", "30days")
	if err != nil {
		log.Warn().Err(err).Msg("onchain/blockchain: transaction-fees chart fetch failed")
	} else {
		analyzeFees(health, feeChart.Values)
	}

	// 4. Fetch mempool-size chart for count estimate.
	mempoolChart, err := fetchChart(ctx, "mempool-count", "2days")
	if err != nil {
		log.Debug().Err(err).Msg("onchain/blockchain: mempool-count chart fetch failed")
	} else if n := len(mempoolChart.Values); n > 0 {
		health.MempoolTxCount = int64(mempoolChart.Values[n-1].Value)
	}

	health.Available = true
	return health
}

func fetchBlockchainStats(ctx context.Context) (*BlockchainStatsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blockchainStatsURL+"?format=json", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := bcClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 status: %d", resp.StatusCode)
	}

	var stats BlockchainStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &stats, nil
}

func fetchChart(ctx context.Context, chartName, timespan string) (*BlockchainChartResponse, error) {
	url := fmt.Sprintf("%s/%s?timespan=%s&format=json", blockchainChartsURL, chartName, timespan)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := bcClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 status: %d", resp.StatusCode)
	}

	var chart BlockchainChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&chart); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &chart, nil
}

// analyzeHashRate computes 7-day average, change %, and capitulation detection.
func analyzeHashRate(h *BTCNetworkHealth, points []BlockchainChartPoint) {
	n := len(points)
	if n < 2 {
		return
	}

	// 7-day average: last 7 points (daily data).
	start7 := n - 7
	if start7 < 0 {
		start7 = 0
	}
	var sum7 float64
	count7 := 0
	for i := start7; i < n; i++ {
		sum7 += points[i].Value
		count7++
	}
	if count7 > 0 {
		// Chart hash-rate is in TH/s.
		h.HashRate7DAvg = sum7 / float64(count7) / 1e12
	}

	// Change over 7 days: compare first-week avg vs last-week avg.
	if n >= 14 {
		var oldSum float64
		oldStart := n - 14
		for i := oldStart; i < oldStart+7; i++ {
			oldSum += points[i].Value
		}
		oldAvg := oldSum / 7.0
		newAvg := sum7 / float64(count7)
		if oldAvg > 0 {
			h.HashRateChange = (newAvg - oldAvg) / oldAvg * 100
		}
	} else if n >= 2 {
		oldVal := points[0].Value
		newVal := points[n-1].Value
		if oldVal > 0 {
			h.HashRateChange = (newVal - oldVal) / oldVal * 100
		}
	}

	// Miner capitulation: hash rate dropped >10% over 7 days.
	h.MinerCapitulation = h.HashRateChange < -10
}

// analyzeFees computes fee analysis and spike detection.
func analyzeFees(h *BTCNetworkHealth, points []BlockchainChartPoint) {
	n := len(points)
	if n < 2 {
		return
	}

	// Latest daily fee total (in BTC).
	h.TotalFeesBTC = points[n-1].Value

	// Compute 30-day average fee.
	var total float64
	for _, p := range points {
		total += p.Value
	}
	avg30 := total / float64(n)

	// Average fee per tx.
	if h.NTx24H > 0 {
		h.AvgFeeBTC = h.TotalFeesBTC / float64(h.NTx24H)
	}

	// Fee spike: latest > 2x 30-day average.
	h.FeeSpike = avg30 > 0 && h.TotalFeesBTC > 2*avg30

	// Difficulty change: approximate from consecutive difficulty values.
	// (Not available from fees chart, but we set it from stats if available.)
}
