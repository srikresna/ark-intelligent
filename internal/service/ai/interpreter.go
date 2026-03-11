package ai

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
)

// WeeklyOutlookData is the internal data structure used by the AI interpreter
// for generating weekly outlooks. It mirrors ports.WeeklyData but with
// types that are more convenient for prompt building.
type WeeklyOutlookData struct {
	COTAnalyses      []domain.COTAnalysis
	HighImpactEvents []domain.FFEvent
	SurpriseIndices  map[string]*domain.SurpriseIndex
	Rankings         *domain.CurrencyRanking
	Confluences      []domain.ConfluenceScore
}

// Interpreter orchestrates AI-powered narrative generation for all analysis types.
// It implements the ports.AIAnalyzer interface, bridging the quantitative
// engines with natural language interpretation via Gemini.
type Interpreter struct {
	gemini    *GeminiClient
	eventRepo ports.EventRepository
	cotRepo   ports.COTRepository
}

// NewInterpreter creates an AI interpreter.
func NewInterpreter(gemini *GeminiClient, eventRepo ports.EventRepository, cotRepo ports.COTRepository) *Interpreter {
	return &Interpreter{
		gemini:    gemini,
		eventRepo: eventRepo,
		cotRepo:   cotRepo,
	}
}

// Ensure Interpreter implements ports.AIAnalyzer at compile time.
var _ ports.AIAnalyzer = (*Interpreter)(nil)

// IsAvailable returns true if the Gemini client is configured and ready.
// FIX: This method was missing, required by ports.AIAnalyzer interface.
func (ip *Interpreter) IsAvailable() bool {
	return ip.gemini != nil
}

// AnalyzeCOT generates a natural language interpretation of COT positioning data.
func (ip *Interpreter) AnalyzeCOT(ctx context.Context, analyses []domain.COTAnalysis) (string, error) {
	if len(analyses) == 0 {
		return "No COT data available for analysis.", nil
	}

	prompt := BuildCOTAnalysisPrompt(analyses)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] COT analysis failed: %v", err)
		return ip.fallbackCOTSummary(analyses), nil
	}

	return formatResponse("COT ANALYSIS", result), nil
}

// PredictEventImpact generates an AI interpretation of an upcoming event.
func (ip *Interpreter) PredictEventImpact(ctx context.Context, event domain.FFEvent, history []domain.FFEventDetail) (string, error) {
	prompt := BuildEventImpactPrompt(event, history)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] event impact failed: %v", err)
		return ip.fallbackEventSummary(event, history), nil
	}

	return formatResponse(fmt.Sprintf("%s %s IMPACT", event.Currency, event.Title), result), nil
}

// SynthesizeConfluence generates an AI interpretation of confluence scoring.
func (ip *Interpreter) SynthesizeConfluence(ctx context.Context, score domain.ConfluenceScore) (string, error) {
	prompt := BuildConfluencePrompt(score)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] confluence failed: %v", err)
		// FIX: Use Bias instead of Direction
		return fmt.Sprintf("%s Confluence: %.1f/100 (%s) - AI interpretation unavailable",
			score.CurrencyPair, score.TotalScore, string(score.Bias)), nil
	}

	return formatResponse(fmt.Sprintf("%s CONFLUENCE", score.CurrencyPair), result), nil
}

// GenerateWeeklyOutlook creates a comprehensive weekly market outlook.
func (ip *Interpreter) GenerateWeeklyOutlook(ctx context.Context, data ports.WeeklyData) (string, error) {
	// FIX: Convert ports.WeeklyData to internal WeeklyOutlookData
	// ports.WeeklyData uses: UpcomingEvents, SurpriseIndices ([]domain.SurpriseIndex),
	// CurrencyRanking, ConfluenceScores
	surpriseMap := make(map[string]*domain.SurpriseIndex)
	for i := range data.SurpriseIndices {
		idx := data.SurpriseIndices[i]
		surpriseMap[idx.Currency] = &idx
	}

	outlookData := WeeklyOutlookData{
		COTAnalyses:      data.COTAnalyses,
		HighImpactEvents: data.UpcomingEvents,     // FIX: was data.HighImpactEvents
		SurpriseIndices:  surpriseMap,              // FIX: convert slice to map
		Rankings:         data.CurrencyRanking,     // FIX: was data.Rankings
		Confluences:      data.ConfluenceScores,    // FIX: was data.Confluences
	}

	prompt := BuildWeeklyOutlookPrompt(outlookData)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] weekly outlook failed: %v", err)
		return ip.fallbackWeeklyOutlook(outlookData), nil
	}

	return formatResponse("WEEKLY OUTLOOK", result), nil
}

// AnalyzeCrossMarket generates cross-market positioning interpretation.
func (ip *Interpreter) AnalyzeCrossMarket(ctx context.Context, cotData map[string]*domain.COTAnalysis) (string, error) {
	prompt := BuildCrossMarketPrompt(cotData)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] cross-market failed: %v", err)
		return "Cross-market analysis unavailable.", nil
	}

	return formatResponse("CROSS-MARKET ANALYSIS", result), nil
}

// --- Batch Operations ---

