package ai

import (
	"context"
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
)

// ClaudeAnalyzer implements ports.AIAnalyzer using the ClaudeClient.
// It reuses the same prompt builders as the Gemini Interpreter so the
// AI output is consistent regardless of which model is used.
//
// Usage: when a user sets PreferredModel="claude" in /settings, use
// WithModel() to get a per-request scoped copy with the user's preferred
// Claude variant, then call the analysis methods. This is fully thread-safe
// since each request works with its own copy and no shared mutable state.
type ClaudeAnalyzer struct {
	claude        *ClaudeClient
	eventRepo     ports.EventRepository
	cotRepo       ports.COTRepository
	overrideModel string // immutable after construction (set via WithModel)
}

// NewClaudeAnalyzer creates a ClaudeAnalyzer wrapping the given client.
func NewClaudeAnalyzer(claude *ClaudeClient, eventRepo ports.EventRepository, cotRepo ports.COTRepository) *ClaudeAnalyzer {
	return &ClaudeAnalyzer{
		claude:    claude,
		eventRepo: eventRepo,
		cotRepo:   cotRepo,
	}
}

// WithModel returns a shallow copy of the ClaudeAnalyzer with the given model override.
// Use this to get a per-request scoped analyzer without mutating the shared instance.
// Pass "" to use the server default model from config.
func (ca *ClaudeAnalyzer) WithModel(model string) *ClaudeAnalyzer {
	copy := *ca
	copy.overrideModel = model
	return &copy
}

// SetOverrideModel is kept for compatibility — prefer WithModel() for new call sites.
// It mutates the receiver; only safe to call before any goroutine shares the analyzer.
func (ca *ClaudeAnalyzer) SetOverrideModel(model string) {
	ca.overrideModel = model
}

// Ensure ClaudeAnalyzer implements ports.AIAnalyzer at compile time.
var _ ports.AIAnalyzer = (*ClaudeAnalyzer)(nil)

// IsAvailable returns true if the Claude client is configured.
func (ca *ClaudeAnalyzer) IsAvailable() bool {
	return ca.claude != nil && ca.claude.endpoint != ""
}

