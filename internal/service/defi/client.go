package defi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/rs/zerolog/log"
)

const (
	protocolsURL    = "https://api.llama.fi/v2/protocols"
	chainsURL       = "https://api.llama.fi/v2/chains"
	dexOverviewURL  = "https://api.llama.fi/overview/dexs"
	stablecoinsURL  = "https://stablecoins.llama.fi/stablecoins"
	clientTimeout   = 20 * time.Second
	cacheTTL        = 4 * time.Hour
)

var (
	globalReport *DeFiReport    //nolint:gochecknoglobals
	cacheMu      sync.RWMutex  //nolint:gochecknoglobals
	client       = httpclient.NewClient(clientTimeout) //nolint:gochecknoglobals
	dlog         = log.With().Str("component", "defi").Logger() //nolint:gochecknoglobals
)

// GetCachedOrFetch returns cached DeFi data if within TTL, otherwise
// fetches fresh data from DefiLlama. Gracefully degrades with stale cache.
func GetCachedOrFetch(ctx context.Context) *DeFiReport {
	cacheMu.RLock()
	if globalReport != nil && time.Since(globalReport.FetchedAt) < cacheTTL {
		r := globalReport
		cacheMu.RUnlock()
		return r
	}
	cacheMu.RUnlock()

	fresh := fetchAll(ctx)

	cacheMu.Lock()
	if fresh.Available {
		globalReport = fresh
	} else if globalReport != nil {
		stale := globalReport
		cacheMu.Unlock()
		dlog.Warn().Msg("fetch failed, returning stale cache")
		return stale
	}
	cacheMu.Unlock()
	return fresh
}

// fetchAll fetches protocols TVL, DEX volume, and stablecoin data concurrently.
func fetchAll(ctx context.Context) *DeFiReport {
	report := &DeFiReport{FetchedAt: time.Now()}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Fetch protocols (TVL)
	wg.Add(1)
	go func() {
		defer wg.Done()
		protocols, chains, err := fetchProtocolsTVL(ctx)
		if err != nil {
			dlog.Error().Err(err).Msg("failed to fetch protocols TVL")
			return
		}
		mu.Lock()
		report.TopProtocols = protocols
		report.TopChains = chains
		for _, p := range protocols {
			report.TotalTVL += p.TVL
		}
		// Calculate total TVL change as weighted average of top protocols
		if len(protocols) > 0 && report.TotalTVL > 0 {
			var weightedChange float64
			for _, p := range protocols {
				weightedChange += p.Change1D * (p.TVL / report.TotalTVL)
			}
			report.TVLChange24h = weightedChange
		}
		mu.Unlock()
	}()

	// Fetch DEX volume
	wg.Add(1)
	go func() {
		defer wg.Done()
		dex, err := fetchDEXVolume(ctx)
		if err != nil {
			dlog.Error().Err(err).Msg("failed to fetch DEX volume")
			return
		}
		mu.Lock()
		report.DEX = *dex
		mu.Unlock()
	}()

	// Fetch stablecoins
	wg.Add(1)
	go func() {
		defer wg.Done()
		stables, totalSupply, change7d, err := fetchStablecoins(ctx)
		if err != nil {
			dlog.Error().Err(err).Msg("failed to fetch stablecoins")
			return
		}
		mu.Lock()
		report.Stablecoins = stables
		report.TotalStablecoinSupply = totalSupply
		report.StablecoinChange7D = change7d
		mu.Unlock()
	}()

	wg.Wait()

	// Mark available if we got at least TVL data
	report.Available = report.TotalTVL > 0 || len(report.Stablecoins) > 0
	if report.Available {
		report.Signals = analyzeSignals(report)
	}
	return report
}

// --- Protocol / TVL ---

// llamaProtocol matches the DefiLlama /v2/protocols response shape.
type llamaProtocol struct {
	Name     string  `json:"name"`
	Symbol   string  `json:"symbol"`
	TVL      float64 `json:"tvl"`
	Change1H float64 `json:"change_1h"`
	Change1D float64 `json:"change_1d"`
	Change7D float64 `json:"change_7d"`
	Chain    string  `json:"chain"`
	Category string  `json:"category"`
	Chains   []string `json:"chains"`
}

// llamaChain matches the DefiLlama /v2/chains response shape.
type llamaChain struct {
	Name string  `json:"name"`
	TVL  float64 `json:"tvl"`
}

