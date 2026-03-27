package strategy

import (
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/factors"
)

// Macro regime constants
const (
	macroExpansion = "EXPANSION"
	macroSlowdown  = "SLOWDOWN"
	macroRecession = "RECESSION"
	macroRecovery  = "RECOVERY"
)

// Input bundles all data the strategy engine needs to generate a playbook.
type Input struct {
	// Factor ranking from the factor engine
	Ranking *factors.RankingResult

	// Macro regime from FRED service (optional)
	MacroRegime string // e.g. "EXPANSION", "SLOWDOWN", "RECESSION", "RECOVERY"

	// COT bias per contract code (optional) — pre-computed
	COTBias map[string]string // contractCode → "BULLISH"/"BEARISH"/"NEUTRAL"

	// Volatility regime per contract code (optional)
	VolRegime map[string]string // contractCode → "EXPANDING"/"CONTRACTING"/"NORMAL"

	// Rate differentials in bps per contract code (for carry context)
	CarryBps map[string]float64

	// Transition data (from FRED regime history)
	TransitionProb float64 // probability of macro regime transition
	TransitionFrom string
	TransitionTo   string
}

// Engine generates strategy playbooks from factor rankings and macro context.
type Engine struct{}

// NewEngine creates a Strategy Engine.
func NewEngine() *Engine { return &Engine{} }

