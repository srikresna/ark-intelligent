// Package domain defines core business entities with ZERO external dependencies.
// These types are shared across all layers: services, adapters, and ports.
package domain

import "time"

// ---------------------------------------------------------------------------
// Impact Level
// ---------------------------------------------------------------------------

// ImpactLevel represents the market impact severity of an economic event.
type ImpactLevel int

const (
	ImpactNone   ImpactLevel = 0
	ImpactLow    ImpactLevel = 1
	ImpactMedium ImpactLevel = 2
	ImpactHigh   ImpactLevel = 3
)

// String returns the human-readable impact label.
func (i ImpactLevel) String() string {
	switch i {
	case ImpactHigh:
		return "High"
	case ImpactMedium:
		return "Medium"
	case ImpactLow:
		return "Low"
	default:
		return "None"
	}
}

// Weight returns the numeric weight for quantitative calculations.
func (i ImpactLevel) Weight() float64 {
	return float64(i)
}

// ParseImpactLevel converts a string (e.g., "High", "Medium", "Low") to ImpactLevel.
func ParseImpactLevel(s string) ImpactLevel {
	switch s {
	case "High", "high", "HIGH":
		return ImpactHigh
	case "Medium", "medium", "MEDIUM", "Med", "med":
		return ImpactMedium
	case "Low", "low", "LOW":
		return ImpactLow
	default:
		return ImpactNone
	}
}

// ---------------------------------------------------------------------------
// Event Category
// ---------------------------------------------------------------------------

// EventCategory classifies what kind of economic event this is.
type EventCategory string

const (
	CategoryEconomicIndicator EventCategory = "INDICATOR"    // GDP, CPI, NFP, etc.
	CategoryCentralBank       EventCategory = "CENTRAL_BANK" // Rate decisions, minutes
	CategorySpeech            EventCategory = "SPEECH"       // Fed/ECB/BOE speeches
	CategoryAuction           EventCategory = "AUCTION"      // Bond auctions
	CategoryHoliday           EventCategory = "HOLIDAY"      // Market holidays
	CategoryOther             EventCategory = "OTHER"
)

// ---------------------------------------------------------------------------
// Release Type
// ---------------------------------------------------------------------------

// ReleaseType indicates whether data is preliminary, revised, or final.
type ReleaseType string

const (
	ReleasePreliminary ReleaseType = "PRELIMINARY" // Flash/Advance estimate
	ReleaseRevised     ReleaseType = "REVISED"     // Revised from earlier release
	ReleaseFinal       ReleaseType = "FINAL"       // Final/definitive release
	ReleaseRegular     ReleaseType = "REGULAR"     // Normal release (no qualifier)
)

// ---------------------------------------------------------------------------
// FFEvent — Core Economic Calendar Event
// ---------------------------------------------------------------------------

// FFEvent represents a single economic calendar event.
// Used by the legacy monolith (main.go) — kept for backward compatibility.
type FFEvent struct {
	// Core identification
	ID       string `json:"id"`       // Unique event ID (generated: {date}:{currency}:{title_hash})
	Title    string `json:"title"`    // Event name (e.g., "Non-Farm Employment Change")
	Currency string `json:"currency"` // 3-letter currency code (e.g., "USD")
	Country  string `json:"country"`  // Country name (e.g., "United States")

	// Timing
	Date     time.Time `json:"date"`       // Event date in WIB
	Time     string    `json:"time"`       // Original time string (e.g., "8:30pm", "All Day", "Tentative")
	IsAllDay bool      `json:"is_all_day"` // True for all-day events (holidays, etc.)

	// Impact & category
	Impact   ImpactLevel   `json:"impact"`
	Category EventCategory `json:"category"`

	// Data values
	Actual   string `json:"actual"`   // Actual released value (string to preserve formatting like "%")
	Forecast string `json:"forecast"` // Market consensus forecast
	Previous string `json:"previous"` // Previous period value

	Revision      *EventRevision `json:"revision,omitempty"` // Non-nil if Previous was revised
	ReleaseType   ReleaseType    `json:"release_type"`       // Preliminary/Revised/Final/Regular
	IsPreliminary bool           `json:"is_preliminary"`     // Flash/advance estimate
	IsFinal       bool           `json:"is_final"`           // Final release (no more revisions expected)

	SpeakerName string `json:"speaker_name,omitempty"` // e.g., "Powell", "Lagarde"
	SpeakerRole string `json:"speaker_role,omitempty"` // e.g., "Fed Chair", "ECB President"

	MultiDayGroup string `json:"multi_day_group,omitempty"` // Group ID for multi-day events (e.g., "FOMC-2024-03")
	MultiDayIndex int    `json:"multi_day_index,omitempty"` // Day number within group (1, 2, ...)

	// Source metadata
	SourceURL string `json:"source_url,omitempty"` // Link to FF detail page
	DetailURL string `json:"detail_url,omitempty"` // Link to historical data page

	// Scraping metadata
	ScrapedAt time.Time `json:"scraped_at"` // When this data was scraped
	Source    string    `json:"source"`     // "mql5", "manual"
}