func fetchProtocolsTVL(ctx context.Context) ([]ProtocolTVL, []ChainTVL, error) {
	// Fetch protocols
	var rawProtocols []llamaProtocol
	if err := getJSON(ctx, protocolsURL, &rawProtocols); err != nil {
		return nil, nil, fmt.Errorf("protocols: %w", err)
	}

	// Sort by TVL descending and take top 10
	sort.Slice(rawProtocols, func(i, j int) bool {
		return rawProtocols[i].TVL > rawProtocols[j].TVL
	})

	limit := 10
	if len(rawProtocols) < limit {
		limit = len(rawProtocols)
	}

	protocols := make([]ProtocolTVL, 0, limit)
	for _, p := range rawProtocols[:limit] {
		chain := p.Chain
		if len(p.Chains) > 0 {
			chain = p.Chains[0]
			if len(p.Chains) > 1 {
				chain = "Multi-chain"
			}
		}
		protocols = append(protocols, ProtocolTVL{
			Name:     p.Name,
			Symbol:   p.Symbol,
			TVL:      p.TVL,
			Change1D: p.Change1D,
			Change7D: p.Change7D,
			Chain:    chain,
			Category: p.Category,
		})
	}

	// Fetch chains
	var rawChains []llamaChain
	if err := getJSON(ctx, chainsURL, &rawChains); err != nil {
		dlog.Warn().Err(err).Msg("failed to fetch chain TVL, continuing without")
		return protocols, nil, nil
	}

	sort.Slice(rawChains, func(i, j int) bool {
		return rawChains[i].TVL > rawChains[j].TVL
	})
	chainLimit := 8
	if len(rawChains) < chainLimit {
		chainLimit = len(rawChains)
	}
	chains := make([]ChainTVL, 0, chainLimit)
	for _, c := range rawChains[:chainLimit] {
		chains = append(chains, ChainTVL{Name: c.Name, TVL: c.TVL})
	}

	return protocols, chains, nil
}

// --- DEX Volume ---

// llamaDEXOverview matches the DefiLlama /overview/dexs response.
type llamaDEXOverview struct {
	TotalDataChart       []any `json:"totalDataChart"`
	Total24h             float64 `json:"total24h"`
	Total48hto24h        float64 `json:"total48hto24h"`
	TotalDataChartBreakdown []any `json:"totalDataChartBreakdown"`
	Protocols            []llamaDEXProtocol `json:"protocols"`
}

type llamaDEXProtocol struct {
	Name          string  `json:"name"`
	Total24h      float64 `json:"total24h"`
	Total48hto24h float64 `json:"total48hto24h"`
	Change1d      float64 `json:"change_1d"`
}

func fetchDEXVolume(ctx context.Context) (*DEXVolume, error) {
	var overview llamaDEXOverview
	if err := getJSON(ctx, dexOverviewURL, &overview); err != nil {
		return nil, fmt.Errorf("dex overview: %w", err)
	}

	change24h := 0.0
	if overview.Total48hto24h > 0 {
		change24h = ((overview.Total24h - overview.Total48hto24h) / overview.Total48hto24h) * 100
	}

	// Sort protocols by volume
	sort.Slice(overview.Protocols, func(i, j int) bool {
		return overview.Protocols[i].Total24h > overview.Protocols[j].Total24h
	})

	limit := 5
	if len(overview.Protocols) < limit {
		limit = len(overview.Protocols)
	}

	top := make([]DEXProtocol, 0, limit)
	for _, p := range overview.Protocols[:limit] {
		top = append(top, DEXProtocol{
			Name:      p.Name,
			Volume24h: p.Total24h,
			Change24h: p.Change1d,
		})
	}

	return &DEXVolume{
		TotalVolume24h: overview.Total24h,
		Change24h:      change24h,
		TopProtocols:   top,
	}, nil
}

// --- Stablecoins ---

// llamaStablecoinsResp matches the DefiLlama stablecoins response.
type llamaStablecoinsResp struct {
	PeggedAssets []llamaStablecoin `json:"peggedAssets"`
}

type llamaStablecoin struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	CircSupply map[string]map[string]float64 `json:"chainCirculating"`
	Chains   []string `json:"chains"`
	PegType  string `json:"pegType"`
	Price    float64 `json:"price"`
}

func fetchStablecoins(ctx context.Context) ([]StablecoinData, float64, float64, error) {
	var resp llamaStablecoinsResp
	if err := getJSON(ctx, stablecoinsURL, &resp); err != nil {
		return nil, 0, 0, fmt.Errorf("stablecoins: %w", err)
	}

	// Calculate total supply per stablecoin from chain data
	// The API returns circulating supply broken down by chain
	type stableInfo struct {
		name   string
		symbol string
		supply float64
	}

	var stables []stableInfo
	for _, s := range resp.PeggedAssets {
		if s.PegType != "peggedUSD" {
			continue
		}
		totalSupply := 0.0
		for _, chainData := range s.CircSupply {
			if v, ok := chainData["current"]; ok {
				totalSupply += v
			}
		}
		if totalSupply > 100_000_000 { // only include >$100M
			stables = append(stables, stableInfo{
				name:   s.Name,
				symbol: s.Symbol,
				supply: totalSupply,
			})
		}
	}

	sort.Slice(stables, func(i, j int) bool {
		return stables[i].supply > stables[j].supply
	})

	limit := 6
	if len(stables) < limit {
		limit = len(stables)
	}

	var totalSupply float64
	result := make([]StablecoinData, 0, limit)
	for _, s := range stables[:limit] {
		totalSupply += s.supply
		result = append(result, StablecoinData{
			Name:        s.name,
			Symbol:      s.symbol,
			TotalSupply: s.supply,
		})
	}

	// Note: 7d change requires historical endpoint; omit for V1
	return result, totalSupply, 0, nil
}

// --- Helpers ---

func getJSON(ctx context.Context, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}
