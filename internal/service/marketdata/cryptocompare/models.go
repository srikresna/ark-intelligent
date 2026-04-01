// Package cryptocompare provides a client for the CryptoCompare free API.
// Tracks per-exchange volume to detect market share shifts and volume divergence.
// No API key required for basic tier. Rate: 100K calls/month.
package cryptocompare

import "time"

// ExchangeVolume holds daily volume data for a single exchange.
type ExchangeVolume struct {
	Exchange  string    // e.g. "Binance", "Coinbase"
	VolumeUSD float64   // Total USD volume for the day
	Change24H float64   // 24h volume change percentage
	Change7D  float64   // 7d volume change percentage
	Share     float64   // Market share percentage (0-100)
	FetchedAt time.Time // When data was fetched
}

// TopAsset holds volume-ranked asset info.
type TopAsset struct {
	Symbol    string  // e.g. "BTC"
	FullName  string  // e.g. "Bitcoin"
	VolumeUSD float64 // 24h volume in USD
	Change24H float64 // 24h volume change percentage
	Price     float64 // Current price in USD
}

// VolumeSummary aggregates exchange volume analysis.
type VolumeSummary struct {
	Exchanges  []ExchangeVolume // Per-exchange volume data (sorted by volume desc)
	TopAssets  []TopAsset       // Top assets by volume
	TotalUSD   float64          // Total volume across all tracked exchanges
	Divergence string           // "NONE" | "MODERATE" | "SIGNIFICANT"
	DivDetail  string           // Human-readable divergence description
	FetchedAt  time.Time        // When analysis was computed
	Available  bool             // Whether data was successfully retrieved
}

// apiExchangeDayPoint represents one day of exchange volume from the API.
type apiExchangeDayPoint struct {
	Time   int64   `json:"time"`
	Volume float64 `json:"volume"`
}

// apiExchangeHistoResponse is the response from /data/exchange/histoday.
type apiExchangeHistoResponse struct {
	Response string                `json:"Response"`
	Message  string                `json:"Message"`
	Data     []apiExchangeDayPoint `json:"Data"`
}

// apiTopVolCoin is one entry from the /data/top/totalvolfull response.
type apiTopVolCoin struct {
	CoinInfo struct {
		Name     string `json:"Name"`
		FullName string `json:"FullName"`
	} `json:"CoinInfo"`
	RAW map[string]struct {
		VOLUME24HOURTO   float64 `json:"VOLUME24HOURTO"`
		CHANGEPCT24HOUR  float64 `json:"CHANGEPCT24HOUR"`
		PRICE            float64 `json:"PRICE"`
	} `json:"RAW"`
}

// apiTopVolResponse is the response from /data/top/totalvolfull.
type apiTopVolResponse struct {
	Response string          `json:"Response"`
	Message  string          `json:"Message"`
	Data     []apiTopVolCoin `json:"Data"`
}