// HasActual returns true if the actual value has been released.
func (e *FFEvent) HasActual() bool {
	return e.Actual != "" && e.Actual != "N/A"
}

// HasForecast returns true if a forecast exists.
func (e *FFEvent) HasForecast() bool {
	return e.Forecast != "" && e.Forecast != "N/A"
}

// IsHighImpact returns true for high-impact events.
func (e *FFEvent) IsHighImpact() bool {
	return e.Impact == ImpactHigh
}

// IsSpeech returns true if this is a speech/testimony event.
func (e *FFEvent) IsSpeech() bool {
	return e.Category == CategorySpeech
}

// IsUpcoming returns true if the event hasn't happened yet.
func (e *FFEvent) IsUpcoming() bool {
	return !e.HasActual() && e.Date.After(time.Now())
}

// WasRevised returns true if the previous value was revised.
func (e *FFEvent) WasRevised() bool {
	return e.Revision != nil
}

// ---------------------------------------------------------------------------
// FFEventDetail — Historical Data Point
// ---------------------------------------------------------------------------

// FFEventDetail represents a single historical data point for a recurring event.
// Used to build the historical dataset for surprise index calculations.
type FFEventDetail struct {
	EventName string    `json:"event_name"` // e.g., "Non-Farm Employment Change"
	Currency  string    `json:"currency"`   // e.g., "USD"
	Date      time.Time `json:"date"`       // Release date
	Actual    float64   `json:"actual"`     // Actual value (numeric)
	Forecast  float64   `json:"forecast"`   // Forecast value (numeric)
	Previous  float64   `json:"previous"`   // Previous period value (numeric)
	Revised   float64   `json:"revised"`    // Revised previous (0 if no revision)
	Surprise  float64   `json:"surprise"`   // Actual - Forecast
}

// HasRevision returns true if the previous value was revised for this data point.
func (d *FFEventDetail) HasRevision() bool {
	return d.Revised != 0 && d.Revised != d.Previous
}

// ---------------------------------------------------------------------------
// EventRevision — Tracks revisions to previous values
// ---------------------------------------------------------------------------

// RevisionDirection indicates whether a revision was upward or downward.
type RevisionDirection string

const (
	RevisionUp   RevisionDirection = "UP"
	RevisionDown RevisionDirection = "DOWN"
	RevisionFlat RevisionDirection = "FLAT"
)

// EventRevision records when a "Previous" value gets revised.
// Revision momentum is a leading indicator for economic trends.
type EventRevision struct {
	EventID       string            `json:"event_id,omitempty"` // Links to FFEvent.ID
	EventName     string            `json:"event_name"`
	Currency      string            `json:"currency"`
	Field         string            `json:"field,omitempty"` // "actual", "previous", "forecast", "status"
	RevisionDate  time.Time         `json:"revision_date"`   // When the revision was detected
	OriginalValue string            `json:"original_value"`  // Original "Previous" value
	RevisedValue  string            `json:"revised_value"`   // New revised value
	Direction     RevisionDirection `json:"direction"`       // UP, DOWN, or FLAT
	Magnitude     float64           `json:"magnitude"`       // Absolute change in numeric terms
}

