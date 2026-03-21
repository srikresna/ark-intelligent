package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("ai")

// allowedTags are the only HTML tags Telegram Bot API accepts in HTML parse mode.
var allowedTags = map[string]bool{
	"b": true, "/b": true,
	"i": true, "/i": true,
	"u": true, "/u": true,
	"s": true, "/s": true,
	"code": true, "/code": true,
	"pre": true, "/pre": true,
	"a": true, "/a": true,
	"tg-spoiler": true, "/tg-spoiler": true,
}

// reTags matches any <tag> or </tag> including those with attributes.
var reTags = regexp.MustCompile(`<(/?\w[\w-]*)(\s[^>]*)?>`)

// sanitizeTelegramHTML strips any HTML tags that Telegram does not support,
// replacing them with their inner text or escaping the angle brackets.
// Allowed tags (b, i, u, s, code, pre, a, tg-spoiler) are kept as-is.
func sanitizeTelegramHTML(s string) string {
	return reTags.ReplaceAllStringFunc(s, func(match string) string {
		sub := reTags.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		tagName := strings.ToLower(sub[1])
		if allowedTags[tagName] {
			return match // keep valid Telegram tags
		}
		// Unknown tag — escape the angle brackets so Telegram won't try to parse it
		match = strings.ReplaceAll(match, "<", "&lt;")
		match = strings.ReplaceAll(match, ">", "&gt;")
		return match
	})
}

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
	return ip.AnalyzeCOTWithPrice(ctx, analyses, nil)
}

// AnalyzeCOTWithPrice generates a price-aware COT interpretation.
func (ip *Interpreter) AnalyzeCOTWithPrice(ctx context.Context, analyses []domain.COTAnalysis, priceCtx map[string]*domain.PriceContext) (string, error) {
	if len(analyses) == 0 {
		return "No COT data available for analysis.", nil
	}

	var prompt string
	if len(priceCtx) > 0 {
		prompt = BuildCOTAnalysisPrompt(analyses, priceCtx)
	} else {
		prompt = BuildCOTAnalysisPrompt(analyses)
	}

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt(), prompt)
	if err != nil {
		log.Error().Err(err).Msg("COT analysis failed")
		return ip.fallbackCOTSummary(analyses), nil
	}

	return formatResponse("COT ANALYSIS", result), nil
}

// GenerateWeeklyOutlook creates a comprehensive weekly market outlook.
// Gap E: if MacroData is provided in WeeklyData, the FRED macro regime is injected
// into the COT outlook prompt so the COT outlook is always regime-aware.
func (ip *Interpreter) GenerateWeeklyOutlook(ctx context.Context, data ports.WeeklyData) (string, error) {
	outlookData := WeeklyOutlookData{
		COTAnalyses: data.COTAnalyses,
	}

	// Gap E: derive regime from MacroData when available
	var macroRegime *fred.MacroRegime
	if data.MacroData != nil {
		r := fred.ClassifyMacroRegime(data.MacroData)
		macroRegime = &r
	}

	prompt := BuildWeeklyOutlookPrompt(outlookData, data.Language, macroRegime)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt(), prompt)
	if err != nil {
		log.Error().Err(err).Msg("weekly outlook failed")
		return ip.fallbackWeeklyOutlook(outlookData), nil
	}

	return formatResponse("WEEKLY OUTLOOK", result), nil
}

// AnalyzeCrossMarket generates cross-market positioning interpretation.
func (ip *Interpreter) AnalyzeCrossMarket(ctx context.Context, cotData map[string]*domain.COTAnalysis) (string, error) {
	prompt := BuildCrossMarketPrompt(cotData)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt(), prompt)
	if err != nil {
		log.Error().Err(err).Msg("cross-market analysis failed")
		return "Cross-market analysis unavailable.", nil
	}

	return formatResponse("CROSS-MARKET ANALYSIS", result), nil
}

// AnalyzeNewsOutlook generates a calendar-focused weekly intelligence report.
// Gap E: macroData is fetched from cache (non-blocking) so news analysis
// gets FRED regime context injected into the prompt automatically.
func (ip *Interpreter) AnalyzeNewsOutlook(ctx context.Context, events []domain.NewsEvent, lang string) (string, error) {
	if len(events) == 0 {
		return "No upcoming economic events found for the week.", nil
	}

	// Gap E: try to get cached FRED regime — non-fatal if unavailable
	var macroRegime *fred.MacroRegime
	if macroData, err := fred.GetCachedOrFetch(ctx); err == nil && macroData != nil {
		r := fred.ClassifyMacroRegime(macroData)
		macroRegime = &r
	}

	prompt := BuildNewsOutlookPrompt(events, lang, macroRegime)
	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt(), prompt)
	if err != nil {
		log.Error().Err(err).Msg("news outlook failed")
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

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt(), prompt)
	if err != nil {
		log.Error().Err(err).Msg("combined outlook failed")
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

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt(), prompt)
	if err != nil {
		log.Error().Err(err).Msg("FRED outlook failed")
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

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt(), prompt)
	if err != nil {
		log.Error().Err(err).Msg("actual release flash failed")
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
			log.Error().Err(err).Msg("batch COT failed")
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
		log.Error().Err(err).Msg("batch weekly failed")
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
			log.Error().Err(err).Msg("batch cross-market failed")
		} else {
			results["cross_market"] = crossResult
		}
		throttle()
	}

	log.Info().Int("count", len(results)).Msg("generated insights")
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
	return fmt.Sprintf("=== %s ===\n\n%s", header, sanitizeTelegramHTML(content))
}

// throttle adds a small delay between API calls to avoid rate limiting.
func throttle() {
	time.Sleep(500 * time.Millisecond)
}
