package sentiment

import (
	"github.com/arkcode369/ark-intelligent/internal/service/dvol"
)

// IntegrateDVOLIntoSentiment copies DVOL analysis results into SentimentData
// so the sentiment dashboard can display crypto volatility alongside traditional
// indicators (VIX, MOVE, Fear & Greed).
func IntegrateDVOLIntoSentiment(data *SentimentData, result *dvol.DVOLResult) {
	if result == nil || !result.Available {
		return
	}

	// BTC DVOL
	if result.BTC.Available {
		data.DVOLBTCCurrent = result.BTC.Current
		data.DVOLBTCChange24hPct = result.BTC.Change24hPct
		data.DVOLBTCHigh24h = result.BTC.High24h
		data.DVOLBTCLow24h = result.BTC.Low24h
		data.DVOLBTCHV = result.BTC.HV
		data.DVOLBTCIVHVSpread = result.BTC.IVHVSpread
		data.DVOLBTCIVHVRatio = result.BTC.IVHVRatio
		data.DVOLBTCSpike = result.BTC.Spike
		data.DVOLBTCAvailable = true
	}

	// ETH DVOL
	if result.ETH.Available {
		data.DVOLETHCurrent = result.ETH.Current
		data.DVOLETHChange24hPct = result.ETH.Change24hPct
		data.DVOLETHHigh24h = result.ETH.High24h
		data.DVOLETHLow24h = result.ETH.Low24h
		data.DVOLETHHV = result.ETH.HV
		data.DVOLETHIVHVSpread = result.ETH.IVHVSpread
		data.DVOLETHIVHVRatio = result.ETH.IVHVRatio
		data.DVOLETHSpike = result.ETH.Spike
		data.DVOLETHAvailable = true
	}

	data.DVOLAvailable = data.DVOLBTCAvailable || data.DVOLETHAvailable
}