// ---------------------------------------------------------------------------
// EventState — Alert tracking state per event
// ---------------------------------------------------------------------------

// EventState tracks which alerts have been sent for an event.
// Used by the alerter to avoid duplicate notifications.
type EventState struct {
	AlertedMinutes map[int]bool `json:"alerted_minutes"` // Minutes before event that were alerted
	ActualSent     bool         `json:"actual_sent"`     // Whether actual-release alert was sent
	RevisionSent   bool         `json:"revision_sent"`   // Whether revision alert was sent
}

// NewEventState creates a fresh EventState.
func NewEventState() *EventState {
	return &EventState{
		AlertedMinutes: make(map[int]bool),
	}
}

// ---------------------------------------------------------------------------
// AlertConfig — Alert subscription configuration
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// AlertConfig — Alert subscription configuration
// ---------------------------------------------------------------------------

// AlertConfig defines an alert subscription for a chat.
type AlertConfig struct {
	ChatID    int64       `json:"chat_id"`
	MinImpact ImpactLevel `json:"min_impact"` // Minimum impact level to alert
	Minutes   []int       `json:"minutes"`    // Minutes before event to alert
	Enabled   bool        `json:"enabled"`
}

// ---------------------------------------------------------------------------
// EventImpact — Records price impact of an economic release
// ---------------------------------------------------------------------------

// EventImpact records the price movement caused by an economic event release.
// Created after the actual value is published, with follow-up measurements
// at different time horizons (e.g., 1h, 4h after release).
type EventImpact struct {
	EventTitle  string    `json:"event_title"`   // e.g., "Non-Farm Employment Change"
	Currency    string    `json:"currency"`       // e.g., "USD"
	SigmaLevel  string   `json:"sigma_level"`    // Bucket: ">+2σ", "+1σ to +2σ", "-1σ to +1σ", "-1σ to -2σ", "<-2σ"
	PriceBefore float64  `json:"price_before"`   // Price at release time
	PriceAfter  float64  `json:"price_after"`    // Price at release + TimeHorizon
	PriceChange float64  `json:"price_change"`   // Change in pips
	PctChange   float64  `json:"pct_change"`     // Percentage change
	TimeHorizon string   `json:"time_horizon"`   // "1h" or "4h"
	Timestamp   time.Time `json:"timestamp"`     // Release timestamp
}

// EventImpactSummary aggregates historical impacts for a specific event
// and sigma bucket, providing average/median statistics.
type EventImpactSummary struct {
	EventTitle      string  `json:"event_title"`
	Currency        string  `json:"currency"`
	SigmaBucket     string  `json:"sigma_bucket"`      // e.g., ">+2σ", "+1σ to +2σ"
	AvgPriceImpactPips float64 `json:"avg_price_impact_pips"`
	AvgPctChange    float64 `json:"avg_pct_change"`
	Occurrences     int     `json:"occurrences"`
	MedianImpact    float64 `json:"median_impact"`     // Median pips change
}

// SigmaToBucket classifies a sigma value into a human-readable bucket string.
func SigmaToBucket(sigma float64) string {
	switch {
	case sigma >= 2.0:
		return ">+2σ"
	case sigma >= 1.0:
		return "+1σ to +2σ"
	case sigma > -1.0:
		return "-1σ to +1σ"
	case sigma > -2.0:
		return "-1σ to -2σ"
	default:
		return "<-2σ"
	}
}

// AllSigmaBuckets returns the ordered list of sigma buckets for display.
func AllSigmaBuckets() []string {
	return []string{">+2σ", "+1σ to +2σ", "-1σ to +1σ", "-1σ to -2σ", "<-2σ"}
}
