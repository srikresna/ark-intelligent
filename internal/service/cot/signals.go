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
type SignalDetector struct {
	MinStrength int // minimum strength to emit (0 = no filter, default)
}

// NewSignalDetector creates a signal detector.
func NewSignalDetector() *SignalDetector {
	return &SignalDetector{}
}

// NewSignalDetectorWithMinStrength creates a signal detector with a minimum strength filter.
func NewSignalDetectorWithMinStrength(minStrength int) *SignalDetector {
	return &SignalDetector{MinStrength: minStrength}
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
	SignalThinMarket      SignalType = "THIN_MARKET"
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
		if s := sd.detectExtreme(a, history); s != nil {
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

	// Filter by minimum strength if configured
	if sd.MinStrength > 0 {
		filtered := signals[:0]
		for _, s := range signals {
			if s.Strength >= sd.MinStrength {
				filtered = append(filtered, s)
			}
		}
		signals = filtered
	}

	return signals
}

// detectSmartMoney identifies when commercial hedgers are making significant moves.
//
// Interpretation differs by report type:
//   - DISAGGREGATED (commodities): Prod/Swap are true hedgers.
//     High COTIndexComm (>80) = less hedging = BULLISH for price.
//     Low COTIndexComm (<20) = heavy hedging = BEARISH for price.
//   - TFF (forex/indices): Dealers are market makers with inventory-driven positioning.
//     High COTIndexComm (>80) = dealers forced long by client selling = BEARISH.
//     Low COTIndexComm (<20) = dealers forced short by client buying = BULLISH.
func (sd *SignalDetector) detectSmartMoney(a domain.COTAnalysis, history []domain.COTRecord) *Signal {
	commChangeAbs := math.Abs(a.CommNetChange)

	// Use relative threshold (% of open interest) instead of absolute contracts.
	// 5000 contracts in EUR (large market) is noise; 5000 in NZD is huge.
	if a.OpenInterest > 0 {
		changePct := commChangeAbs / a.OpenInterest * 100
		if changePct < 2.0 {
			return nil // less than 2% of OI — not significant
		}
	} else if commChangeAbs < 5000 {
		return nil // fallback absolute threshold
	}

	// Check if commercial index is at extreme
	if a.COTIndexComm < 20 || a.COTIndexComm > 80 {
		// Direction logic differs by report type
		var direction string
		if a.Contract.ReportType == "TFF" {
			// TFF: Dealers are market makers, not directional traders.
			// Dealers forced long = clients sold heavily = BEARISH for price.
			// Dealers forced short = clients bought heavily = BULLISH for price.
			direction = "BEARISH"
			if a.COTIndexComm < 20 {
				direction = "BULLISH"
			}
		} else {
			// DISAGGREGATED: Producers/merchants are true hedgers.
			// High COTIndexComm = less hedging (bullish outlook) = BULLISH.
			// Low COTIndexComm = heavy hedging (bearish outlook) = BEARISH.
			direction = "BULLISH"
			if a.COTIndexComm < 20 {
				direction = "BEARISH"
			}
		}

		// Strength based on relative magnitude
		var changePct float64
		if a.OpenInterest > 0 {
			changePct = commChangeAbs / a.OpenInterest * 100
		}
		strength := 3
		if changePct > 6.0 || commChangeAbs > 15000 {
			strength = 5
		} else if changePct > 4.0 || commChangeAbs > 10000 {
			strength = 4
		}

		confidence := mathutil.Clamp(commChangeAbs/200, 30, 95)

		factors := []string{
			fmt.Sprintf("Commercial net change: %s", fmtutil.FmtNumSigned(a.CommNetChange, 0)),
			fmt.Sprintf("Commercial COT Index: %.1f", a.COTIndexComm),
			fmt.Sprintf("Report type: %s", a.Contract.ReportType),
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

// detectExtreme identifies when positioning reaches historically extreme levels
// AND shows signs of reverting. Fires only when the extreme is starting to unwind,
// not when it's still building — catching the turn, not the continuation.
func (sd *SignalDetector) detectExtreme(a domain.COTAnalysis, history []domain.COTRecord) *Signal {
	// Check COT Index extremes
	if a.COTIndex > 10 && a.COTIndex < 90 {
		return nil
	}

	// Require reversal confirmation: previous week must also have been extreme,
	// and current week must be LESS extreme (pulling back toward center).
	if len(history) < 2 {
		return nil
	}

	rt := a.Contract.ReportType
	// Compute previous week's COT Index from history
	prevNets := make([]float64, 0, len(history)-1)
	for _, r := range history[1:] {
		prevNets = append(prevNets, r.GetSmartMoneyNet(rt))
	}
	if len(prevNets) < 3 {
		return nil
	}

	prevIdx := computeCOTIndex(prevNets)

	isReverting := false
	if a.COTIndex <= 10 {
		// Bearish extreme — reverting if prev was also extreme AND more extreme
		if prevIdx <= 10 && prevIdx < a.COTIndex {
			isReverting = true // e.g., was 5, now 8 = starting to revert up
		}
	} else if a.COTIndex >= 90 {
		// Bullish extreme — reverting if prev was also extreme AND more extreme
		if prevIdx >= 90 && prevIdx > a.COTIndex {
			isReverting = true // e.g., was 95, now 92 = starting to revert down
		}
	}

	if !isReverting {
		return nil // still building extreme or first touch — wait for turn
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
		fmt.Sprintf("Speculator COT Index: %.1f (extreme, reverting from %.1f)", a.COTIndex, prevIdx),
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
		Description:  fmt.Sprintf("Extreme positioning reverting: Spec COT Index at %.0f (contrarian %s)", a.COTIndex, direction),
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
			// Require meaningful magnitude — filter out noise-level divergences.
			minChange := 2000.0
			if a.OpenInterest > 0 {
				minChange = a.OpenInterest * 0.005 // 0.5% of OI
			}
			if (specChg > minChange && commChg < -minChange) ||
				(specChg < -minChange && commChg > minChange) {
				consecutive++
			} else {
				break
			}
		}
	}

	if consecutive < 3 {
		return nil // require 3+ weeks of persistent divergence
	}

	// Direction follows commercials (smart money)
	direction := "BULLISH"
	if a.CommNetChange < 0 {
		direction = "BEARISH"
	}

	strength := min(consecutive+1, 5)
	confidence := mathutil.Clamp(float64(consecutive)*15+10, 40, 90)

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

// detectMomentumShift identifies when positioning momentum changes direction
// with sufficient magnitude and higher-timeframe confirmation.
func (sd *SignalDetector) detectMomentumShift(a domain.COTAnalysis, history []domain.COTRecord) *Signal {
	if len(history) < 6 {
		return nil
	}

	currentMom := a.SpecMomentum4W

	prevNets := extractNetsFloat(history[1:min(6, len(history))], func(r domain.COTRecord) float64 {
		return r.GetSmartMoneyNet(a.Contract.ReportType)
	})
	prevMom := mathutil.Momentum(reverseFloats(prevNets), 4)

	// Detect sign change (momentum flip)
	if (currentMom > 0 && prevMom > 0) || (currentMom < 0 && prevMom < 0) || currentMom == 0 {
		return nil
	}

	magnitude := math.Abs(currentMom - prevMom)

	// Require minimum magnitude relative to OI to filter noise in ranging markets.
	// Every small sign flip in a ranging market was generating signals (731 at 50% WR).
	if a.OpenInterest > 0 {
		magnitudePct := magnitude / a.OpenInterest * 100
		if magnitudePct < 1.0 {
			return nil // less than 1% of OI shift — noise
		}
	}

	// Require 8W momentum confirmation — not just a boost.
	// 4W flip without 8W agreement is usually noise, not a real trend change.
	if a.SpecMomentum8W == 0 {
		return nil // no 8W data available
	}
	m8Confirms := (currentMom > 0 && a.SpecMomentum8W > 0) || (currentMom < 0 && a.SpecMomentum8W < 0)
	if !m8Confirms {
		return nil // 4W and 8W disagree — likely noise
	}

	direction := "BULLISH"
	if currentMom < 0 {
		direction = "BEARISH"
	}

	strength := 3
	if magnitude > 20000 {
		strength = 5
	} else if magnitude > 10000 {
		strength = 4
	}

	confidence := mathutil.Clamp(magnitude/300, 40, 95)

	factors := []string{
		fmt.Sprintf("Momentum flipped from %s to %s", fmtutil.FmtNumSigned(prevMom, 0), fmtutil.FmtNumSigned(currentMom, 0)),
		fmt.Sprintf("Magnitude of shift: %s", fmtutil.FmtNum(magnitude, 0)),
		fmt.Sprintf("8W momentum confirms: %s", fmtutil.FmtNumSigned(a.SpecMomentum8W, 0)),
	}

	return &Signal{
		ContractCode: a.Contract.Code,
		Currency:     a.Contract.Currency,
		Type:         SignalMomentumShift,
		Direction:    direction,
		Strength:     strength,
		Confidence:   confidence,
		Description:  fmt.Sprintf("Momentum shift to %s: 4W+8W spec momentum aligned", direction),
		Factors:      factors,
	}
}

// detectConcentrationRisk flags when top traders hold unusually large positions.
func (sd *SignalDetector) detectConcentrationRisk(a domain.COTAnalysis) *Signal {
	// Top4 concentration > 55% is concerning
	if a.Top4Concentration < 55 {
		return nil
	}

	// Direction based on which side is more concentrated
	direction := "BEARISH" // concentrated longs = unwind risk → price drops
	if a.Top4ShortPct > a.Top4LongPct {
		direction = "BULLISH" // concentrated shorts = squeeze risk → price rises
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

// detectCrowdContrarian flags when large speculators are extremely one-sided.
//
// CrowdingIndex measures how one-sided large spec (LevFund/ManagedMoney) positioning is.
// Direction must be based on the SAME group — large spec net position (NetPosition) —
// not small spec positioning, which is a different group that may disagree.
//
// Contrarian logic: if large specs are crowded long → BEARISH (fade the crowd).
//
//	if large specs are crowded short → BULLISH (fade the crowd).
func (sd *SignalDetector) detectCrowdContrarian(a domain.COTAnalysis) *Signal {
	if a.CrowdingIndex < 70 {
		return nil
	}

	// Direction based on the SAME group CrowdingIndex measures (large specs).
	// NetPosition = LevFundNet (TFF) or ManagedMoneyNet (DISAGG) — the crowded group.
	direction := "BEARISH" // large specs crowded long → contrarian sell
	if a.NetPosition < 0 {
		direction = "BULLISH" // large specs crowded short → contrarian buy
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
		fmt.Sprintf("Large spec net: %s", fmtutil.FmtNumSigned(a.NetPosition, 0)),
		fmt.Sprintf("Small spec net: %s (ref)", fmtutil.FmtNumSigned(a.NetSmallSpec, 0)),
	}

	return &Signal{
		ContractCode: a.Contract.Code,
		Currency:     a.Contract.Currency,
		Type:         SignalCrowdContrarian,
		Direction:    direction,
		Strength:     strength,
		Confidence:   confidence,
		Description:  fmt.Sprintf("Crowd contrarian %s: Large specs crowded (%.0f) — fading the crowd", direction, a.CrowdingIndex),
		Factors:      factors,
	}
}

// detectThinMarket flags when a key trader category has very few participants.
// Thin markets are prone to sharp reversals when even a single large trader exits.
//
// Direction is based on which SIDE is thin (not overall net position):
//   - Thin longs → BEARISH (unwind risk if few longs exit)
//   - Thin shorts → BULLISH (squeeze risk if few shorts cover)
func (sd *SignalDetector) detectThinMarket(a domain.COTAnalysis) *Signal {
	if !a.ThinMarketAlert || a.ThinMarketDesc == "" {
		return nil
	}

	// Determine which side is thin and derive direction from it.
	// Direction should match the risk: thin longs = bearish (unwind), thin shorts = bullish (squeeze).
	direction := ""
	minTraders := a.TotalTraders
	thinSide := ""

	if a.Contract.ReportType == "TFF" {
		if a.LevFundLongTraders > 0 && a.LevFundLongTraders < 10 {
			direction = "BEARISH" // thin longs → unwind risk
			minTraders = a.LevFundLongTraders
			thinSide = "lev fund longs"
		}
		if a.LevFundShortTraders > 0 && a.LevFundShortTraders < 10 {
			if direction == "" || a.LevFundShortTraders < minTraders {
				direction = "BULLISH" // thin shorts → squeeze risk
				minTraders = a.LevFundShortTraders
				thinSide = "lev fund shorts"
			}
		}
		if a.DealerShortTraders > 0 && a.DealerShortTraders < 10 {
			if direction == "" || a.DealerShortTraders < minTraders {
				direction = "BULLISH" // thin dealer shorts → squeeze risk
				minTraders = a.DealerShortTraders
				thinSide = "dealer shorts"
			}
		}
	} else {
		if a.MMoneyLongTraders > 0 && a.MMoneyLongTraders < 10 {
			direction = "BEARISH" // thin longs → unwind risk
			minTraders = a.MMoneyLongTraders
			thinSide = "managed money longs"
		}
		if a.MMoneyShortTraders > 0 && a.MMoneyShortTraders < 10 {
			if direction == "" || a.MMoneyShortTraders < minTraders {
				direction = "BULLISH" // thin shorts → squeeze risk
				minTraders = a.MMoneyShortTraders
				thinSide = "managed money shorts"
			}
		}
	}

	if direction == "" {
		return nil // no category thin enough at <10 threshold
	}

	strength := 4
	if minTraders < 7 {
		strength = 5
	}

	confidence := mathutil.Clamp(float64(100-minTraders*8), 40, 90)

	factors := []string{
		fmt.Sprintf("Thin side: %s (%d traders)", thinSide, minTraders),
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

// FormatBias creates a Telegram-formatted bias summary.
func FormatBias(signals []Signal) string {
	if len(signals) == 0 {
		return "No actionable COT biases detected."
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
