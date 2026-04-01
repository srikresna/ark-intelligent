package telegram

// Chatbot — Free-text & /clear

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	aisvc "github.com/arkcode369/ark-intelligent/internal/service/ai"
)

// ---------------------------------------------------------------------------
// Chatbot — Free-text message handling via Claude
// ---------------------------------------------------------------------------

// HandleFreeText processes non-command messages through the Claude chatbot pipeline.
// This is registered as the Bot's FreeTextHandler during wiring.
func (h *Handler) HandleFreeText(ctx context.Context, chatID string, userID int64, username string, text string, contentBlocks []ports.ContentBlock) error {
	if h.chatService == nil {
		// No chatbot configured — send a helpful hint
		_, err := h.bot.SendHTML(ctx, chatID,
			"I only respond to commands for now. Type /help for available commands.")
		return err
	}

	// Check per-user chat cooldown BEFORE quota check to avoid consuming
	// AI quota on cooldown-blocked requests (owner bypassed — unlimited access).
	if !h.bot.isOwner(userID) && !h.checkChatCooldown(userID) {
		_, err := h.bot.SendHTML(ctx, chatID,
			"\u23f3 Please wait a moment before sending another message.")
		return err
	}

	// Check AI quota via middleware (after cooldown so blocked requests don't waste quota)
	if h.middleware != nil {
		allowed, reason := h.middleware.CheckAIQuota(ctx, userID)
		if !allowed {
			_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("\u26d4 %s", reason))
			return err
		}
	}

	// Send "thinking" indicator
	thinkMsgID, _ := h.bot.SendMessage(ctx, chatID, "\u2699\ufe0f Thinking...")

	// Progress callback: updates the "thinking" message with tool activity status.
	// This lets the user see what the model is doing (e.g. "Searching the web...")
	// instead of staring at a static "Thinking..." message.
	onProgress := func(status string) {
		if thinkMsgID > 0 {
			_ = h.bot.EditMessage(ctx, chatID, thinkMsgID, status)
		}
	}

	// Get user role and preferred model for routing
	role := domain.RoleFree
	preferredModel := ""
	if h.middleware != nil {
		profile := h.middleware.GetUserProfile(ctx, userID)
		if profile != nil {
			role = profile.Role
		}
	}
	var claudeModelOverride string
	if prefs, err := h.prefsRepo.Get(ctx, userID); err == nil {
		preferredModel = prefs.PreferredModel
		// Pass specific Claude model variant if user selected one
		if prefs.ClaudeModel != "" && domain.IsValidClaudeModel(prefs.ClaudeModel) {
			claudeModelOverride = string(prefs.ClaudeModel)
		}
	}

	// Call chat service. No blanket timeout — the Claude HTTP client already has
	// a per-request timeout (default 120s) that handles hung requests.
	// As long as Claude keeps responding (tool round-trips), let it work freely.
	response, err := h.chatService.HandleMessage(ctx, userID, text, role, contentBlocks, onProgress, preferredModel, claudeModelOverride)

	// Delete "thinking" indicator
	if thinkMsgID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, thinkMsgID)
	}

	// Handle template fallback: still send the response but refund the AI quota
	// since no real AI call succeeded.
	if errors.Is(err, aisvc.ErrAIFallback) {
		if h.middleware != nil {
			h.middleware.RefundAIQuota(ctx, userID)
		}
		// Send the template fallback content (err contains ErrAIFallback but response is valid)
		if _, sendErr := h.bot.SendHTML(ctx, chatID, response); sendErr != nil {
			_, sendErr = h.bot.SendMessage(ctx, chatID, response)
			return sendErr
		}
		return nil
	}

	if err != nil {
		log.Error().Err(err).Int64("user_id", userID).Msg("chat service error")
		// Refund AI quota on total failure (context timeout, etc.)
		if h.middleware != nil {
			h.middleware.RefundAIQuota(ctx, userID)
		}
		_, sendErr := h.bot.SendHTML(ctx, chatID,
			"Error processing your message. Please try again later or use /help.")
		return sendErr
	}

	// Send response (Claude follows HTML constraints via system prompt).
	// Fall back to plain text if Telegram rejects the HTML (malformed tags, etc.)
	if _, err = h.bot.SendHTML(ctx, chatID, response); err != nil {
		log.Warn().Err(err).Int64("user_id", userID).Msg("SendHTML failed for chat response, falling back to plain text")
		_, err = h.bot.SendMessage(ctx, chatID, response)
	}
	return err
}

// checkChatCooldown returns true if the user is allowed to send a chat message (cooldown elapsed).
// Updates the cooldown timestamp if allowed.
// Uses a separate map from AI command cooldown to avoid interference.
func (h *Handler) checkChatCooldown(userID int64) bool {
	h.chatCooldownMu.Lock()
	defer h.chatCooldownMu.Unlock()

	now := time.Now()

	// Opportunistic cleanup: remove stale entries when map grows large.
	if len(h.chatCooldown) > 100 {
		cutoff := now.Add(-5 * time.Minute)
		for uid, ts := range h.chatCooldown {
			if ts.Before(cutoff) {
				delete(h.chatCooldown, uid)
			}
		}
	}

	if last, ok := h.chatCooldown[userID]; ok {
		// Use a 5-second cooldown for chat messages
		if now.Sub(last) < 5*time.Second {
			return false
		}
	}
	h.chatCooldown[userID] = now
	return true
}

// cmdClearChat handles the /clear command to wipe conversation history.
func (h *Handler) cmdClearChat(ctx context.Context, chatID string, userID int64, _ string) error {
	if h.chatService == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Chat mode is not enabled.")
		return err
	}

	if err := h.chatService.ClearHistory(ctx, userID); err != nil {
		log.Error().Err(err).Int64("user_id", userID).Msg("clear chat history failed")
		_, sendErr := h.bot.SendHTML(ctx, chatID, "Failed to clear chat history. Please try again.")
		return sendErr
	}

	_, err := h.bot.SendHTML(ctx, chatID,
		"\u2705 Chat history cleared. Starting fresh conversation.")
	return err
}
