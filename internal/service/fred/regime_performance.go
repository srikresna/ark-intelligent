package fred

import (
	"context"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// RegimeAssetReturn holds the return of a single asset during a specific regime.
type RegimeAssetReturn struct {
	Regime           string
	Currency         string
	TotalWeeks       int
	AvgWeeklyReturn  float64 // %
	AnnualizedReturn float64 // Avg weekly * 52
	WinWeeks         int     // Weeks with positive return
	WinRate          float64 // %
}

// RegimePerformanceMatrix maps regime -> []RegimeAssetReturn (one per currency).
type RegimePerformanceMatrix struct {
	Regimes    []string                       // ordered regime names
	Currencies []string                       // ordered currency names
	Data       map[string][]RegimeAssetReturn // keyed by regime
	Current    string                         // current active regime (if known)
}

// SignalGetter is the subset of SignalRepository needed by RegimePerformanceBuilder.
// Defined here to avoid importing the ports package (which imports fred, causing a cycle).
type SignalGetter interface {
	GetAllSignals(ctx context.Context) ([]domain.PersistedSignal, error)
}

// RegimePerformanceBuilder computes the regime-asset performance matrix
// from historical persisted signals.
type RegimePerformanceBuilder struct {
	signalRepo SignalGetter
}

func NewRegimePerformanceBuilder(signalRepo SignalGetter) *RegimePerformanceBuilder {
	return &RegimePerformanceBuilder{signalRepo: signalRepo}
}

// Build computes the performance matrix.
func (b *RegimePerformanceBuilder) Build(ctx context.Context) (*RegimePerformanceMatrix, error) {
	allSignals, err := b.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, err
	}

	// Group by regime + currency, collect 1W returns
	type groupKey struct {
		regime   string
		currency string
	}
	groups := make(map[groupKey][]float64)
	regimeSet := make(map[string]bool)
	currencySet := make(map[string]bool)

	for _, s := range allSignals {
		if s.FREDRegime == "" || s.Outcome1W == "" || s.Outcome1W == domain.OutcomePending {
			continue
		}
		gk := groupKey{regime: s.FREDRegime, currency: s.Currency}
		groups[gk] = append(groups[gk], s.Return1W)
		regimeSet[s.FREDRegime] = true
		currencySet[s.Currency] = true
	}

	if len(groups) == 0 {
		return nil, nil
	}

	// Build sorted lists
	var regimes []string
	for r := range regimeSet {
		regimes = append(regimes, r)
	}
	sort.Strings(regimes)

	var currencies []string
	for c := range currencySet {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)

	// Compute per-regime returns
	data := make(map[string][]RegimeAssetReturn)
	for _, regime := range regimes {
		var returns []RegimeAssetReturn
		for _, currency := range currencies {
			gk := groupKey{regime: regime, currency: currency}
			weeklyReturns := groups[gk]
			if len(weeklyReturns) == 0 {
				returns = append(returns, RegimeAssetReturn{
					Regime:   regime,
					Currency: currency,
				})
				continue
			}

			var sum float64
			var wins int
			for _, r := range weeklyReturns {
				sum += r
				if r > 0 {
					wins++
				}
			}
			n := len(weeklyReturns)
			avgWeekly := sum / float64(n)

			returns = append(returns, RegimeAssetReturn{
				Regime:           regime,
				Currency:         currency,
				TotalWeeks:       n,
				AvgWeeklyReturn:  math.Round(avgWeekly*10000) / 10000,
				AnnualizedReturn: math.Round(avgWeekly*52*100) / 100,
				WinWeeks:         wins,
				WinRate:          math.Round(float64(wins)/float64(n)*100*10) / 10,
			})
		}
		data[regime] = returns
	}

	// Detect current regime from most recent signal
	current := ""
	if len(allSignals) > 0 {
		// Signals may not be sorted; find the latest
		latest := allSignals[0]
		for _, s := range allSignals[1:] {
			if s.ReportDate.After(latest.ReportDate) {
				latest = s
			}
		}
		current = latest.FREDRegime
	}

	return &RegimePerformanceMatrix{
		Regimes:    regimes,
		Currencies: currencies,
		Data:       data,
		Current:    current,
	}, nil
}
