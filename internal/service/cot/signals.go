package cot

import (
	"fmt"
	"math"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

// SignalDetector identifies actionable trading signals from COT data.
// It combines multiple positioning metrics to generate high-confidence
// signals: smart money moves, extreme positioning, and divergences.
type SignalDetector struct{}

// NewSignalDetector creates a signal detector.
func NewSignalDetector() *SignalDetector {
	return &SignalDetector{}
}

// Signal represents an actionable COT-based trading signal.
type Signal struct {
	ContractCode string
	Currency     string
	Type         SignalType
	Direction    string  // BULLISH, BEARISH
	Strength     int     // 1-5 (5 = strongest)
	Confidence   float64 // 0-100%
	Description  string
	Factors      []string // contributing factors
}

// SignalType categorizes the kind of signal.
type SignalType string

const (
	SignalSmartMoney      SignalType = "SMART_MONEY"
	SignalExtreme         SignalType = "EXTREME_POSITIONING"
	SignalDivergence      SignalType = "DIVERGENCE"
	SignalMomentumShift   SignalType = "MOMENTUM_SHIFT"
	SignalConcentration   SignalType = "CONCENTRATION"
	SignalCrowdContrarian SignalType = "CROWD_CONTRARIAN"
	SignalThinMarket     SignalType = "THIN_MARKET"
)

// DetectAll runs all signal detectors on a set of analyses.
func (sd *SignalDetector) DetectAll(analyses []domain.COTAnalysis, historyMap map[string][]domain.COTRecord) []Signal {
	var signals []Signal

	for _, a := range analyses {
		history := historyMap[a.Contract.Code]

		// Run each detector
		if s := sd.detectSmartMoney(a, history); s != nil {
			signals = append(signals, *s)
		}
		if s := sd.detectExtreme(a); s != nil {
			signals = append(signals, *s)
		}
		if s := sd.detectDivergence(a, history); s != nil {
			signals = append(signals, *s)
		}
		if s := sd.detectMomentumShift(a, history); s != nil {
			signals = append(signals, *s)
		}
		if s := sd.detectConcentrationRisk(a); s != nil {
			signals = append(signals, *s)
		}
		if s := sd.detectCrowdContrarian(a); s != nil {
			signals = append(signals, *s)
		}
		if s := sd.detectThinMarket(a); s != nil {
			signals = append(signals, *s)
		}
	}

	// Sort by strength descending
	sortSignals(signals)
	return signals
}

// detectSmartMoney identifies when commercial hedgers are making significant moves.
//
// Interpretation differs by report type:
//   - DISAGGREGATED (commodities): Prod/Swap are true hedgers — contrarian signal.
//     High COTIndexComm (>80) = they are heavily long = BULLISH for price.
//     Low COTIndexComm (<20)  = they are net short/hedging = BEARISH for price.
//   - TFF (forex/indices): Dealers are market makers, not true hedgers.
//     High COTIndexComm (>80) = dealers accumulating long = BULLISH (directional).
//     Low COTIndexComm (<20)  = dealers net short = BEARISH (directional).
func (sd *SignalDetector) detectSmartMoney(a domain.COTAnalysis, history []domain.COTRecord) *Signal {
	// Need significant commercial position change
	commChangeAbs := math.Abs(a.CommNetChange)
	if commChangeAbs < 5000 {
		return nil
	}

	// Check if commercial index is at extreme
	if a.COTIndexComm < 20 || a.COTIndexComm > 80 {
		// Direction logic:
		// For DISAGGREGATED: high COTIndexComm = commercials net long = BULLISH (producers know value)
		// For TFF: high COTIndexComm = dealers net long = BULLISH (directional accumulation)
		// Both cases: COTIndexComm > 80 = BULLISH, < 20 = BEARISH
		direction := "BULLISH"
		if a.COTIndexComm < 20 {
			direction = "BEARISH"
		}

		strength := 3
		if commChangeAbs > 15000 {
			strength = 5
		} else if commChangeAbs > 10000 {
			strength = 4
		}

		confidence := mathutil.Clamp(commChangeAbs/200, 30, 95)

		factors := []string{
			fmt.Sprintf("Commercial net change: %s", fmtutil.FmtNumSigned(a.CommNetChange, 0)),
			fmt.Sprintf("Commercial COT Index: %.1f", a.COTIndexComm),
			fmt.Sprintf("Commercial signal: %s", a.CommercialSignal),
		}

		hedgerLabel := "Commercials"
		if a.Contract.ReportType == "TFF" {
			hedgerLabel = "Dealers"
		}
		return &Signal{
			ContractCode: a.Contract.Code,
			Currency:     a.Contract.Currency,
			Type:         SignalSmartMoney,
			Direction:    direction,
			Strength:     strength,
			Confidence:   confidence,
			Description:  fmt.Sprintf("%s at extreme COT Index (%.0f) with large position change → %s", hedgerLabel, a.COTIndexComm, direction),
			Factors:      factors,
		}
	}

	return nil
}

// detectExtreme identifies when positioning reaches historically extreme levels.
func (sd *SignalDetector) detectExtreme(a domain.COTAnalysis) *Signal {
	// Check COT Index extremes
	if a.COTIndex > 10 && a.COTIndex < 90 {
		return nil
	}

	// Additional confirmation from Z-Score
	hasZConfirm := math.Abs(a.WillcoIndex-50) > 30

	direction := "BULLISH"
	if a.COTIndex >= 90 {
		// Speculators extremely bullish = potential contrarian sell
		direction = "BEARISH"
	}

	strength := 3
	if (a.COTIndex <= 5 || a.COTIndex >= 95) && hasZConfirm {
		strength = 5
	} else if a.COTIndex <= 5 || a.COTIndex >= 95 {
		strength = 4
	}

	confidence := 60.0
	if hasZConfirm {
		confidence = 80.0
	}

	factors := []string{
		fmt.Sprintf("Speculator COT Index: %.1f (extreme)", a.COTIndex),
		fmt.Sprintf("Willco Index: %.2f", a.WillcoIndex),
		fmt.Sprintf("Percentile: %.0f%%", a.COTIndex),
	}

	return &Signal{
		ContractCode: a.Contract.Code,
		Currency:     a.Contract.Currency,
		Type:         SignalExtreme,
		Direction:    direction,
		Strength:     strength,
		Confidence:   confidence,
		Description:  fmt.Sprintf("Extreme positioning: Spec COT Index at %.0f (contrarian %s)", a.COTIndex, direction),
		Factors:      factors,
	}
}

// detectDivergence identifies when commercials and speculators are moving
// in opposite directions — a classic setup for reversals.
func (sd *SignalDetector) detectDivergence(a domain.COTAnalysis, history []domain.COTRecord) *Signal {
	if !a.DivergenceFlag {
		return nil
	}

	// Check if divergence is persistent (at least 2 consecutive weeks)
	// Count consecutive weeks of divergence using modern API fields.
	// Compute changes from position diffs between consecutive weeks.
	consecutive := 1
	rt := a.Contract.ReportType
	if len(history) >= 3 {
		for i := 1; i < min(4, len(history)-1); i++ {
			curr := history[i]
			prev := history[i+1]
			// Use modern fields: compute spec/comm net change from position diffs
			var specChg, commChg float64
			if rt == "TFF" {
				specChg = (curr.LevFundLong - curr.LevFundShort) - (prev.LevFundLong - prev.LevFundShort)
				commChg = (curr.DealerLong - curr.DealerShort) - (prev.DealerLong - prev.DealerShort)
			} else {
				specChg = (curr.ManagedMoneyLong - curr.ManagedMoneyShort) - (prev.ManagedMoneyLong - prev.ManagedMoneyShort)
				commChg = (curr.ProdMercLong - curr.ProdMercShort + curr.SwapDealerLong - curr.SwapDealerShort) -
					(prev.ProdMercLong - prev.ProdMercShort + prev.SwapDealerLong - prev.SwapDealerShort)
			}
			if (specChg > 0 && commChg < 0) || (specChg < 0 && commChg > 0) {
				consecutive++
			} else {
				break
			}
		}
	}

	if consecutive < 2 {
		return nil // single-week divergence is noise
	}

	// Direction follows commercials (smart money)
	direction := "BULLISH"
	if a.CommNetChange < 0 {
		direction = "BEARISH"
	}

	strength := min(consecutive+1, 5)
	confidence := mathutil.Clamp(float64(consecutive)*25, 40, 90)

	factors := []string{
		fmt.Sprintf("Divergence persisting %d weeks", consecutive),
		fmt.Sprintf("Spec net change: %s", fmtutil.FmtNumSigned(a.NetChange, 0)),
		fmt.Sprintf("Comm net change: %s", fmtutil.FmtNumSigned(a.CommNetChange, 0)),
		fmt.Sprintf("Momentum direction: %s", a.MomentumDir),
	}

	return &Signal{
		ContractCode: a.Contract.Code,
		Currency:     a.Contract.Currency,
		Type:         SignalDivergence,
		Direction:    direction,
		Strength:     strength,
		Confidence:   confidence,
		Description:  fmt.Sprintf("Spec/Commercial divergence (%d weeks): Commercials %s while specs go opposite", consecutive, strings.ToLower(direction)),
		Factors:      factors,
	}
}

// detectMomentumShift identifies when positioning momentum changes direction.
func (sd *SignalDetector) detectMomentumShift(a domain.COTAnalysis, history []domain.COTRecord) *Signal {
	if len(history) < 4 {
		return nil
	}

	// Compute current vs previous 4-week momentum
	currentMom := a.SpecMomentum4W

	// Calculate previous week's momentum for comparison
	if len(history) < 6 {
		return nil
	}
	prevNets := extractNetsFloat(history[1:min(6, len(history))], func(r domain.COTRecord) float64 {
		return r.GetSmartMoneyNet(a.Contract.ReportType)
	})
	prevMom := mathutil.Momentum(reverseFloats(prevNets), 4)

	// Detect sign change (momentum flip)
	if (currentMom > 0 && prevMom > 0) || (currentMom < 0 && prevMom < 0) || currentMom == 0 {
		return nil
	}

	direction := "BULLISH"
	if currentMom < 0 {
		direction = "BEARISH"
	}

	magnitude := math.Abs(currentMom - prevMom)
	strength := 3
	if magnitude > 20000 {
		strength = 5
	} else if magnitude > 10000 {
		strength = 4
	}

	confidence := mathutil.Clamp(magnitude/300, 40, 85)

	// 8W momentum as higher-timeframe confirmation
	if a.SpecMomentum8W != 0 {
		m8Confirms := (currentMom > 0 && a.SpecMomentum8W > 0) || (currentMom < 0 && a.SpecMomentum8W < 0)
		if m8Confirms {
			strength = min(strength+1, 5)
			confidence = mathutil.Clamp(confidence+10, 40, 95)
		} else {
			// 8W opposes — reduce confidence
			confidence = mathutil.Clamp(confidence-15, 30, 85)
		}
	}

	factors := []string{
		fmt.Sprintf("Momentum flipped from %s to %s", fmtutil.FmtNumSigned(prevMom, 0), fmtutil.FmtNumSigned(currentMom, 0)),
		fmt.Sprintf("Magnitude of shift: %s", fmtutil.FmtNum(magnitude, 0)),
		fmt.Sprintf("Spec net position: %s", fmtutil.FmtNumSigned(a.NetPosition, 0)),
	}

	return &Signal{
		ContractCode: a.Contract.Code,
		Currency:     a.Contract.Currency,
		Type:         SignalMomentumShift,
		Direction:    direction,
		Strength:     strength,
		Confidence:   confidence,
		Description:  fmt.Sprintf("Momentum shift to %s: 4W spec momentum flipped sign", direction),
		Factors:      factors,
	}
}

// detectConcentrationRisk flags when top traders hold unusually large positions.
func (sd *SignalDetector) detectConcentrationRisk(a domain.COTAnalysis) *Signal {
	// Top4 concentration > 50% is concerning
	if a.Top4Concentration < 50 {
		return nil
	}

	// High concentration = potential for sharp reversal
	direction := "BEARISH" // default: concentrated long = risk of unwind
	if a.NetPosition < 0 {
		direction = "BULLISH" // concentrated short = risk of short squeeze
	}

	strength := 3
	if a.Top4Concentration > 65 {
		strength = 5
	} else if a.Top4Concentration > 55 {
		strength = 4
	}

	confidence := mathutil.Clamp(a.Top4Concentration-20, 30, 80)

	factors := []string{
		fmt.Sprintf("Top 4 traders: %.1f%% of OI", a.Top4Concentration),
		fmt.Sprintf("Top 8 traders: %.1f%% of OI", a.Top8Concentration),
		fmt.Sprintf("Spec net: %s", fmtutil.FmtNumSigned(a.NetPosition, 0)),
	}

	return &Signal{
		ContractCode: a.Contract.Code,
		Currency:     a.Contract.Currency,
		Type:         SignalConcentration,
		Direction:    direction,
		Strength:     strength,
		Confidence:   confidence,
		Description:  fmt.Sprintf("Concentration risk: Top 4 hold %.0f%% of OI — vulnerable to %s", a.Top4Concentration, map[string]string{"BEARISH": "unwind", "BULLISH": "squeeze"}[direction]),
		Factors:      factors,
	}
}

// detectCrowdContrarian flags when small speculators are extremely crowded.
func (sd *SignalDetector) detectCrowdContrarian(a domain.COTAnalysis) *Signal {
	if a.CrowdingIndex < 70 {
		return nil
	}

	// Crowd is wrong at extremes — contrarian signal
	direction := "BEARISH"
	if a.NetSmallSpec < 0 {
		direction = "BULLISH" // crowd is short = contrarian buy
	}

	strength := 3
	if a.CrowdingIndex > 85 {
		strength = 5
	} else if a.CrowdingIndex > 75 {
		strength = 4
	}

	confidence := mathutil.Clamp(a.CrowdingIndex-30, 40, 85)

	factors := []string{
		fmt.Sprintf("Crowding index: %.1f (extreme)", a.CrowdingIndex),
		fmt.Sprintf("Small spec net: %s", fmtutil.FmtNumSigned(a.NetSmallSpec, 0)),
		fmt.Sprintf("Small spec signal: %s", a.SmallSpecSignal),
	}

	return &Signal{
		ContractCode: a.Contract.Code,
		Currency:     a.Contract.Currency,
		Type:         SignalCrowdContrarian,
		Direction:    direction,
		Strength:     strength,
		Confidence:   confidence,
		Description:  fmt.Sprintf("Crowd contrarian %s: Small specs crowded (%.0f) — fading the crowd", direction, a.CrowdingIndex),
		Factors:      factors,
	}
}

// detectThinMarket flags when a key trader category has very few participants.
// Thin markets are prone to sharp reversals when even a single large trader exits.
func (sd *SignalDetector) detectThinMarket(a domain.COTAnalysis) *Signal {
	if !a.ThinMarketAlert || a.ThinMarketDesc == "" {
		return nil
	}

	// Direction: thin market in dominant side = reversal risk
	direction := "BEARISH"
	if a.NetPosition < 0 {
		direction = "BULLISH" // thin shorts = squeeze risk
	}

	strength := 3
	// Very thin (< 10 traders in key category) = higher severity
	minTraders := a.TotalTraders
	if a.Contract.ReportType == "TFF" {
		if a.DealerShortTraders > 0 && a.DealerShortTraders < minTraders {
			minTraders = a.DealerShortTraders
		}
		if a.LevFundLongTraders > 0 && a.LevFundLongTraders < minTraders {
			minTraders = a.LevFundLongTraders
		}
	} else {
		if a.MMoneyLongTraders > 0 && a.MMoneyLongTraders < minTraders {
			minTraders = a.MMoneyLongTraders
		}
		if a.MMoneyShortTraders > 0 && a.MMoneyShortTraders < minTraders {
			minTraders = a.MMoneyShortTraders
		}
	}
	if minTraders < 10 {
		strength = 5
	} else if minTraders < 12 {
		strength = 4
	}

	confidence := mathutil.Clamp(float64(100-minTraders*5), 40, 90)

	factors := []string{
		a.ThinMarketDesc,
		fmt.Sprintf("Total traders: %d (%s)", a.TotalTraders, a.TraderConcentration),
		fmt.Sprintf("Net position: %s", fmtutil.FmtNumSigned(a.NetPosition, 0)),
	}

	return &Signal{
		ContractCode: a.Contract.Code,
		Currency:     a.Contract.Currency,
		Type:         SignalThinMarket,
		Direction:    direction,
		Strength:     strength,
		Confidence:   confidence,
		Description:  fmt.Sprintf("Thin market %s: %s — reversal risk elevated", direction, a.ThinMarketDesc),
		Factors:      factors,
	}
}

// FormatSignals creates a Telegram-formatted signal summary.
func FormatSignals(signals []Signal) string {
	if len(signals) == 0 {
		return "No actionable COT signals detected."
	}

	var b strings.Builder
	b.WriteString("=== COT SIGNALS ===")

	for i, s := range signals {
		if i >= 10 {
			b.WriteString(fmt.Sprintf("\n... and %d more signals", len(signals)-10))
			break
		}

		strengthBar := strings.Repeat("|", s.Strength)
		dirIcon := "^" // bullish
		if s.Direction == "BEARISH" {
			dirIcon = "v" // bearish
		}

		b.WriteString(fmt.Sprintf("\n\n%s %s %s [%s]\n",
			dirIcon, s.Currency, s.Type, strengthBar))
		b.WriteString(fmt.Sprintf("%s\n", s.Description))
		b.WriteString(fmt.Sprintf("Confidence: %.0f%%\n", s.Confidence))

		for _, f := range s.Factors {
			b.WriteString(fmt.Sprintf("  - %s\n", f))
		}
	}

	return b.String()
}

// sortSignals sorts by strength descending, then confidence descending.
func sortSignals(signals []Signal) {
	for i := 1; i < len(signals); i++ {
		for j := i; j > 0; j-- {
			if signals[j].Strength > signals[j-1].Strength ||
				(signals[j].Strength == signals[j-1].Strength && signals[j].Confidence > signals[j-1].Confidence) {
				signals[j], signals[j-1] = signals[j-1], signals[j]
			} else {
				break
			}
		}
	}
}
