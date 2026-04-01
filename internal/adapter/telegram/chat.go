package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Chat Message Handling (non-command / chatbot)
// ---------------------------------------------------------------------------

// handleChatMessage processes a non-command message via the chatbot handler.
// Supports text, photo, document, and voice messages.
// If no handler is registered, the message is silently ignored.
// Only active in private chats — group messages are ignored to prevent
// unintended Claude API costs and noise.
func (b *Bot) handleChatMessage(ctx context.Context, msg *Message) {
	if b.freeTextHandler == nil {
		return // No chatbot configured — silently ignore
	}

	// Only respond to free-text in private chats.
	// In groups/supergroups, non-command messages are silently ignored to
	// avoid triggering Claude API calls on every group message.
	if msg.Chat.Type != "private" {
		return
	}

	// Determine text content (Text for text messages, Caption for media)
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		text = strings.TrimSpace(msg.Caption)
	}

	// Build content blocks for media messages
	var contentBlocks []ports.ContentBlock

	// Handle photo messages (pick the largest size)
	if len(msg.Photo) > 0 {
		largest := msg.Photo[len(msg.Photo)-1] // Telegram sends sizes in ascending order
		data, mimeType, err := b.downloadFileBase64(ctx, largest.FileID)
		if err != nil {
			log.Error().Err(err).Msg("failed to download photo")
			if strings.Contains(err.Error(), "too large") {
				// Notify user about size limit
				chatID := strconv.FormatInt(msg.Chat.ID, 10)
				_, _ = b.SendHTML(ctx, chatID, "Image is too large (max 5MB). Please send a smaller image.")
				return
			}
		} else {
			if mimeType == "" {
				mimeType = "image/jpeg"
			}
			contentBlocks = append(contentBlocks, ports.ContentBlock{
				Type:      "image",
				MediaType: mimeType,
				Data:      data,
			})
		}
	}

	// Handle document uploads
	if msg.Document != nil {
		data, mimeType, err := b.downloadFileBase64(ctx, msg.Document.FileID)
		if err != nil {
			log.Error().Err(err).Msg("failed to download document")
			if strings.Contains(err.Error(), "too large") {
				chatID := strconv.FormatInt(msg.Chat.ID, 10)
				_, _ = b.SendHTML(ctx, chatID, "Document is too large (max 5MB). Please send a smaller file.")
				return
			}
		} else {
			if mimeType == "" {
				mimeType = msg.Document.MimeType
			}
			// Determine block type based on MIME
			blockType := "document"
			if strings.HasPrefix(mimeType, "image/") {
				blockType = "image"
			}
			contentBlocks = append(contentBlocks, ports.ContentBlock{
				Type:      blockType,
				MediaType: mimeType,
				Data:      data,
				FileName:  msg.Document.FileName,
			})
		}
	}

	// Handle voice messages
	if msg.Voice != nil {
		// Voice messages aren't directly supported by Claude vision API,
		// but we note them in the text so the AI can acknowledge
		if text == "" {
			text = "[Voice message received — voice transcription not yet supported]"
		} else {
			text = text + "\n[Voice message also attached — voice transcription not yet supported]"
		}
	}

	// If no text and no usable content blocks, ignore
	if text == "" && len(contentBlocks) == 0 {
		return
	}

	// If we have media, add the text as a content block too
	if len(contentBlocks) > 0 && text != "" {
		// Prepend text block before image/document blocks
		textBlock := ports.ContentBlock{Type: "text", Text: text}
		contentBlocks = append([]ports.ContentBlock{textBlock}, contentBlocks...)
	}

	chatID := strconv.FormatInt(msg.Chat.ID, 10)
	if msg.MessageThreadID != 0 {
		chatID = fmt.Sprintf("%s:%d", chatID, msg.MessageThreadID)
	}
	userID := int64(0)
	username := ""
	if msg.From != nil {
		userID = msg.From.ID
		username = msg.From.Username
	}

	// Authorization via middleware (ban + rate limit only — no command counter increment).
	// AI quota is checked separately in HandleFreeText via CheckAIQuota.
	if userID != 0 && b.middleware != nil {
		result := b.middleware.AuthorizeCallback(ctx, userID)
		if !result.Allowed {
			log.Warn().Int64("user_id", userID).Str("reason", result.Reason).Msg("chat message denied by middleware")
			_, _ = b.SendHTML(ctx, chatID, fmt.Sprintf("\u26d4 %s", result.Reason))
			return
		}
	} else if userID != 0 && !b.isOwner(userID) {
		if allowed, retryAfter := b.userLimiter.Allow(userID); !allowed {
			waitSec := int(retryAfter.Seconds())
			msg := fmt.Sprintf("⏳ Batas request tercapai. Coba lagi dalam ~%d detik.", waitSec)
			_, _ = b.SendHTML(ctx, chatID, msg)
			return
		}
	}

	log.Info().Int64("user_id", userID).Str("chat_id", chatID).
		Int("content_blocks", len(contentBlocks)).Msg("chat message received")
	if err := b.freeTextHandler(ctx, chatID, userID, username, text, contentBlocks); err != nil {
		log.Error().Err(err).Int64("user_id", userID).Msg("chat handler error")
		_, _ = b.SendHTML(ctx, chatID, "Error processing message. Please try again later or use /help.")
	}
}
