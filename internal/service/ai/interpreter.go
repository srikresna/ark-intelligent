package ai

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
	"github.com/arkcode369/ff-calendar-bot/internal/service/fred"
)

// WeeklyOutlookData is the internal data structure used by the AI interpreter
// for generating weekly outlooks. It mirrors ports.WeeklyData but with
// types that are more convenient for prompt building.
type WeeklyOutlookData struct {
	COTAnalyses []domain.COTAnalysis
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

// GenerateWeeklyOutlook creates a comprehensive weekly market outlook.
func (ip *Interpreter) GenerateWeeklyOutlook(ctx context.Context, data ports.WeeklyData) (string, error) {
	outlookData := WeeklyOutlookData{
		COTAnalyses: data.COTAnalyses,
	}

	prompt := BuildWeeklyOutlookPrompt(outlookData, data.Language)

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

// AnalyzeNewsOutlook generates a calendar-focused weekly intelligence report.
func (ip *Interpreter) AnalyzeNewsOutlook(ctx context.Context, events []domain.NewsEvent, lang string) (string, error) {
	if len(events) == 0 {
		return "No upcoming economic events found for the week.", nil
	}

	prompt := BuildNewsOutlookPrompt(events, lang)
	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] news outlook failed: %v", err)
		return "News outlook unavailable.", nil
	}

	return formatResponse("NEWS OUTLOOK", result), nil
}

// AnalyzeCombinedOutlook fuses COT macro positioning with upcoming calendar catalysts.
// If data.MacroData is populated, it also includes FRED macro backdrop in the analysis.
func (ip *Interpreter) AnalyzeCombinedOutlook(ctx context.Context, data ports.WeeklyData) (string, error) {
	var prompt string
	var header string

	if data.MacroData != nil {
		regime := fred.ClassifyMacroRegime(data.MacroData)
		prompt = BuildCombinedWithFREDPrompt(data, regime)
		header = "FUSED OUTLOOK (COT + NEWS + FRED)"
	} else {
		prompt = BuildCombinedOutlookPrompt(data)
		header = "FUSED OUTLOOK (COT + NEWS)"
	}

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] combined outlook failed: %v", err)
		return "Combined outlook unavailable.", nil
	}

	return formatResponse(header, result), nil
}

// AnalyzeFREDOutlook generates a macro-economic AI narrative from FRED quantitative data.
func (ip *Interpreter) AnalyzeFREDOutlook(ctx context.Context, data *fred.MacroData, lang string) (string, error) {
	if data == nil {
		return "No FRED macro data available for analysis.", nil
	}

	regime := fred.ClassifyMacroRegime(data)
	prompt := BuildFREDOutlookPrompt(data, regime, lang)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] FRED outlook failed: %v", err)
		return ip.fallbackFREDSummary(data, regime), nil
	}

	return formatResponse("FRED MACRO OUTLOOK", result), nil
}

// fallbackFREDSummary generates a template-based FRED summary when Gemini is unavailable.
func (ip *Interpreter) fallbackFREDSummary(data *fred.MacroData, regime fred.MacroRegime) string {
	var b strings.Builder
	b.WriteString("=== FRED MACRO OUTLOOK (Auto-generated) ===\n\n")
	b.WriteString(fmt.Sprintf("Regime: %s | Risk Score: %d/100\n", regime.Name, regime.Score))
	b.WriteString(fmt.Sprintf("Bias: %s\n", regime.Bias))
	b.WriteString(fmt.Sprintf("Description: %s\n\n", regime.Description))
	b.WriteString(fmt.Sprintf("Yield Spread: %.2f%% | Core PCE: %.1f%%\n", data.YieldSpread, data.CorePCE))
	if data.FedFundsRate > 0 {
		b.WriteString(fmt.Sprintf("Fed Funds: %.2f%% | NFCI: %.3f\n", data.FedFundsRate, data.NFCI))
	}
	b.WriteString("\nAI detailed narrative unavailable. Use /macro for raw data.")
	return b.String()
}

// AnalyzeActualRelease evaluates a single economic release against its forecast.
func (ip *Interpreter) AnalyzeActualRelease(ctx context.Context, event domain.NewsEvent, lang string) (string, error) {
	prompt := BuildActualReleasePrompt(event, lang)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] actual release flash failed: %v", err)
		return "", err
	}

	return result, nil // no header needed for inline alert
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

	// 2. Weekly Outlook
	weeklyData := ports.WeeklyData{
		COTAnalyses: data.COTAnalyses,
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

func (ip *Interpreter) fallbackWeeklyOutlook(data WeeklyOutlookData) string {
	var b strings.Builder
	b.WriteString("=== WEEKLY OUTLOOK (Auto-generated) ===\n\n")

	if len(data.COTAnalyses) > 0 {
		b.WriteString(fmt.Sprintf("COT: %d contracts analyzed\n", len(data.COTAnalyses)))
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
