package ports

import (
	"context"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/service/fred"
)

// ---------------------------------------------------------------------------
// WeeklyData — Aggregated input for weekly outlook generation
// ---------------------------------------------------------------------------

// WeeklyData bundles all available data for AI weekly outlook generation.
type WeeklyData struct {
	COTAnalyses []domain.COTAnalysis `json:"cot_analyses"`
	NewsEvents  []domain.NewsEvent   `json:"news_events"`
	MacroData   *fred.MacroData      `json:"macro_data,omitempty"` // FRED macro data, optional
	Language    string               `json:"language"`             // "id" or "en"
}

// ---------------------------------------------------------------------------
// AIAnalyzer — Gemini AI interpretation interface
// ---------------------------------------------------------------------------

// AIAnalyzer defines the interface for AI-powered market analysis.
// Primary implementation uses Google Gemini API.
// Fallback: template-based interpretation (no AI required).
type AIAnalyzer interface {
	// AnalyzeCOT generates a narrative interpretation of COT positioning.
	// Input: latest COT analyses for all tracked contracts.
	// Output: 3-4 sentence institutional positioning narrative.
	AnalyzeCOT(ctx context.Context, analyses []domain.COTAnalysis) (string, error)

	// GenerateWeeklyOutlook generates a comprehensive weekly briefing.
	// Input: all available data aggregated.
	// Output: 500-800 word market outlook.
	GenerateWeeklyOutlook(ctx context.Context, data WeeklyData) (string, error)

	// AnalyzeCrossMarket generates a risk-on/risk-off regime narrative.
	// Input: COT data across Gold, USD, Bonds, Oil.
	// Output: cross-market correlation analysis.
	AnalyzeCrossMarket(ctx context.Context, cotData map[string]*domain.COTAnalysis) (string, error)

	// AnalyzeNewsOutlook generates a calendar-focused weekly intelligence report.
	// Input: array of the week's economic events.
	AnalyzeNewsOutlook(ctx context.Context, events []domain.NewsEvent, lang string) (string, error)

	// AnalyzeCombinedOutlook fuses COT macro positioning with upcoming calendar catalysts.
	// Input: WeeklyData containing both COT and News.
	AnalyzeCombinedOutlook(ctx context.Context, data WeeklyData) (string, error)

	// AnalyzeFREDOutlook generates a macro-economic AI narrative from FRED data.
	// Input: MacroData + regime classification + optional language preference.
	AnalyzeFREDOutlook(ctx context.Context, data *fred.MacroData, lang string) (string, error)

	// AnalyzeActualRelease evaluates a single economic release against its forecast.
	AnalyzeActualRelease(ctx context.Context, event domain.NewsEvent, lang string) (string, error)

	// IsAvailable returns true if the AI service is configured and reachable.
	IsAvailable() bool
}
