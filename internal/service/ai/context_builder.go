package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

// ContextBuilder constructs system prompts for chatbot conversations
// by injecting live market data when the query is forex-related.
// FedSpeechSummary is a lightweight summary for AI context injection.
type FedSpeechSummary struct {
	Speaker     string
	Title       string
	PublishedAt string // formatted date
}

// FedSpeechProvider returns recent Fed speeches for AI context.
type FedSpeechProvider func(n int) []FedSpeechSummary

type ContextBuilder struct {
	cotRepo           ports.COTRepository
	newsRepo          ports.NewsRepository
	priceRepo         ports.PriceRepository
	fedSpeechProvider FedSpeechProvider
}

// NewContextBuilder creates a new ContextBuilder.
// All repos are nil-safe — missing data is simply omitted from the prompt.
func NewContextBuilder(
	cotRepo ports.COTRepository,
	newsRepo ports.NewsRepository,
	priceRepo ports.PriceRepository,
) *ContextBuilder {
	return &ContextBuilder{
		cotRepo:   cotRepo,
		newsRepo:  newsRepo,
		priceRepo: priceRepo,
	}
}

// SetFedSpeechProvider registers a callback that supplies recent Fed speeches.
func (cb *ContextBuilder) SetFedSpeechProvider(fn FedSpeechProvider) {
	cb.fedSpeechProvider = fn
}

// forexKeywords are terms that trigger market data injection into the system prompt.
var forexKeywords = []string{
	"forex", "fx", "usd", "eur", "gbp", "jpy", "aud", "nzd", "cad", "chf",
	"gold", "xau", "oil", "wti", "brent",
	"cot", "positioning", "commitment", "traders",
	"macro", "fed", "ecb", "boj", "boe", "rba", "rbnz", "boc", "snb",
	"nfp", "cpi", "gdp", "pmi", "fomc", "rate", "yield", "treasury",
	"currency", "pair", "bullish", "bearish", "dovish", "hawkish",
	"inflation", "employment", "payroll", "interest",
	"dxy", "dollar", "euro", "pound", "yen",
	"signal", "outlook", "analysis",
}

