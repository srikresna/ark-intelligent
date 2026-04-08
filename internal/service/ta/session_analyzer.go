// Package ta — session_analyzer.go
//
// Trading Session Analysis Engine: classifies London/NY/Tokyo session behavior
// per currency pair and recommends a strategy (breakout vs mean-reversion).
//
// Sessions (UTC):
//   - Tokyo/Asia:  00:00 – 09:00
//   - London:      08:00 – 17:00
//   - New York:    13:00 – 22:00
//   - Overlap:     13:00 – 17:00 (London–NY)
//
// For each session, the engine computes a rolling 20-session ADX average,
// range (pip-equivalent), volatility, and % time trending vs ranging, then
// classifies the session character and recommends a strategy.
package ta

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// ---------------------------------------------------------------------------
// Constants & session windows
// ---------------------------------------------------------------------------

const (
	SessionTokyo   = "TOKYO"
	SessionLondon  = "LONDON"
	SessionNewYork = "NEW_YORK"
	SessionOverlap = "LONDON_NY_OVERLAP"
	SessionOff     = "OFF_HOURS"

	// Minimum bars needed to classify a session window.
	minSessionBars = 3
	// Rolling window: last N completed sessions used for statistics.
	sessionRollingN = 20
	// ADX thresholds.
	adxTrendingThreshold = 25.0
	adxRangingThreshold  = 20.0
)

// sessionWindow defines a trading session with UTC hour boundaries.
type sessionWindow struct {
	name      string
	label     string
	startHour int // inclusive
	endHour   int // exclusive
}

// standardSessions lists the three main sessions.
var standardSessions = []sessionWindow{
	{SessionTokyo, "Tokyo/Asia", 0, 9},
	{SessionLondon, "London", 8, 17},
	{SessionNewYork, "New York", 13, 22},
}

// ---------------------------------------------------------------------------
// Output types
// ---------------------------------------------------------------------------

// SessionCharacter describes the dominant behavior observed in a session.
type SessionCharacter string

const (
	CharTrending SessionCharacter = "TRENDING"
	CharRanging  SessionCharacter = "RANGING"
	CharVolatile SessionCharacter = "VOLATILE"
	CharCalm     SessionCharacter = "CALM"
	CharMixed    SessionCharacter = "MIXED"
)

// SessionStrategy is the recommended approach for a session.
type SessionStrategy string

const (
	StratBreakout   SessionStrategy = "BREAKOUT"
	StratMeanRevert SessionStrategy = "MEAN_REVERSION"
	StratTrend      SessionStrategy = "TREND_FOLLOWING"
	StratNeutral    SessionStrategy = "NEUTRAL"
)

// SessionStats holds computed statistics for one session over the rolling window.
type SessionStats struct {
	Session       string           // e.g. "LONDON"
	Label         string           // e.g. "London"
	ADXAvg        float64          // average ADX across sessions
	RangeAvgPips  float64          // average H-L range (pip units × 10000)
	VolatilityPct float64          // avg bar-to-bar % change (annualized proxy)
	PctTrending   float64          // % of sessions where ADX > 25
	PctRanging    float64          // % of sessions where ADX < 20
	Character     SessionCharacter // dominant character
	Strategy      SessionStrategy  // recommended strategy
	SampleCount   int              // number of sessions analyzed
}

// SessionAnalysisResult holds analysis for all sessions + current context.
type SessionAnalysisResult struct {
	Currency       string
	CurrentSession string // which session is active right now
	CurrentTime    time.Time
	MinUntilNext   int // minutes until next session boundary
	NextSession    string

	Tokyo   *SessionStats
	London  *SessionStats
	NewYork *SessionStats

	Available bool
}

// ---------------------------------------------------------------------------
// SessionAnalyzer
// ---------------------------------------------------------------------------

// SessionAnalyzer computes session behavior statistics from intraday OHLCV bars.
type SessionAnalyzer struct{}

// NewSessionAnalyzer creates a SessionAnalyzer.
func NewSessionAnalyzer() *SessionAnalyzer {
	return &SessionAnalyzer{}
}

