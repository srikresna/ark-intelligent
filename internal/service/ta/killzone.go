package ta

// killzone.go — ICT Killzone & Trading Session Classifier.
//
// Identifies the current trading session (killzone) based on UTC time:
//
//   - Asia Session:     00:00 – 04:00 UTC
//   - London Open:      07:00 – 10:00 UTC
//   - NY Open:          12:00 – 15:00 UTC
//   - London Close:     15:00 – 16:00 UTC
//   - NY Close:         20:00 – 22:00 UTC
//   - Off Hours:        everything else
//
// IntersessionOverlap = true during 13:00 – 16:00 UTC (London–NY overlap).

import "time"

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// KillzoneResult describes the current trading session and timing context.
type KillzoneResult struct {
	ActiveKillzone      string // "LONDON_OPEN", "NY_OPEN", "LONDON_CLOSE", "NY_CLOSE", "ASIA", "OFF_HOURS"
	IsActive            bool   // true if currently inside a killzone
	SessionDescription  string // human-readable description
	MinutesUntilNext    int    // minutes until the next killzone starts
	NextKillzone        string // name of the next killzone
	IntersessionOverlap bool   // true during 13:00–16:00 UTC (London–NY overlap)
}

// killzoneWindow defines a named trading session window (UTC hours).
type killzoneWindow struct {
	name        string
	startHour   int
	startMinute int
	endHour     int
	endMinute   int
	description string
}

// sessions defines all known killzone windows in UTC, ordered by start time.
var sessions = []killzoneWindow{
	{"ASIA", 0, 0, 4, 0, "Asia Session (Tokyo/Sydney)"},
	{"LONDON_OPEN", 7, 0, 10, 0, "London Open Killzone"},
	{"NY_OPEN", 12, 0, 15, 0, "New York Open Killzone"},
	{"LONDON_CLOSE", 15, 0, 16, 0, "London Close Killzone"},
	{"NY_CLOSE", 20, 0, 22, 0, "New York Close Killzone"},
}

// ---------------------------------------------------------------------------
// ClassifyKillzone
// ---------------------------------------------------------------------------

// ClassifyKillzone returns the ICT killzone classification for time t (UTC assumed).
func ClassifyKillzone(t time.Time) KillzoneResult {
	utc := t.UTC()
	totalMin := utc.Hour()*60 + utc.Minute()

	// Check if in an active session
	activeKZ := "OFF_HOURS"
	activeDesc := "Off-hours (low institutional activity)"
	isActive := false

	for _, s := range sessions {
		start := s.startHour*60 + s.startMinute
		end := s.endHour*60 + s.endMinute
		if totalMin >= start && totalMin < end {
			activeKZ = s.name
			activeDesc = s.description
			isActive = true
			break
		}
	}

	// Intersession overlap: London–NY 13:00–16:00 UTC
	isOverlap := totalMin >= 13*60 && totalMin < 16*60

	// Find next killzone
	nextName, nextIn := NextKillzoneInfo(t)

	return KillzoneResult{
		ActiveKillzone:      activeKZ,
		IsActive:            isActive,
		SessionDescription:  activeDesc,
		MinutesUntilNext:    nextIn,
		NextKillzone:        nextName,
		IntersessionOverlap: isOverlap,
	}
}

// ---------------------------------------------------------------------------
// NextKillzoneInfo
// ---------------------------------------------------------------------------

// NextKillzoneInfo returns the name and minutes until the next killzone begins.
// If currently inside a killzone, returns the NEXT one after the current ends.
func NextKillzoneInfo(t time.Time) (name string, startsIn int) {
	utc := t.UTC()
	totalMin := utc.Hour()*60 + utc.Minute()
	dayMin := 24 * 60

	// Collect all start times for today + wraparound for tomorrow
	type kzStart struct {
		name      string
		startMin  int
	}

	var starts []kzStart
	for _, s := range sessions {
		start := s.startHour*60 + s.startMinute
		starts = append(starts, kzStart{s.name, start})
		// Add tomorrow's occurrence for wraparound
		starts = append(starts, kzStart{s.name, start + dayMin})
	}

	// Find the earliest start time strictly after current time
	bestName := "ASIA"
	bestStart := dayMin + sessions[0].startHour*60 + sessions[0].startMinute + dayMin

	for _, ks := range starts {
		if ks.startMin > totalMin && ks.startMin < bestStart {
			bestStart = ks.startMin
			bestName = ks.name
		}
	}

	minutesUntil := bestStart - totalMin
	if minutesUntil >= dayMin {
		minutesUntil -= dayMin
	}

	return bestName, minutesUntil
}