// isForexRelated checks if a user message contains forex-related terms.
func isForexRelated(text string) bool {
	lower := strings.ToLower(text)
	for _, kw := range forexKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// basePersona is the system prompt for all conversations.
const basePersona = `You are ARK Intelligence, an institutional-grade macro analyst AI assistant integrated into a Telegram bot.

You specialize in forex markets, COT (Commitments of Traders) data analysis, and macroeconomic intelligence. You can also help with general financial topics.

You have access to powerful tools:
- Web search: search the internet for real-time data and news.
- Web fetch: fetch and read specific URLs for detailed content.
- Code execution: run Python code for calculations, data analysis, and visualizations.

Rules:
- Respond in the same language as the user (Indonesian/English).
- Be concise and actionable. Prioritize data-driven insights.
- Format for Telegram HTML: use ONLY <b>, <i>, <code> tags. NO other HTML tags.
- NEVER use angle brackets for non-HTML purposes. Use parentheses or square brackets instead.
- NEVER use markdown formatting (no **, ##, -, etc). Use ONLY Telegram HTML tags.
- Keep responses under 1000 words.
- Use WIB (UTC+7) for all times.
- When asked about current market data, use web search to get real-time information.
- When calculations are needed, use code execution for accuracy.
- If asked about data you don't have, suggest the appropriate /command.

<reasoning>
Always show your reasoning process. Do not just state conclusions — explain WHY:
- What data points support your view?
- What is the opposing argument or risk?
- What could invalidate your analysis?
Structure analysis as: Observation → Evidence → Conclusion → Risk.
</reasoning>

<confidence>
Express your confidence level honestly:
- HIGH: multiple data points align, clear trend, strong confluence
- MEDIUM: mixed signals, some data supports, some contradicts
- LOW: insufficient data, conflicting indicators, unclear picture
Never present uncertain analysis as certain. If data is stale or incomplete, say so explicitly. If you do not know something, say "I don't have that data" rather than guessing.
</confidence>

<external_validation>
When providing market analysis, PROACTIVELY use web search and web fetch to validate and enrich your response — even if the user did not ask for it:
- Cross-check injected COT/macro data with current live sources for freshness.
- Search for latest central bank statements, breaking news, or sentiment shifts that may affect the analysis.
- If the user asks about current price, conditions, or "what's happening now", always search first before answering.
Do not rely solely on injected context data. Treat it as a starting point, then verify and supplement with external sources.
</external_validation>

<memory_usage>
You have a memory tool to store and recall user-specific information across sessions. Use it to:
- Save the user's trading preferences (pairs, timeframe, risk tolerance, style) when they mention them.
- Save important context (account size, broker, active positions) when shared.
- Read memory at the start of analytical requests to personalize your response.
- Update memory when preferences change.
Do NOT save trivial conversation or greetings. Only persist information that improves future analysis quality.
</memory_usage>

<available_commands>
The bot has these slash commands users can run directly. When relevant, suggest them:
- /cot — Detailed COT positioning analysis per currency
- /outlook — Weekly macro + COT market outlook (AI-generated)
- /calendar — Economic calendar for the current week
- /macro — FRED macro regime dashboard (yield curve, PCE, unemployment, DXY)
- /bias — Active COT-based directional biases (positioning analysis)
- /rank — Currency strength ranking based on COT + macro data
- /backtest — Historical backtest performance of COT signals
- /accuracy — Quick signal accuracy summary
- /settings — User notification preferences
- /clear — Clear conversation history
When the user asks for something a command handles better, suggest the command instead of repeating raw data.
</available_commands>`

// BuildSystemPrompt constructs the system prompt for a chat message.
// If the message is forex-related, live market data is injected.
func (cb *ContextBuilder) BuildSystemPrompt(ctx context.Context, userMessage string) string {
	// BUG #9 FIX: Use timeutil.NowWIB() instead of time.Now().UTC().Add(7h).
	// The manual UTC+7 offset is functionally equivalent most of the time but is
	// inconsistent with the rest of the codebase and does not use the proper
	// Asia/Jakarta timezone location, which can diverge if the system clock or
	// timezone DB changes. Using timeutil.NowWIB() ensures all date/time logic
	// in the system uses the same canonical WIB source.
	nowWIB := timeutil.NowWIB()

	if !isForexRelated(userMessage) {
		return fmt.Sprintf("%s\n\nToday's date: %s (UTC+7 WIB).",
			basePersona,
			nowWIB.Format("Monday, 02 January 2006"),
		)
	}

	var b strings.Builder
	b.WriteString(basePersona)
	b.WriteString(fmt.Sprintf("\n\nToday's date: %s (UTC+7 WIB).\n",
		nowWIB.Format("Monday, 02 January 2006")))

	// Inject COT data
	cb.injectCOTContext(ctx, &b)

	// Inject FRED macro regime
	cb.injectFREDContext(ctx, &b)

	// Inject upcoming calendar events
	cb.injectCalendarContext(ctx, &b)

	// Inject recent Fed speeches / FOMC releases
	cb.injectFedSpeechContext(&b)

	return b.String()
}

// injectCOTContext adds latest COT positioning data to the prompt.
func (cb *ContextBuilder) injectCOTContext(ctx context.Context, b *strings.Builder) {
	if cb.cotRepo == nil {
		return
	}

	analyses, err := cb.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil || len(analyses) == 0 {
		return
	}

	b.WriteString("\n--- LIVE COT POSITIONING DATA ---\n")
	for _, a := range analyses {
		b.WriteString(fmt.Sprintf("%s (%s) | Report: %s | Spec Net: %s (chg: %s) | Comm Net: %s | Sentiment: %.1f\n",
			a.Contract.Code, a.Contract.Currency,
			a.ReportDate.Format("2006-01-02"),
			fmtutil.FmtNumSigned(a.NetPosition, 0),
			fmtutil.FmtNumSigned(a.NetChange, 0),
			fmtutil.FmtNumSigned(a.CommercialNet, 0),
			a.SentimentScore,
		))
	}
}

// injectFREDContext adds FRED macro regime data to the prompt.
func (cb *ContextBuilder) injectFREDContext(ctx context.Context, b *strings.Builder) {
	macroData, err := fred.GetCachedOrFetch(ctx)
	if err != nil || macroData == nil {
		return
	}

	regime := fred.ClassifyMacroRegime(macroData)
	b.WriteString(fmt.Sprintf("\n--- FRED MACRO REGIME ---\nRegime: %s — %s\n", regime.Name, regime.Description))
	b.WriteString(fmt.Sprintf("Yield Curve (10Y-2Y): %.2f%%\n", macroData.YieldSpread))
	if macroData.UnemployRate > 0 {
		b.WriteString(fmt.Sprintf("Unemployment Rate: %.1f%%\n", macroData.UnemployRate))
	}
	if macroData.CorePCE > 0 {
		b.WriteString(fmt.Sprintf("Core PCE: %.1f%%\n", macroData.CorePCE))
	}
	if macroData.DXY > 0 {
		b.WriteString(fmt.Sprintf("DXY: %.1f\n", macroData.DXY))
	}
}

// injectCalendarContext adds upcoming economic events to the prompt.
func (cb *ContextBuilder) injectCalendarContext(ctx context.Context, b *strings.Builder) {
	if cb.newsRepo == nil {
		return
	}

	// BUG #9 FIX: Use timeutil.StartOfWeekWIB() instead of manual UTC+7 arithmetic.
	// The old code used time.Now().UTC().Add(7h) which is not a proper timezone location
	// and can produce wrong week boundaries near midnight WIB. StartOfWeekWIB() correctly
	// computes Monday 00:00:00 WIB using the Asia/Jakarta location.
	monday := timeutil.StartOfWeekWIB(timeutil.NowWIB())
	weekStart := monday.Format("20060102")

	events, err := cb.newsRepo.GetByWeek(ctx, weekStart)
	if err != nil || len(events) == 0 {
		return
	}

	// Filter to high-impact events only (to keep prompt size manageable)
	b.WriteString("\n--- UPCOMING HIGH-IMPACT EVENTS ---\n")
	shown := 0
	for _, e := range events {
		if e.Impact != "high" {
			continue
		}
		if shown >= 15 { // cap at 15 events
			break
		}
		timeStr := e.Date
		if !e.TimeWIB.IsZero() {
			timeStr = e.TimeWIB.Format("Mon 02 Jan 15:04 WIB")
		}
		b.WriteString(fmt.Sprintf("%s | %s | %s | Forecast: %s | Previous: %s\n",
			timeStr, e.Currency, e.Event,
			e.Forecast, e.Previous,
		))
		shown++
	}
}

// injectFedSpeechContext adds recent Fed speeches/FOMC releases to the prompt.
func (cb *ContextBuilder) injectFedSpeechContext(b *strings.Builder) {
	if cb.fedSpeechProvider == nil {
		return
	}
	speeches := cb.fedSpeechProvider(3)
	if len(speeches) == 0 {
		return
	}
	b.WriteString("\n--- RECENT FED COMMUNICATIONS ---\n")
	for _, s := range speeches {
		if s.Speaker != "" {
			fmt.Fprintf(b, "- %s (%s): %s\n", s.Speaker, s.PublishedAt, s.Title)
		} else {
			fmt.Fprintf(b, "- %s: %s\n", s.PublishedAt, s.Title)
		}
	}
}