// AnalyzeWithStore fetches 1H bars via the provided store and computes stats.
// bars: OHLCV slice newest-first (Date is bar open time in UTC).
func (sa *SessionAnalyzer) Analyze(_ context.Context, currency string, bars []OHLCV) (*SessionAnalysisResult, error) {
	if len(bars) < 10 {
		return nil, fmt.Errorf("insufficient intraday bars for %s (%d)", currency, len(bars))
	}

	now := time.Now().UTC()
	cur, minUntilNext, nextSess := classifyCurrentSession(now)

	result := &SessionAnalysisResult{
		Currency:       currency,
		CurrentSession: cur,
		CurrentTime:    now,
		MinUntilNext:   minUntilNext,
		NextSession:    nextSess,
		Available:      true,
	}

	for _, sw := range standardSessions {
		stats := sa.computeSessionStats(bars, sw)
		switch sw.name {
		case SessionTokyo:
			result.Tokyo = stats
		case SessionLondon:
			result.London = stats
		case SessionNewYork:
			result.NewYork = stats
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Internal: session classification
// ---------------------------------------------------------------------------

func classifyCurrentSession(t time.Time) (active string, minUntilNext int, next string) {
	h := t.Hour()
	m := t.Minute()
	totalMin := h*60 + m

	// Overlap takes precedence
	if totalMin >= 13*60 && totalMin < 17*60 {
		active = SessionOverlap
		minUntilNext = 17*60 - totalMin
		next = SessionNewYork // NY continues
		return
	}

	for _, sw := range standardSessions {
		if totalMin >= sw.startHour*60 && totalMin < sw.endHour*60 {
			active = sw.name
			minUntilNext = sw.endHour*60 - totalMin
			// Find next session
			next = nextSessionAfter(sw.name)
			return
		}
	}

	active = SessionOff
	// Find next session start
	bestStart := 24*60 + standardSessions[0].startHour*60
	nextName := standardSessions[0].name
	for _, sw := range standardSessions {
		start := sw.startHour * 60
		if start > totalMin && start < bestStart {
			bestStart = start
			nextName = sw.name
		}
	}
	// Wrap to next day
	if bestStart == 24*60+standardSessions[0].startHour*60 {
		bestStart = standardSessions[0].startHour * 60
		nextName = standardSessions[0].name
	}
	if bestStart > totalMin {
		minUntilNext = bestStart - totalMin
	} else {
		// tomorrow
		minUntilNext = 24*60 - totalMin + bestStart
	}
	next = nextName
	return
}

func nextSessionAfter(current string) string {
	order := []string{SessionTokyo, SessionLondon, SessionOverlap, SessionNewYork, SessionTokyo}
	for i, s := range order {
		if s == current && i+1 < len(order) {
			return order[i+1]
		}
	}
	return SessionTokyo
}

// ---------------------------------------------------------------------------
// Internal: per-session statistics
// ---------------------------------------------------------------------------

// computeSessionStats slices bars into session-windows and computes stats.
func (sa *SessionAnalyzer) computeSessionStats(bars []OHLCV, sw sessionWindow) *SessionStats {
	stats := &SessionStats{
		Session: sw.name,
		Label:   sw.label,
	}

	// Group bars by session occurrence (by date + hour bracket)
	sessions := groupBySession(bars, sw)
	if len(sessions) == 0 {
		stats.Character = CharMixed
		stats.Strategy = StratNeutral
		return stats
	}

	// Keep last sessionRollingN sessions
	if len(sessions) > sessionRollingN {
		sessions = sessions[len(sessions)-sessionRollingN:]
	}

	stats.SampleCount = len(sessions)

	var adxSum, rangeSum, volSum float64
	trendCount := 0
	rangeCount := 0

	for _, sessionBars := range sessions {
		if len(sessionBars) < minSessionBars {
			continue
		}

		// Range (H - L across session bars)
		hi := sessionBars[0].High
		lo := sessionBars[0].Low
		for _, b := range sessionBars[1:] {
			if b.High > hi {
				hi = b.High
			}
			if b.Low < lo {
				lo = b.Low
			}
		}
		rangeN := (hi - lo) * 10000 // convert to pip-equivalent
		rangeSum += rangeN

		// Volatility proxy: average |bar pct change|
		for i := 1; i < len(sessionBars); i++ {
			if sessionBars[i].Close > 0 {
				pctChg := math.Abs((sessionBars[i-1].Close - sessionBars[i].Close) / sessionBars[i].Close * 100)
				volSum += pctChg
			}
		}

		// ADX of this session's bars
		adxResult := CalcADX(sessionBars, 7) // shorter period for intraday
		if adxResult != nil {
			adxSum += adxResult.ADX
			if adxResult.ADX > adxTrendingThreshold {
				trendCount++
			} else if adxResult.ADX < adxRangingThreshold {
				rangeCount++
			}
		}
	}

	n := float64(stats.SampleCount)
	stats.ADXAvg = adxSum / n
	stats.RangeAvgPips = rangeSum / n
	stats.VolatilityPct = volSum / n
	stats.PctTrending = float64(trendCount) / n * 100
	stats.PctRanging = float64(rangeCount) / n * 100

	stats.Character = classifyCharacter(stats)
	stats.Strategy = recommendStrategy(stats)

	return stats
}

// groupBySession groups bars by their session occurrence date.
// Returns oldest-first list of session bar groups.
func groupBySession(bars []OHLCV, sw sessionWindow) [][]OHLCV {
	// bars are newest-first; reverse for processing
	sorted := make([]OHLCV, len(bars))
	copy(sorted, bars)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date.Before(sorted[j].Date)
	})

	type sessionKey struct {
		year  int
		month time.Month
		day   int
	}

	dayMap := make(map[sessionKey][]OHLCV)
	for _, b := range sorted {
		h := b.Date.UTC().Hour()
		if h >= sw.startHour && h < sw.endHour {
			k := sessionKey{b.Date.Year(), b.Date.Month(), b.Date.Day()}
			dayMap[k] = append(dayMap[k], b)
		}
	}

	// Sort keys
	type keyedGroup struct {
		k    sessionKey
		bars []OHLCV
	}
	var groups []keyedGroup
	for k, brs := range dayMap {
		groups = append(groups, keyedGroup{k, brs})
	}
	sort.Slice(groups, func(i, j int) bool {
		ki, kj := groups[i].k, groups[j].k
		if ki.year != kj.year {
			return ki.year < kj.year
		}
		if ki.month != kj.month {
			return ki.month < kj.month
		}
		return ki.day < kj.day
	})

	result := make([][]OHLCV, len(groups))
	for i, g := range groups {
		result[i] = g.bars
	}
	return result
}

// classifyCharacter determines dominant session behavior from stats.
func classifyCharacter(s *SessionStats) SessionCharacter {
	if s.PctTrending > 50 {
		return CharTrending
	}
	if s.PctRanging > 50 {
		if s.VolatilityPct < 0.1 {
			return CharCalm
		}
		return CharRanging
	}
	if s.VolatilityPct > 0.3 {
		return CharVolatile
	}
	return CharMixed
}

// recommendStrategy maps session character to a trading strategy.
func recommendStrategy(s *SessionStats) SessionStrategy {
	switch s.Character {
	case CharTrending:
		return StratTrend
	case CharVolatile:
		return StratBreakout
	case CharRanging, CharCalm:
		return StratMeanRevert
	default:
		return StratNeutral
	}
}