// Generate produces a PlaybookResult from factor rankings and macro context.
func (e *Engine) Generate(in Input) *PlaybookResult {
	if in.Ranking == nil || len(in.Ranking.Assets) == 0 {
		return &PlaybookResult{ComputedAt: time.Now()}
	}

	entries := make([]PlaybookEntry, 0, len(in.Ranking.Assets))
	maxAssets := in.Ranking.AssetCount

	for _, asset := range in.Ranking.Assets {
		// Determine direction from factor signal
		dir := factorSignalToDirection(asset.Signal)
		if dir == DirectionFlat {
			continue // skip flat signals
		}

		// Base conviction from composite score magnitude
		conviction := convictionFromScore(asset.CompositeScore)

		// Boost/penalize from COT alignment
		cotBias := ""
		if in.COTBias != nil {
			cotBias = in.COTBias[asset.ContractCode]
		}
		conviction = adjustForCOT(conviction, dir, cotBias)

		// Penalize if vol is expanding (risk management)
		volRegime := ""
		if in.VolRegime != nil {
			volRegime = in.VolRegime[asset.ContractCode]
		}
		if volRegime == "EXPANDING" {
			conviction *= 0.80
		}

		// Penalize if macro regime doesn't fit trade
		regimeFit := assessRegimeFit(in.MacroRegime, asset.Currency, dir)
		if regimeFit == "AGAINST_REGIME" {
			conviction *= 0.60
		} else if regimeFit == "ALIGNED" {
			conviction *= 1.10
		}
		if conviction > 1.0 {
			conviction = 1.0
		}

		// Check transition warning
		isTransition := in.TransitionProb > 0.50
		transNote := ""
		if isTransition {
			conviction *= 0.75
			transNote = buildTransitionNote(in.TransitionFrom, in.TransitionTo)
		}

		carry := 0.0
		if in.CarryBps != nil {
			carry = in.CarryBps[asset.ContractCode]
		}

		entries = append(entries, PlaybookEntry{
			ContractCode:     asset.ContractCode,
			Currency:         asset.Currency,
			Name:             asset.Name,
			Direction:        dir,
			Conviction:       conviction,
			ConvLevel:        ConvictionToLevel(conviction),
			FactorScore:      asset.CompositeScore,
			COTBias:          cotBias,
			RegimeFit:        regimeFit,
			RateDiffBps:      carry,
			VolatilityRegime: volRegime,
			IsTransition:     isTransition,
			TransitionNote:   transNote,
			UpdatedAt:        time.Now(),
		})
	}

	// Sort by conviction descending, then by direction (longs before shorts)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Conviction != entries[j].Conviction {
			return entries[i].Conviction > entries[j].Conviction
		}
		// longs first
		if entries[i].Direction != entries[j].Direction {
			return entries[i].Direction == DirectionLong
		}
		return false
	})

	heat := computeHeat(entries)
	transition := buildTransitionWarning(in, entries)

	_ = maxAssets // used for future portfolio constraints
	return &PlaybookResult{
		Playbook:    entries,
		Heat:        heat,
		Transition:  transition,
		MacroRegime: in.MacroRegime,
		ComputedAt:  time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func factorSignalToDirection(s factors.Signal) Direction {
	switch s {
	case factors.SignalStrongLong, factors.SignalLong:
		return DirectionLong
	case factors.SignalStrongShort, factors.SignalShort:
		return DirectionShort
	default:
		return DirectionFlat
	}
}

func convictionFromScore(composite float64) float64 {
	// Map [-1,+1] composite to [0,1] conviction
	abs := composite
	if abs < 0 {
		abs = -abs
	}
	// linear: 0.20 composite → 0.40 conviction, 1.0 → 1.0
	c := abs * 1.0
	if c > 1.0 {
		c = 1.0
	}
	return c
}

func adjustForCOT(conviction float64, dir Direction, cotBias string) float64 {
	switch {
	case dir == DirectionLong && cotBias == "BULLISH":
		return conviction * 1.15 // confirmed
	case dir == DirectionShort && cotBias == "BEARISH":
		return conviction * 1.15
	case dir == DirectionLong && cotBias == "BEARISH":
		return conviction * 0.70 // divergence
	case dir == DirectionShort && cotBias == "BULLISH":
		return conviction * 0.70
	}
	return conviction
}

// assessRegimeFit checks if a trade direction is aligned with the macro regime.
// Returns "ALIGNED", "NEUTRAL", or "AGAINST_REGIME".
func assessRegimeFit(regime, currency string, dir Direction) string {
	if regime == "" {
		return "NEUTRAL"
	}
	// Risk-on assets: equities, AUD, NZD, CAD, OIL → long in expansion
	// Risk-off assets: JPY, CHF, bonds, gold → long in slowdown/recession
	riskOn := map[string]bool{
		"SPX500": true, "NDX": true, "DJI": true, "RUT": true,
		"AUD": true, "NZD": true, "CAD": true, "OIL": true,
		"RBOB": true, "ULSD": true, "BTC": true, "ETH": true,
	}
	riskOff := map[string]bool{
		"JPY": true, "CHF": true, "BOND": true, "BOND30": true,
		"BOND5": true, "BOND2": true, "XAU": true, "XAG": true,
	}

	switch regime {
	case macroExpansion, macroRecovery:
		if riskOn[currency] && dir == DirectionLong {
			return "ALIGNED"
		}
		if riskOff[currency] && dir == DirectionShort {
			return "ALIGNED"
		}
		if riskOff[currency] && dir == DirectionLong {
			return "AGAINST_REGIME"
		}
	case macroSlowdown, macroRecession:
		if riskOff[currency] && dir == DirectionLong {
			return "ALIGNED"
		}
		if riskOn[currency] && dir == DirectionShort {
			return "ALIGNED"
		}
		if riskOn[currency] && dir == DirectionLong {
			return "AGAINST_REGIME"
		}
	}
	return "NEUTRAL"
}

func buildTransitionNote(from, to string) string {
	if from == "" || to == "" {
		return "Regime transition in progress — reduce position size"
	}
	return "Regime shifting " + from + " → " + to + " — reduce size"
}

func computeHeat(entries []PlaybookEntry) PortfolioHeat {
	heat := PortfolioHeat{UpdatedAt: time.Now()}
	for _, e := range entries {
		if e.Direction == DirectionLong {
			heat.LongExposure += e.Conviction
			heat.ActiveTrades++
		} else if e.Direction == DirectionShort {
			heat.ShortExposure += e.Conviction
			heat.ActiveTrades++
		}
	}
	// Normalize by max possible exposure (assuming 10 max positions per side)
	const maxPerSide = 10.0
	heat.NetExposure = heat.LongExposure - heat.ShortExposure
	heat.TotalExposure = (heat.LongExposure + heat.ShortExposure) / (2 * maxPerSide)
	if heat.TotalExposure > 1.0 {
		heat.TotalExposure = 1.0
	}
	heat.HeatLevel = ComputeHeatLevel(heat.TotalExposure)
	return heat
}

func buildTransitionWarning(in Input, entries []PlaybookEntry) TransitionWarning {
	if in.TransitionProb < 0.30 {
		return TransitionWarning{}
	}
	affected := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsTransition {
			affected = append(affected, e.Currency)
		}
	}
	return TransitionWarning{
		IsActive:       in.TransitionProb > 0.50,
		FromRegime:     in.TransitionFrom,
		ToRegime:       in.TransitionTo,
		Probability:    in.TransitionProb,
		AffectedAssets: affected,
		Note:           buildTransitionNote(in.TransitionFrom, in.TransitionTo),
		DetectedAt:     time.Now(),
	}
}
