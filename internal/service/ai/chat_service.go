package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var chatLog = logger.Component("chat-service")

// ErrAIFallback is returned when all AI services fail and a template fallback
// is used. The caller should still display the returned content, but can use
// this error to refund consumed AI quota since no real AI call succeeded.
var ErrAIFallback = errors.New("all AI services unavailable, using template fallback")

// OwnerNotifyFunc is a callback to notify the bot owner of AI service issues.
// html is the notification message in Telegram HTML format.
type OwnerNotifyFunc func(ctx context.Context, html string)

// ChatService orchestrates the chatbot pipeline:
// 1. Load conversation history
// 2. Build context-aware system prompt
// 3. Call Claude (primary) → Gemini (fallback) → template (last resort)
// 4. Persist conversation
type ChatService struct {
	claude         ports.ChatEngine
	gemini         *GeminiClient // may be nil
	convRepo       ports.ConversationRepository
	contextBuilder *ContextBuilder
	toolConfig     *ToolConfig
	ownerNotify    OwnerNotifyFunc // may be nil
}

// NewChatService creates a ChatService with the given dependencies.
// gemini may be nil (no Gemini fallback available).
func NewChatService(
	claude ports.ChatEngine,
	gemini *GeminiClient,
	convRepo ports.ConversationRepository,
	contextBuilder *ContextBuilder,
	toolConfig *ToolConfig,
) *ChatService {
	return &ChatService{
		claude:         claude,
		gemini:         gemini,
		convRepo:       convRepo,
		contextBuilder: contextBuilder,
		toolConfig:     toolConfig,
	}
}

// SetOwnerNotify registers a callback for notifying the bot owner about
// AI service failures. Non-blocking — caller should fire in a goroutine.
func (cs *ChatService) SetOwnerNotify(fn OwnerNotifyFunc) {
	cs.ownerNotify = fn
}

// HandleMessage processes a free-text user message through the AI pipeline.
// contentBlocks is non-nil when the message contains media (images, documents).
// onProgress is an optional callback for reporting status updates during tool round-trips.
// preferredModel is the user's model preference: "gemini" uses Gemini as primary, anything else uses Claude.
// showTokenInfo, when true, appends a compact token usage summary to Claude responses.
// claudeModelOverride (optional variadic) specifies the exact Claude model variant (e.g. "claude-sonnet-4-5").
// Thread-safe — passed via ChatRequest.OverrideModel, not shared state mutation.
// Returns the assistant's response text.
func (cs *ChatService) HandleMessage(ctx context.Context, userID int64, text string, role domain.UserRole, contentBlocks []ports.ContentBlock, onProgress func(string), preferredModel string, showTokenInfo bool, claudeModelOverride ...string) (string, error) {
	// 1. Load conversation history (last 20 messages for context window)
	history, err := cs.convRepo.GetHistory(ctx, userID, 20)
	if err != nil {
		chatLog.Warn().Err(err).Int64("user_id", userID).Msg("failed to load conversation history")
		history = nil // non-fatal — proceed without history
	}

	// Resolve the effective text for context building and history.
	// For multimodal messages without caption, extract text from content blocks
	// or generate a descriptive placeholder.
	effectiveText := text
	if effectiveText == "" && len(contentBlocks) > 0 {
		// Try to extract text from content blocks
		for _, b := range contentBlocks {
			if b.Type == "" {
				continue // skip zero-value blocks
			}
			if b.Type == "text" && b.Text != "" {
				effectiveText = b.Text
				break
			}
		}
		// If still empty, generate a descriptive label for context
		if effectiveText == "" {
			effectiveText = describeContentBlocks(contentBlocks)
		}
	}

	// 2. Build system prompt with market data injection
	systemPrompt := cs.contextBuilder.BuildSystemPrompt(ctx, effectiveText)

	// 3. Build messages array (history + current message)
	messages := make([]ports.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)

	// Build the current user message (multimodal or text-only)
	currentMsg := ports.ChatMessage{Role: "user"}
	if len(contentBlocks) > 0 {
		currentMsg.ContentBlocks = contentBlocks
	} else {
		currentMsg.Content = text
	}
	messages = append(messages, currentMsg)

	// 4. Resolve tools for user's tier
	tools := cs.toolConfig.ToolsForRole(role)

	// Resolve optional Claude model variant override.
	var modelOverride string
	if len(claudeModelOverride) > 0 && claudeModelOverride[0] != "" {
		modelOverride = claudeModelOverride[0]
	}

	// 5. Route to preferred model
	if preferredModel == "gemini" {
		return cs.handleGeminiPrimary(ctx, userID, systemPrompt, effectiveText, onProgress)
	}
	return cs.handleClaudePrimary(ctx, userID, messages, systemPrompt, tools, effectiveText, onProgress, showTokenInfo, modelOverride)
}

