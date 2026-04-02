// Package defi provides DeFi metrics via DefiLlama public APIs.
package defi

import "time"

// ProtocolTVL holds TVL data for a single DeFi protocol.
type ProtocolTVL struct {
	Name      string  `json:"name"`
	Symbol    string  `json:"symbol"`
	TVL       float64 `json:"tvl"`
	Change1D  float64 `json:"change_1d"`  // % change 24h
	Change7D  float64 `json:"change_7d"`  // % change 7d
	Chain     string  `json:"chain"`
	Category  string  `json:"category"`
}

// ChainTVL holds TVL data for a single blockchain.
type ChainTVL struct {
	Name  string  `json:"name"`
	TVL   float64 `json:"tvl"`
}

// DEXVolume holds 24h DEX trading volume.
type DEXVolume struct {
	TotalVolume24h float64       `json:"total_volume_24h"`
	Change24h      float64       `json:"change_24h"` // % change vs previous 24h
	TopProtocols   []DEXProtocol `json:"top_protocols"`
}

// DEXProtocol holds volume for a single DEX.
type DEXProtocol struct {
	Name      string  `json:"name"`
	Volume24h float64 `json:"volume_24h"`
	Change24h float64 `json:"change_24h"`
}

// StablecoinData holds stablecoin supply data.
type StablecoinData struct {
	Name        string  `json:"name"`
	Symbol      string  `json:"symbol"`
	TotalSupply float64 `json:"total_supply"` // circulating supply in USD
	Change7D    float64 `json:"change_7d"`    // % change 7d
}

// DeFiReport holds the combined DeFi dashboard data.
type DeFiReport struct {
	// TVL data
	TotalTVL      float64       `json:"total_tvl"`
	TVLChange24h  float64       `json:"tvl_change_24h"` // % change
	TopProtocols  []ProtocolTVL `json:"top_protocols"`   // top 10 by TVL
	TopChains     []ChainTVL    `json:"top_chains"`      // top chains by TVL

	// DEX volume
	DEX DEXVolume `json:"dex"`

	// Stablecoins
	TotalStablecoinSupply float64          `json:"total_stablecoin_supply"`
	StablecoinChange7D    float64          `json:"stablecoin_change_7d"` // % change
	Stablecoins           []StablecoinData `json:"stablecoins"`

	// Signals
	Signals []DeFiSignal `json:"signals"`

	FetchedAt time.Time `json:"fetched_at"`
	Available bool      `json:"available"`
}

// DeFiSignal represents a detected market signal from DeFi metrics.
type DeFiSignal struct {
	Type    string `json:"type"`    // "risk_off", "liquidity_inflow", "dex_surge", etc.
	Message string `json:"message"` // human-readable description
	Severity string `json:"severity"` // "info", "warning", "alert"
}
