package ai

import (
	"context"
	"fmt"

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
func (ca *ClaudeAnalyzer) GenerateUnifiedOutlook(ctx context.Context, data UnifiedOutlookData) (string, error) {
	prompt := BuildUnifiedOutlookPrompt(data)

	// Build request with web_search and web_fetch tools enabled.
	// These are server-managed tools — Claude's servers execute them automatically.
	req := ports.ChatRequest{
		SystemPrompt:  SystemPrompt(),
		OverrideModel: ca.overrideModel,
		Messages: []ports.ChatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 4096,
		Tools: []ports.ServerTool{
			{Type: "web_search_20250305", Name: "web_search", MaxUses: 5},
			{Type: "web_fetch_20260309", Name: "web_fetch", MaxUses: 3},
		},
	}

	resp, err := ca.claude.Chat(ctx, req)
	if err != nil {
		log.Error().Err(err).Msg("ClaudeAnalyzer: unified outlook failed")
		return "Unified outlook unavailable.", nil
	}

	toolInfo := ""
	if len(resp.ToolsUsed) > 0 {
		toolInfo = fmt.Sprintf(" [enriched via: %s]", joinUnique(resp.ToolsUsed))
	}

	return formatResponse("UNIFIED OUTLOOK"+toolInfo, resp.Content), nil
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