// handleClaudePrimary tries Claude first, then Gemini fallback, then template.
// modelOverride, if non-empty, overrides the server-default Claude model for this request only.
func (cs *ChatService) handleClaudePrimary(ctx context.Context, userID int64, messages []ports.ChatMessage, systemPrompt string, tools []ports.ServerTool, effectiveText string, onProgress func(string), showTokenInfo bool, modelOverride string) (string, error) {
	req := ports.ChatRequest{
		UserID:        userID,
		Messages:      messages,
		OverrideModel: modelOverride,
		SystemPrompt:  systemPrompt,
		Tools:         tools,
		OnProgress:    onProgress,
	}

	resp, err := cs.claude.Chat(ctx, req)
	if err == nil && resp.Content != "" {
		cs.saveConversation(ctx, userID, effectiveText, resp.Content)

		logEvent := chatLog.Info().
			Int64("user_id", userID).
			Int("input_tokens", resp.InputTokens).
			Int("output_tokens", resp.OutputTokens)

		if len(resp.ToolsUsed) > 0 {
			logEvent.Strs("tools_used", resp.ToolsUsed)
		}
		if resp.CacheReadTokens > 0 {
			logEvent.Int("cache_read", resp.CacheReadTokens)
		}

		logEvent.Msg("Claude response")

		content := resp.Content
		if showTokenInfo {
			content += fmt.Sprintf("\n\n<i>📊 Tokens: %d+%d | Cache: %d</i>",
				resp.InputTokens, resp.OutputTokens, resp.CacheReadTokens)
		}
		return content, nil
	}

	// Claude failed — log and attempt Gemini fallback
	if err != nil {
		chatLog.Warn().Err(err).Int64("user_id", userID).Msg("Claude failed, attempting Gemini fallback")
	} else {
		err = fmt.Errorf("empty response (no text content)")
		chatLog.Warn().Int64("user_id", userID).Msg("Claude returned empty response, attempting Gemini fallback")
	}

	// Try Gemini fallback — single-turn only (no history or multimodal support)
	if cs.gemini != nil && effectiveText != "" {
		geminiResp, geminiErr := cs.gemini.GenerateWithSystem(ctx, systemPrompt, effectiveText)
		if geminiErr == nil && geminiResp != "" {
			fallbackResponse := fmt.Sprintf(
				"<i>⚠️ Claude sedang tidak tersedia. Response ini dari model alternatif (Gemini).\n"+
					"Kualitas mungkin berbeda. Coba lagi dalam 5-10 menit untuk Claude.</i>\n\n%s",
				geminiResp,
			)
			cs.saveConversation(ctx, userID, effectiveText, geminiResp)
			chatLog.Info().Int64("user_id", userID).Msg("Gemini fallback succeeded")

			cs.notifyOwner(ctx, fmt.Sprintf(
				"⚠️ <b>Claude Fallback Triggered</b>\nUser: <code>%d</code>\nError: <code>%s</code>\nGemini fallback: ✅ succeeded",
				userID, truncateErr(err),
			))
			return fallbackResponse, nil
		}

		if geminiErr != nil {
			chatLog.Error().Err(geminiErr).Int64("user_id", userID).Msg("Gemini fallback also failed")
		}
	}

	// Template fallback (last resort)
	chatLog.Error().Int64("user_id", userID).Msg("all AI services unavailable — using template fallback")
	cs.notifyOwner(ctx, fmt.Sprintf(
		"🚨 <b>All AI Services Down</b>\nUser: <code>%d</code>\nClaude: <code>%s</code>\nGemini: unavailable\nFallback: template response sent",
		userID, truncateErr(err),
	))
	return templateFallback(), ErrAIFallback
}