// GenerateAllInsights runs all AI analyses and returns combined output.
func (ip *Interpreter) GenerateAllInsights(ctx context.Context, data WeeklyOutlookData) (map[string]string, error) {
	results := make(map[string]string)

	// 1. COT Analysis
	if len(data.COTAnalyses) > 0 {
		cotResult, err := ip.AnalyzeCOT(ctx, data.COTAnalyses)
		if err != nil {
			log.Printf("[ai] batch COT: %v", err)
		} else {
			results["cot"] = cotResult
		}
		throttle()
	}

	// 2. Weekly Outlook - FIX: Convert WeeklyOutlookData back to ports.WeeklyData
	// Convert surprise map back to slice
	var surpriseSlice []domain.SurpriseIndex
	for _, idx := range data.SurpriseIndices {
		if idx != nil {
			surpriseSlice = append(surpriseSlice, *idx)
		}
	}

	weeklyData := ports.WeeklyData{
		COTAnalyses:      data.COTAnalyses,
		UpcomingEvents:   data.HighImpactEvents,   // FIX: was HighImpactEvents
		SurpriseIndices:  surpriseSlice,            // FIX: convert map to slice
		CurrencyRanking:  data.Rankings,            // FIX: was Rankings
		ConfluenceScores: data.Confluences,         // FIX: was Confluences
	}
	weeklyResult, err := ip.GenerateWeeklyOutlook(ctx, weeklyData)
	if err != nil {
		log.Printf("[ai] batch weekly: %v", err)
	} else {
		results["weekly"] = weeklyResult
	}
	throttle()

	// 3. Cross-Market
	if len(data.COTAnalyses) > 1 {
		cotMap := make(map[string]*domain.COTAnalysis)
		for i := range data.COTAnalyses {
			a := data.COTAnalyses[i]
			// FIX: Use Contract.Code instead of ContractCode
			cotMap[a.Contract.Code] = &a
		}
		crossResult, err := ip.AnalyzeCrossMarket(ctx, cotMap)
		if err != nil {
			log.Printf("[ai] batch cross-market: %v", err)
		} else {
			results["cross_market"] = crossResult
		}
		throttle()
	}

	// 4. Top 3 event impacts
	eventCount := 0
	for _, ev := range data.HighImpactEvents {
		if eventCount >= 3 {
			break
		}
		if ev.Impact != domain.ImpactHigh {
			continue
		}

		history, _ := ip.eventRepo.GetEventHistory(ctx, ev.Title, ev.Currency, 12)
		eventResult, err := ip.PredictEventImpact(ctx, ev, history)
		if err != nil {
			log.Printf("[ai] batch event %s: %v", ev.Title, err)
			continue
		}
		results[fmt.Sprintf("event_%s_%s", ev.Currency, ev.Title)] = eventResult
		eventCount++
		throttle()
	}

	log.Printf("[ai] generated %d insights", len(results))
	return results, nil
}

// --- Fallback summaries (when Gemini is unavailable) ---

func (ip *Interpreter) fallbackCOTSummary(analyses []domain.COTAnalysis) string {
	var b strings.Builder
	b.WriteString("=== COT ANALYSIS (Auto-generated) ===\n\n")

	for _, a := range analyses {
		bias := "NEUTRAL"
		if a.SentimentScore > 20 {
			bias = "BULLISH"
		} else if a.SentimentScore < -20 {
			bias = "BEARISH"
		}

		// FIX: Use Contract.Currency and Contract.Name instead of Currency/ContractName
		b.WriteString(fmt.Sprintf("%s: %s\n", a.Contract.Currency, bias))
		b.WriteString(fmt.Sprintf("  Spec COT Index: %.0f | Comm: %s\n",
			a.COTIndex, a.CommercialSignal))

		if a.DivergenceFlag {
			b.WriteString("  [!] DIVERGENCE detected\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (ip *Interpreter) fallbackEventSummary(event domain.FFEvent, history []domain.FFEventDetail) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== %s %s ===\n", event.Currency, event.Title))

	if event.Forecast != "" {
		b.WriteString(fmt.Sprintf("Forecast: %s", event.Forecast))
		if event.Previous != "" {
			b.WriteString(fmt.Sprintf(" | Previous: %s", event.Previous))
		}
		b.WriteString("\n")
	}

	if len(history) > 0 {
		beats := 0
		for _, h := range history {
			if h.Surprise > 0 {
				beats++
			}
		}
		b.WriteString(fmt.Sprintf("History: Beat %d/%d releases\n", beats, len(history)))
	}

	return b.String()
}

func (ip *Interpreter) fallbackWeeklyOutlook(data WeeklyOutlookData) string {
	var b strings.Builder
	b.WriteString("=== WEEKLY OUTLOOK (Auto-generated) ===\n\n")

	if len(data.COTAnalyses) > 0 {
		b.WriteString(fmt.Sprintf("COT: %d contracts analyzed\n", len(data.COTAnalyses)))
	}
	if len(data.HighImpactEvents) > 0 {
		b.WriteString(fmt.Sprintf("Events: %d high-impact events upcoming\n", len(data.HighImpactEvents)))
	}
	if len(data.Confluences) > 0 {
		b.WriteString(fmt.Sprintf("Confluence: %d pairs scored\n", len(data.Confluences)))
	}

	b.WriteString("\nAI detailed analysis unavailable.\n")
	return b.String()
}

// --- Utility helpers ---

// formatResponse wraps AI output with a header.
func formatResponse(header, content string) string {
	return fmt.Sprintf("=== %s ===\n\n%s", header, content)
}

// throttle adds a small delay between API calls to avoid rate limiting.
func throttle() {
	time.Sleep(500 * time.Millisecond)
}
