package fred

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// CarryMonitor extends the rate differential engine with daily tracking
// and carry unwind detection.
type CarryMonitor struct {
	engine *RateDifferentialEngine

	mu        sync.Mutex
	prevRange float64 // stores previous spread range for week-over-week comparison
}

var (
	globalCarryMonitor *CarryMonitor
	carryOnce          sync.Once
)

// GetCarryMonitor returns the package-level CarryMonitor singleton.
func GetCarryMonitor() *CarryMonitor {
	carryOnce.Do(func() {
		globalCarryMonitor = &CarryMonitor{
			engine: NewRateDifferentialEngine(),
		}
	})
	return globalCarryMonitor
}

// FetchCarryDashboard fetches current rates, computes carry snapshots per pair,
// ranks them by attractiveness, and detects unwind risk.
func (m *CarryMonitor) FetchCarryDashboard(ctx context.Context) (*domain.CarryMonitorResult, error) {
	ranking, err := m.engine.FetchCarryRanking(ctx)
	if err != nil {
		return nil, err
	}

	var pairs []domain.CarryPairSnapshot
	for _, rd := range ranking.Pairs {
		spreadBps := rd.Differential * 100 // convert percentage points to basis points
		dailyBps := spreadBps / 365.0

		pairs = append(pairs, domain.CarryPairSnapshot{
			Currency:     rd.Currency,
			Spread:       roundN(spreadBps, 1),
			DailyAccrual: roundN(dailyBps, 2),
			Direction:    rd.Direction,
		})
	}

	// Sort by spread descending (most attractive carry first)
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Spread > pairs[j].Spread
	})

	// Compute spread range: max - min across all pairs
	spreadRange := computeSpreadRange(pairs)

	m.mu.Lock()
	prevRange := m.prevRange

	// Detect unwind risk
	rangeChange := 0.0
	if prevRange > 0 {
		rangeChange = (spreadRange - prevRange) / prevRange * 100
	}
	risk := classifyUnwindRisk(spreadRange, prevRange, rangeChange)

	// Update previous range for next comparison
	m.prevRange = spreadRange
	m.mu.Unlock()

	result := &domain.CarryMonitorResult{
		Pairs:       pairs,
		SpreadRange: roundN(spreadRange, 1),
		PrevRange:   roundN(prevRange, 1),
		RangeChange: roundN(rangeChange, 1),
		Risk:        risk,
		AsOf:        time.Now().Format("2006-01-02"),
	}

	if len(pairs) > 0 {
		result.BestCarry = pairs[0].Currency
		result.WorstCarry = pairs[len(pairs)-1].Currency
	}

	return result, nil
}

// CheckCarryAlerts compares two CarryMonitorResult snapshots and returns alerts
// if the carry unwind risk has escalated.
func CheckCarryAlerts(current, previous *domain.CarryMonitorResult) []MacroAlert {
	if current == nil || previous == nil {
		return nil
	}

	var alerts []MacroAlert

	// Escalation: NORMAL → NARROWING
	if previous.Risk == domain.UnwindNormal && current.Risk == domain.UnwindNarrow {
		alerts = append(alerts, MacroAlert{
			Type:  AlertCarryNarrowing,
			Title: "⚠️ Carry Trade Spreads NARROWING",
			Description: fmt.Sprintf(
				"Carry spread range compressing: %.0fbps → %.0fbps (%.1f%% change). "+
					"Best carry: %s, Worst: %s. "+
					"Early warning — monitor for further compression.",
				previous.SpreadRange, current.SpreadRange, current.RangeChange,
				current.BestCarry, current.WorstCarry),
			Severity: "MEDIUM",
			Value:    current.SpreadRange,
			Previous: previous.SpreadRange,
		})
	}

	// Escalation to UNWIND (from any previous state)
	if current.Risk == domain.UnwindAlert && previous.Risk != domain.UnwindAlert {
		alerts = append(alerts, MacroAlert{
			Type:  AlertCarryUnwind,
			Title: "🔴 Carry Trade UNWIND Alert",
			Description: fmt.Sprintf(
				"Carry spread range collapsed: %.0fbps → %.0fbps (%.1f%% change). "+
					"Best carry: %s, Worst: %s. "+
					"Carry unwind in progress — expect JPY/CHF strengthening, EM currency stress.",
				previous.SpreadRange, current.SpreadRange, current.RangeChange,
				current.BestCarry, current.WorstCarry),
			Severity: "HIGH",
			Value:    current.SpreadRange,
			Previous: previous.SpreadRange,
		})
	}

	return alerts
}

// computeSpreadRange returns the difference between the highest and lowest carry spreads.
func computeSpreadRange(pairs []domain.CarryPairSnapshot) float64 {
	if len(pairs) == 0 {
		return 0
	}
	maxSpread := -math.MaxFloat64
	minSpread := math.MaxFloat64
	for _, p := range pairs {
		if p.Spread > maxSpread {
			maxSpread = p.Spread
		}
		if p.Spread < minSpread {
			minSpread = p.Spread
		}
	}
	return maxSpread - minSpread
}

// classifyUnwindRisk determines the carry unwind danger level.
//
// Logic:
//   - NORMAL: spread range is healthy and stable
//   - NARROWING: spread range compressing (>15% decline) — early warning
//   - UNWIND: spread range collapsing (>30% decline) or absolute range < 50bps — danger
func classifyUnwindRisk(currentRange, prevRange, rangeChangePct float64) domain.UnwindRisk {
	// If spread range is very tight, that alone signals unwind risk
	if currentRange < 50 {
		return domain.UnwindAlert
	}

	// No previous data to compare — assume normal
	if prevRange <= 0 {
		return domain.UnwindNormal
	}

	// Range collapsing > 30% = unwind alert
	if rangeChangePct < -30 {
		return domain.UnwindAlert
	}

	// Range narrowing > 15% = early warning
	if rangeChangePct < -15 {
		return domain.UnwindNarrow
	}

	return domain.UnwindNormal
}
