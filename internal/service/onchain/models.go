// Package onchain provides on-chain metrics via public APIs (CoinMetrics community tier).
package onchain

import "time"

// ExchangeFlow holds daily exchange in/out flow for a single asset.
type ExchangeFlow struct {
	Date       time.Time
	FlowInNtv  float64 // coins flowing into exchanges
	FlowOutNtv float64 // coins flowing out of exchanges
	NetFlow    float64 // FlowInNtv - FlowOutNtv (negative = accumulation)
}

// ActiveAddressMetric holds daily active address and tx count.
type ActiveAddressMetric struct {
	Date           time.Time
	ActiveAddresses int64
	TxCount        int64
}

// AssetOnChainSummary holds computed on-chain metrics for an asset.
type AssetOnChainSummary struct {
	Asset              string
	Flows              []ExchangeFlow
	NetFlow7D          float64   // sum of net flows over last 7 days
	NetFlow30D         float64   // sum of net flows over last 30 days
	ConsecutiveOutflow int       // consecutive days of net outflow (accumulation)
	LargeInflowSpike   bool      // single-day inflow > 2x avg
	FlowTrend          string    // "ACCUMULATION" | "DISTRIBUTION" | "NEUTRAL"
	ActiveAddresses    int64     // latest active address count
	ActiveAddrChange7D float64   // % change in active addresses over 7 days
	TxCount            int64     // latest tx count
	FetchedAt          time.Time
	Available          bool
}

// OnChainReport holds the combined on-chain report for all tracked assets.
type OnChainReport struct {
	Assets    map[string]*AssetOnChainSummary // keyed by asset symbol (e.g. "btc", "eth")
	FetchedAt time.Time
	Available bool
}

// BTCNetworkHealth holds BTC network metrics from Blockchain.com API.
type BTCNetworkHealth struct {
	HashRate          float64   // TH/s — current network hash rate
	HashRate7DAvg     float64   // TH/s — 7-day average
	HashRateChange    float64   // % change over 7 days
	MempoolBytes      int64     // current mempool size in bytes
	MempoolTxCount    int64     // pending transactions in mempool
	TotalFeesBTC      float64   // total fees in BTC (last 24h)
	AvgFeeBTC         float64   // average fee per tx (BTC)
	FeeSpike          bool      // fee > 2x 30-day average
	Difficulty        float64   // current mining difficulty
	DifficultyChange  float64   // last difficulty adjustment %
	MarketPriceUSD    float64   // BTC/USD price from Blockchain.com
	NTx24H            int64     // transactions count last 24h
	MinerCapitulation bool      // hash rate dropped >10% over 7 days
	MempoolCongested  bool      // mempool > 100MB
	FetchedAt         time.Time
	Available         bool
}

// BlockchainChartPoint is a single data point from Blockchain.com Charts API.
type BlockchainChartPoint struct {
	Timestamp int64   `json:"x"` // unix seconds
	Value     float64 `json:"y"`
}

// BlockchainChartResponse is the Charts API response.
type BlockchainChartResponse struct {
	Status string                 `json:"status"`
	Name   string                 `json:"name"`
	Unit   string                 `json:"unit"`
	Period string                 `json:"period"`
	Values []BlockchainChartPoint `json:"values"`
}

// BlockchainStatsResponse is the /stats endpoint response (subset of fields).
type BlockchainStatsResponse struct {
	MarketPriceUSD       float64 `json:"market_price_usd"`
	HashRate             float64 `json:"hash_rate"`
	TotalFeesBTC         float64 `json:"total_fees_btc"`
	NTx                  int64   `json:"n_tx"`
	Difficulty           float64 `json:"difficulty"`
	MinutesBetweenBlocks float64 `json:"minutes_between_blocks"`
	MempoolSize          int64   `json:"mempool_size"`
}