// handleGeminiPrimary tries Gemini first, then Claude fallback, then template.
func (cs *ChatService) handleGeminiPrimary(ctx context.Context, userID int64, systemPrompt string, effectiveText string, onProgress func(string)) (string, error) {
	if cs.gemini != nil && effectiveText != "" {
		geminiResp, geminiErr := cs.gemini.GenerateWithSystem(ctx, systemPrompt, effectiveText)
		if geminiErr == nil && geminiResp != "" {
			cs.saveConversation(ctx, userID, effectiveText, geminiResp)
			chatLog.Info().Int64("user_id", userID).Msg("Gemini response (user preferred)")
			return geminiResp, nil
		}

		if geminiErr != nil {
			chatLog.Warn().Err(geminiErr).Int64("user_id", userID).Msg("Gemini failed (user preferred), attempting Claude fallback")
		}
	}

	// Gemini failed or unavailable — try Claude as fallback
	if cs.claude != nil && cs.claude.IsAvailable(ctx) {
		// Build simple single-turn message for fallback
		messages := []ports.ChatMessage{{Role: "user", Content: effectiveText}}
		req := ports.ChatRequest{
			UserID:       userID,
			Messages:     messages,
			SystemPrompt: systemPrompt,
			OnProgress:   onProgress,
		}

		resp, err := cs.claude.Chat(ctx, req)
		if err == nil && resp.Content != "" {
			fallbackResponse := fmt.Sprintf(
				"<i>[⚠️ Gemini unavailable — response via Claude fallback]</i>\n\n%s",
				resp.Content,
			)
			cs.saveConversation(ctx, userID, effectiveText, resp.Content)
			chatLog.Info().Int64("user_id", userID).Msg("Claude fallback succeeded (Gemini preferred)")
			return fallbackResponse, nil
		}

		if err != nil {
			chatLog.Error().Err(err).Int64("user_id", userID).Msg("Claude fallback also failed")
		}
	}

	// Template fallback
	chatLog.Error().Int64("user_id", userID).Msg("all AI services unavailable — using template fallback")
	cs.notifyOwner(ctx, fmt.Sprintf(
		"🚨 <b>All AI Services Down</b>\nUser: <code>%d</code>\nGemini: preferred, failed\nClaude: fallback, failed\nFallback: template response sent",
		userID,
	))
	return templateFallback(), ErrAIFallback
}

// ClearHistory wipes conversation history for a user.
func (cs *ChatService) ClearHistory(ctx context.Context, userID int64) error {
	return cs.convRepo.ClearHistory(ctx, userID)
}

// IsAvailable returns true if the primary chat engine (Claude) is configured.
func (cs *ChatService) IsAvailable(ctx context.Context) bool {
	return cs.claude != nil && cs.claude.IsAvailable(ctx)
}

// saveConversation persists both user message and assistant response.
func (cs *ChatService) saveConversation(ctx context.Context, userID int64, userMsg, assistantMsg string) {
	if err := cs.convRepo.AppendMessage(ctx, userID, ports.ChatMessage{
		Role:    "user",
		Content: userMsg,
	}); err != nil {
		chatLog.Warn().Err(err).Int64("user_id", userID).Msg("failed to save user message")
	}

	if err := cs.convRepo.AppendMessage(ctx, userID, ports.ChatMessage{
		Role:    "assistant",
		Content: assistantMsg,
	}); err != nil {
		chatLog.Warn().Err(err).Int64("user_id", userID).Msg("failed to save assistant message")
	}
}

// templateFallback returns a helpful message when all AI services are down.
func templateFallback() string {
	var b strings.Builder
	b.WriteString("<b>⚠️ AI services temporarily unavailable</b>\n\n")
	b.WriteString("Both Claude and Gemini are currently unreachable.\n")
	b.WriteString("You can still use these commands:\n\n")
	b.WriteString("/cot — COT positioning overview\n")
	b.WriteString("/outlook — Weekly market outlook\n")
	b.WriteString("/calendar — Economic calendar\n")
	b.WriteString("/macro — FRED macro dashboard\n")
	b.WriteString("/bias — Directional bias overview\n")
	b.WriteString("/rank — Currency strength ranking\n\n")
	b.WriteString("<i>Please try again later for AI chat features.</i>")
	return b.String()
}

// notifyOwner sends a notification to the bot owner if the callback is set.
// Non-blocking — fires in a goroutine with a timeout context to prevent
// goroutine leaks if the notification callback hangs.
func (cs *ChatService) notifyOwner(parentCtx context.Context, html string) {
	if cs.ownerNotify == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
		defer cancel()
		cs.ownerNotify(ctx, html)
	}()
}

// truncateErr returns a truncated error string (max 150 chars) safe for Telegram HTML.
// Returns "nil" for nil errors. Uses rune-based truncation to avoid splitting UTF-8.
func truncateErr(err error) string {
	if err == nil {
		return "nil"
	}
	s := err.Error()
	runes := []rune(s)
	if len(runes) > 150 {
		s = string(runes[:150]) + "..."
	}
	return s
}

// describeContentBlocks generates a descriptive label for multimodal content
// when no text was provided. Used for conversation history and context building.
func describeContentBlocks(blocks []ports.ContentBlock) string {
	var parts []string
	for _, b := range blocks {
		if b.Type == "" {
			continue // skip zero-value blocks
		}
		switch b.Type {
		case "image":
			parts = append(parts, "[Image]")
		case "document":
			if b.FileName != "" {
				parts = append(parts, fmt.Sprintf("[Document: %s]", b.FileName))
			} else {
				parts = append(parts, "[Document]")
			}
		}
	}
	if len(parts) == 0 {
		return "[Media message]"
	}
	return strings.Join(parts, " ")
}