// generate sends a single-turn prompt to Claude and returns the text response.
// It uses ca.overrideModel if set (per-request user model selection),
// otherwise falls back to the client's server-configured default model.
func (ca *ClaudeAnalyzer) generate(ctx context.Context, prompt string) (string, error) {
	req := ports.ChatRequest{
		SystemPrompt:  SystemPrompt(),
		OverrideModel: ca.overrideModel, // "" = use client default
		Messages: []ports.ChatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 2048,
	}

	resp, err := ca.claude.Chat(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// AnalyzeCOT generates a natural language interpretation of COT positioning data.
func (ca *ClaudeAnalyzer) AnalyzeCOT(ctx context.Context, analyses []domain.COTAnalysis) (string, error) {
	return ca.AnalyzeCOTWithPrice(ctx, analyses, nil)
}

// AnalyzeCOTWithPrice generates a price-aware COT interpretation.
func (ca *ClaudeAnalyzer) AnalyzeCOTWithPrice(ctx context.Context, analyses []domain.COTAnalysis, priceCtx map[string]*domain.PriceContext) (string, error) {
	if len(analyses) == 0 {
		return "No COT data available for analysis.", nil
	}

	var prompt string
	if len(priceCtx) > 0 {
		prompt = BuildCOTAnalysisPrompt(analyses, priceCtx)
	} else {
		prompt = BuildCOTAnalysisPrompt(analyses)
	}

	result, err := ca.generate(ctx, prompt)
	if err != nil {
		log.Error().Err(err).Msg("ClaudeAnalyzer: COT analysis failed")
		return fmt.Sprintf("COT analysis unavailable: %s", err.Error()), nil
	}

	return formatResponse("COT ANALYSIS", result), nil
}

// GenerateWeeklyOutlook creates a comprehensive weekly market outlook.
func (ca *ClaudeAnalyzer) GenerateWeeklyOutlook(ctx context.Context, data ports.WeeklyData) (string, error) {
	outlookData := WeeklyOutlookData{
		COTAnalyses:   data.COTAnalyses,
		PriceContexts: data.PriceContexts,
	}

	var macroRegime *fred.MacroRegime
	if data.MacroData != nil {
		r := fred.ClassifyMacroRegime(data.MacroData)
		macroRegime = &r
	}

	prompt := BuildWeeklyOutlookPrompt(outlookData, data.Language, macroRegime, data.BacktestStats)

	result, err := ca.generate(ctx, prompt)
	if err != nil {
		log.Error().Err(err).Msg("ClaudeAnalyzer: weekly outlook failed")
		return "Weekly outlook unavailable.", nil
	}

	return formatResponse("WEEKLY OUTLOOK", result), nil
}

// AnalyzeCrossMarket generates cross-market positioning interpretation.
func (ca *ClaudeAnalyzer) AnalyzeCrossMarket(ctx context.Context, cotData map[string]*domain.COTAnalysis) (string, error) {
	prompt := BuildCrossMarketPrompt(cotData)

	result, err := ca.generate(ctx, prompt)
	if err != nil {
		log.Error().Err(err).Msg("ClaudeAnalyzer: cross-market analysis failed")
		return "Cross-market analysis unavailable.", nil
	}

	return formatResponse("CROSS-MARKET ANALYSIS", result), nil
}

// AnalyzeNewsOutlook generates a calendar-focused weekly intelligence report.
func (ca *ClaudeAnalyzer) AnalyzeNewsOutlook(ctx context.Context, events []domain.NewsEvent, lang string) (string, error) {
	if len(events) == 0 {
		return "No upcoming economic events found for the week.", nil
	}

	var macroRegime *fred.MacroRegime
	if macroData, err := fred.GetCachedOrFetch(ctx); err == nil && macroData != nil {
		r := fred.ClassifyMacroRegime(macroData)
		macroRegime = &r
	}

	prompt := BuildNewsOutlookPrompt(events, lang, macroRegime)

	result, err := ca.generate(ctx, prompt)
	if err != nil {
		log.Error().Err(err).Msg("ClaudeAnalyzer: news outlook failed")
		return "News outlook unavailable.", nil
	}

	return formatResponse("NEWS OUTLOOK", result), nil
}

// AnalyzeCombinedOutlook fuses COT + News + FRED into one outlook.
func (ca *ClaudeAnalyzer) AnalyzeCombinedOutlook(ctx context.Context, data ports.WeeklyData) (string, error) {
	var prompt, header string

	if data.MacroData != nil {
		regime := fred.ClassifyMacroRegime(data.MacroData)
		prompt = BuildCombinedWithFREDPrompt(data, regime)
		header = "FUSED OUTLOOK (COT + NEWS + FRED)"
	} else {
		prompt = BuildCombinedOutlookPrompt(data)
		header = "FUSED OUTLOOK (COT + NEWS)"
	}

	result, err := ca.generate(ctx, prompt)
	if err != nil {
		log.Error().Err(err).Msg("ClaudeAnalyzer: combined outlook failed")
		return "Combined outlook unavailable.", nil
	}

	return formatResponse(header, result), nil
}

// AnalyzeFREDOutlook generates a macro-economic AI narrative from FRED data.
func (ca *ClaudeAnalyzer) AnalyzeFREDOutlook(ctx context.Context, data *fred.MacroData, lang string) (string, error) {
	if data == nil {
		return "No FRED macro data available for analysis.", nil
	}

	regime := fred.ClassifyMacroRegime(data)
	prompt := BuildFREDOutlookPrompt(data, regime, lang)

	result, err := ca.generate(ctx, prompt)
	if err != nil {
		log.Error().Err(err).Msg("ClaudeAnalyzer: FRED outlook failed")
		return "FRED outlook unavailable.", nil
	}

	return formatResponse("FRED MACRO OUTLOOK", result), nil
}

// AnalyzeActualRelease evaluates a single economic release against its forecast.
func (ca *ClaudeAnalyzer) AnalyzeActualRelease(ctx context.Context, event domain.NewsEvent, lang string) (string, error) {
	prompt := BuildActualReleasePrompt(event, lang)

	result, err := ca.generate(ctx, prompt)
	if err != nil {
		log.Error().Err(err).Msg("ClaudeAnalyzer: actual release analysis failed")
		return "", err
	}

	return result, nil
}

// GenerateUnifiedOutlook creates a comprehensive unified outlook fusing ALL
// data sources and enabling Claude's web_search + web_fetch tools for
// real-time data enrichment. This is the primary /outlook path.
//
// Multi-phase design to stay within Vercel's 60s function timeout:
//
//	Phase 1: Full analysis with extended thinking, NO web tools
//	Phase 2a/2b/2c: Individual web searches (1 search per request, lightweight context)
//	Phase 3: Final synthesis combining analysis + web findings
//
// If any phase fails, the best available result is returned (graceful degradation).
func (ca *ClaudeAnalyzer) GenerateUnifiedOutlook(ctx context.Context, data UnifiedOutlookData) (string, error) {
	prompt := BuildUnifiedOutlookPrompt(data)

	// --- Phase 1: Deep analysis with thinking, no web tools ---
	log.Info().Msg("unified outlook: phase 1 — deep analysis with thinking")
	phase1Req := ports.ChatRequest{
		SystemPrompt:  SystemPrompt(),
		OverrideModel: ca.overrideModel,
		Messages: []ports.ChatMessage{
			{Role: "user", Content: prompt + "\n\nIMPORTANT: Analyze ALL the data above thoroughly. Do NOT use web tools yet — focus on the data provided. Provide your full analysis including all 6 sections requested."},
		},
		MaxTokens: config.AIDefaultMaxTokens,
	}

	phase1Resp, err := ca.claude.Chat(ctx, phase1Req)
	if err != nil {
		log.Error().Err(err).Msg("ClaudeAnalyzer: unified outlook phase 1 failed")
		return "", fmt.Errorf("unified outlook phase 1: %w", err)
	}
	log.Info().
		Int("input_tokens", phase1Resp.InputTokens).
		Int("output_tokens", phase1Resp.OutputTokens).
		Msg("unified outlook: phase 1 complete")

	// --- Phase 2: Sequential web searches (1 per request to fit in 60s) ---
	// Each sub-request has minimal context: just a short summary + 1 search query.
	// This avoids sending the full prompt + phase1 response which would be too large.
	webSearchQueries := []string{
		"Search for the latest forex market prices and major moves today for EUR/USD, GBP/USD, USD/JPY, AUD/USD, USD/CAD, NZD/USD, USD/CHF, DXY, Gold, Silver, Copper, Crude Oil, RBOB Gasoline, Heating Oil, Bitcoin, and Ethereum. Report current prices and percentage changes.",
		"Search for the latest central bank news, Fed statements, ECB/BOJ/BOE decisions, and any interest rate expectations changes this week. Also check for bond market moves in US Treasuries (2Y, 5Y, 10Y, 30Y).",
		"Search for breaking geopolitical news, trade policy developments, equity index moves (S&P 500, Nasdaq, Dow, Russell 2000), crypto market cap trends, and risk events that could impact forex, commodity, and equity futures markets this week.",
	}

	var webFindings []string
	var allToolsUsed []string

	for i, query := range webSearchQueries {
		log.Info().Int("round", i+1).Msg("unified outlook: phase 2 — web search round")

		searchReq := ports.ChatRequest{
			OverrideModel:   ca.overrideModel,
			DisableThinking: true,
			Messages: []ports.ChatMessage{
				{Role: "user", Content: query + "\n\nProvide a concise factual summary of what you find. Be brief — just the key data points and facts."},
			},
			MaxTokens: 1024,
			Tools: []ports.ServerTool{
				{Type: "web_search_20250305", Name: "web_search", MaxUses: 1},
			},
		}

		searchResp, err := ca.claude.Chat(ctx, searchReq)
		if err != nil {
			log.Warn().Err(err).Int("round", i+1).Msg("unified outlook: web search round failed, skipping")
			continue
		}

		log.Info().
			Int("round", i+1).
			Int("input_tokens", searchResp.InputTokens).
			Int("output_tokens", searchResp.OutputTokens).
			Strs("tools_used", searchResp.ToolsUsed).
			Msg("unified outlook: web search round complete")

		if searchResp.Content != "" {
			webFindings = append(webFindings, searchResp.Content)
			allToolsUsed = append(allToolsUsed, searchResp.ToolsUsed...)
		}
	}

	// If no web data was gathered, return Phase 1 result
	if len(webFindings) == 0 {
		log.Warn().Msg("unified outlook: all web searches failed, returning phase 1 result")
		return formatResponse("UNIFIED OUTLOOK", phase1Resp.Content), nil
	}

	// --- Phase 3: Final synthesis with reduced context ---
	// Only send a compact version to stay within Vercel 60s timeout.
	// Instead of resending the full original prompt, we provide the phase 1
	// analysis + web findings and ask Claude to merge them.
	log.Info().Int("web_findings", len(webFindings)).Msg("unified outlook: phase 3 — final synthesis")

	// Build web findings summary
	var webSummary string
	for i, finding := range webFindings {
		webSummary += fmt.Sprintf("\n--- Web Research %d ---\n%s\n", i+1, finding)
	}

	lang := "Indonesian (Bahasa Indonesia)"
	if data.Language == "en" {
		lang = "English"
	}

	toolInfo := ""
	if len(allToolsUsed) > 0 {
		toolInfo = fmt.Sprintf(" [enriched via: %s]", joinUnique(allToolsUsed))
	}

	phase3Req := ports.ChatRequest{
		OverrideModel:   ca.overrideModel,
		DisableThinking: true,
		Messages: []ports.ChatMessage{
			{Role: "user", Content: fmt.Sprintf("Below is a market analysis and fresh web research data. Produce the FINAL unified outlook by merging both. Where the web data contradicts or updates the analysis, highlight the discrepancy. Respond in %s. Be concise but comprehensive.\n\n=== ANALYSIS ===\n%s\n\n=== WEB RESEARCH ===\n%s\n\nProduce the definitive UNIFIED OUTLOOK covering: (1) Macro regime & context, (2) Currency-by-currency analysis with bias and conviction, (3) Top 3 trade setups, (4) Cross-market signals, (5) Key risks & catalysts.", lang, phase1Resp.Content, webSummary)},
		},
		MaxTokens: 2048,
	}

	phase3Resp, err := ca.claude.Chat(ctx, phase3Req)
	if err != nil {
		// Phase 3 failed — return Phase 1 analysis + raw web findings appended
		log.Warn().Err(err).Msg("unified outlook: phase 3 synthesis failed, returning phase 1 + web findings")
		combined := phase1Resp.Content + "\n\n---\n📡 WEB RESEARCH UPDATE:\n" + webSummary
		return formatResponse("UNIFIED OUTLOOK"+toolInfo, combined), nil
	}
	log.Info().
		Int("input_tokens", phase3Resp.InputTokens).
		Int("output_tokens", phase3Resp.OutputTokens).
		Msg("unified outlook: phase 3 complete")

	return formatResponse("UNIFIED OUTLOOK"+toolInfo, phase3Resp.Content), nil
}

// joinUnique deduplicates and joins string slice.
func joinUnique(items []string) string {
	seen := make(map[string]bool, len(items))
	var unique []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			unique = append(unique, item)
		}
	}
	result := ""
	for i, u := range unique {
		if i > 0 {
			result += ", "
		}
		result += u
	}
	return result
}
