package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"runtime/debug"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
	"github.com/arkcode369/ark-intelligent/pkg/metrics"
)

var log = logger.Component("telegram")

// ---------------------------------------------------------------------------
// Telegram Bot API types
// ---------------------------------------------------------------------------

// Update represents an incoming Telegram update.
type Update struct {
	UpdateID      int            `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

// Message represents a Telegram message.
type Message struct {
	MessageID       int         `json:"message_id"`
	Chat            Chat        `json:"chat"`
	From            *User       `json:"from,omitempty"`
	Text            string      `json:"text"`
	Caption         string      `json:"caption,omitempty"`           // Caption for media messages
	Date            int64       `json:"date"`
	MessageThreadID int         `json:"message_thread_id,omitempty"` // For groups with topics
	Photo           []PhotoSize `json:"photo,omitempty"`             // Photo messages (multiple sizes)
	Document        *Document   `json:"document,omitempty"`          // Document/file messages
	Voice           *Voice      `json:"voice,omitempty"`             // Voice messages
}

// PhotoSize represents one size variant of a photo.
type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int    `json:"file_size,omitempty"`
}

// Document represents a document/file sent via Telegram.
type Document struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int    `json:"file_size,omitempty"`
}

// Voice represents a voice message.
type Voice struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Duration     int    `json:"duration"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int    `json:"file_size,omitempty"`
}

// Chat represents a Telegram chat.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"` // "private", "group", "supergroup", "channel"
}

// User represents a Telegram user.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// CallbackQuery represents a callback from an inline keyboard button press.
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data"`
}

// apiResponse wraps the Telegram Bot API response format.
type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Parameters  *responseParams `json:"parameters,omitempty"`
}

// responseParams holds optional response parameters from Telegram (e.g. retry_after).
type responseParams struct {
	RetryAfter int `json:"retry_after,omitempty"`
}

// sentMessage is the minimal response from sendMessage.
type sentMessage struct {
	MessageID int `json:"message_id"`
}

// ---------------------------------------------------------------------------
// Bot — Core Telegram bot engine
// ---------------------------------------------------------------------------


// CommandHandler handles a specific bot command.
type CommandHandler func(ctx context.Context, chatID string, userID int64, args string) error

// CallbackHandler handles an inline keyboard callback.
type CallbackHandler func(ctx context.Context, chatID string, msgID int, userID int64, data string) error

// FreeTextHandler handles non-command (chatbot) messages.
// contentBlocks is non-nil when the message contains media (images, documents).
type FreeTextHandler func(ctx context.Context, chatID string, userID int64, username string, text string, contentBlocks []ports.ContentBlock) error

// Bot is the Telegram bot engine that handles polling, message routing,
// and implements ports.Messenger for outbound message delivery.
type Bot struct {
	token      string
	defaultID  string // default chat ID
	ownerID    int64  // owner user ID (exempt from rate limits)
	apiBase    string
	httpClient *http.Client

	// Command routing
	commands  map[string]CommandHandler
	callbacks map[string]CallbackHandler // prefix-based routing

	// Free-text (chatbot) handler — nil means ignore non-command messages
	freeTextHandler FreeTextHandler

	// Polling state
	offset int

	// Rate limiting: Telegram allows ~30 msg/sec to same chat
	sendMu   sync.Mutex
	lastSend time.Time

	// Per-user command rate limiter (used for callback rate limiting)
	userLimiter *userRateLimiter

	// Authorization middleware (tiered access control + quotas)
	middleware *Middleware

	// Worker pool semaphore: limits concurrent handleUpdate goroutines.
	// Buffered channel used as semaphore; capacity = max concurrent handlers.
	// Default capacity is config.MaxConcurrentHandlers (20).
	// Overridable via HANDLER_CONCURRENCY env var.
	workerSem chan struct{}

	// Chunk tracker: records overflow message IDs for multi-part messages
	// so that subsequent edits can clean up old overflow chunks.
	chunks *chunkTracker

	// postCommandHook is called after every successful command execution.
	// Used by the onboarding progress tracker.
	postCommandHook func(ctx context.Context, chatID string, userID int64, cmd string)
}

// ---------------------------------------------------------------------------
// Command & Callback Registration
// ---------------------------------------------------------------------------

// RegisterCommand registers a handler for a slash command (e.g., "/today").
func (b *Bot) RegisterCommand(cmd string, handler CommandHandler) {
	if !strings.HasPrefix(cmd, "/") {
		cmd = "/" + cmd
	}
	b.commands[cmd] = handler
	log.Info().Str("command", cmd).Msg("registered command")
}

// RegisterCallback registers a handler for callbacks matching a prefix.
// E.g., RegisterCallback("cot:", handler) matches "cot:USD", "cot:EUR", etc.
func (b *Bot) RegisterCallback(prefix string, handler CallbackHandler) {
	b.callbacks[prefix] = handler
	log.Info().Str("prefix", prefix).Msg("registered callback prefix")
}

// ---------------------------------------------------------------------------
// Long-Polling Loop
// ---------------------------------------------------------------------------

// StartPolling begins the long-polling loop. Blocks until ctx is cancelled.
func (b *Bot) StartPolling(ctx context.Context) error {
	log.Info().Msg("starting long-polling loop")

	for {
		select {
		case <-ctx.Done():
			log.Info().Err(ctx.Err()).Msg("polling stopped")
			return ctx.Err()
		default:
		}

		updates, err := b.getUpdates(ctx, b.offset, 100, 30)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Error().Err(err).Msg("getUpdates error, retrying in 5s")
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			b.offset = update.UpdateID + 1
			// Acquire a worker slot before spawning the goroutine.
			// The select ensures the polling loop exits immediately on context
			// cancellation even when all slots are occupied.
			// Log a warning when the pool is at capacity (backpressure indicator).
			if len(b.workerSem) == cap(b.workerSem) {
				log.Warn().
					Int("pool_capacity", cap(b.workerSem)).
					Int("pool_used", len(b.workerSem)).
					Msg("worker pool full — update queued (backpressure)")
			}
			select {
			case b.workerSem <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}
			go func(u Update) {
				defer func() { <-b.workerSem }()
				b.handleUpdate(ctx, u)
			}(update)
		}
	}
}

// getUpdates calls the Telegram getUpdates API with long polling.
func (b *Bot) getUpdates(ctx context.Context, offset, limit, timeout int) ([]Update, error) {
	params := map[string]any{
		"offset":          offset,
		"limit":           limit,
		"timeout":         timeout,
		"allowed_updates": []string{"message", "callback_query"},
	}

	var updates []Update
	if err := b.apiCallWithRetry(ctx, "getUpdates", params, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

// handleUpdate routes an incoming update to the appropriate handler.
func (b *Bot) handleUpdate(ctx context.Context, update Update) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Interface("panic", r).
				Str("stack", string(debug.Stack())).
				Msg("PANIC recovered in handleUpdate — bot continues running")
		}
	}()

	if update.CallbackQuery != nil {
		b.handleCallback(ctx, update.CallbackQuery)
		return
	}

	if update.Message != nil {
		msg := update.Message
		// Route text messages (commands or free-text)
		if msg.Text != "" {
			b.handleMessage(ctx, msg)
			return
		}
		// Route media messages (photo, document, voice) to chatbot handler
		if len(msg.Photo) > 0 || msg.Document != nil || msg.Voice != nil {
			b.handleChatMessage(ctx, msg)
			return
		}
	}
}

// handleMessage routes text messages to command handlers or chatbot.
func (b *Bot) handleMessage(ctx context.Context, msg *Message) {
	text := strings.TrimSpace(msg.Text)
	if !strings.HasPrefix(text, "/") {
		// Non-command message → route to chatbot handler (if registered)
		b.handleChatMessage(ctx, msg)
		return
	}

	// Parse command and arguments: "/today USD" -> cmd="/today", args="USD"
	parts := strings.SplitN(text, " ", 2)
	cmd := strings.ToLower(parts[0])

	// Strip @BotUsername suffix for group commands: "/today@FFCalendarBot" -> "/today"
	// If command is targeted at a different bot, ignore it entirely.
	if at := strings.Index(cmd, "@"); at > 0 {
		cmd = cmd[:at]
	}

	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
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

	isGroup := msg.Chat.Type == "group" || msg.Chat.Type == "supergroup"

	// Check if command exists BEFORE authorization (so unknown commands don't consume quota)
	handler, ok := b.commands[cmd]
	if !ok {
		// In group chats, silently ignore unknown commands to avoid interfering
		// with other bots that may handle them.
		if isGroup {
			return
		}
		log.Warn().Str("command", cmd).Int64("user_id", userID).Msg("unknown command")
		if _, err := b.SendHTML(ctx, chatID, fmt.Sprintf(
			"Unknown command <code>%s</code>\nType /help for available commands.",
			html.EscapeString(cmd),
		)); err != nil {
			log.Error().Err(err).Str("chat_id", chatID).Msg("failed to send unknown-command message")
		}
		return
	}

	// Authorization via middleware (tiered rate limiting + quotas)
	if userID != 0 && b.middleware != nil {
		result := b.middleware.Authorize(ctx, userID, username, cmd)
		if !result.Allowed {
			log.Warn().Int64("user_id", userID).Str("command", cmd).Str("reason", result.Reason).Msg("user denied by middleware")
			if _, err := b.SendHTML(ctx, chatID, fmt.Sprintf("\xe2\x9b\x94 %s", result.Reason)); err != nil {
				log.Error().Err(err).Str("chat_id", chatID).Msg("failed to send authorization-denied message")
			}
			return
		}
	} else if userID != 0 && !b.isOwner(userID) {
		if allowed, retryAfter := b.userLimiter.Allow(userID); !allowed {
			// Fallback to legacy rate limiter if middleware not installed
			log.Warn().Int64("user_id", userID).Str("command", cmd).Msg("user rate limited")
			waitSec := int(retryAfter.Seconds())
			msg := fmt.Sprintf("⏳ Batas request tercapai. Coba lagi dalam ~%d detik.", waitSec)
			if _, err := b.SendHTML(ctx, chatID, msg); err != nil {
				log.Error().Err(err).Str("chat_id", chatID).Msg("failed to send rate-limited message")
			}
			return
		}
	}

	log.Info().Str("command", cmd).Int64("user_id", userID).Str("chat_id", chatID).Msg("command received")
	start := time.Now()
	err := handler(ctx, chatID, userID, args)
	elapsed := time.Since(start)

	// Structured metrics logging (see pkg/metrics for format docs)
	metrics.RecordCommand(cmd, userID, elapsed, err)

	// Post-command hook (onboarding progress tracking)
	if err == nil && b.postCommandHook != nil && userID != 0 {
		b.postCommandHook(ctx, chatID, userID, cmd)
	}

	if err != nil {
		if _, err := b.SendHTML(ctx, chatID,
			fmt.Sprintf("Error processing <code>%s</code>. Please try again later.", html.EscapeString(cmd))); err != nil {
			log.Error().Err(err).Str("chat_id", chatID).Msg("failed to send error message")
		}
	}
}

// handleCallback routes inline keyboard callbacks.
func (b *Bot) handleCallback(ctx context.Context, cb *CallbackQuery) {
	chatID := ""
	msgID := 0
	if cb.Message != nil {
		chatID = strconv.FormatInt(cb.Message.Chat.ID, 10)
		msgID = cb.Message.MessageID
	}

	userID := cb.From.ID
	log.Info().Str("data", cb.Data).Int64("user_id", userID).Msg("callback received")

	// Guard: cb.Message can be nil when the message is too old (>48h) or deleted.
	// Without a valid chatID, handlers cannot send responses to Telegram.
	// Return a session-expired toast so the user knows to re-issue the command.
	if chatID == "" {
		log.Warn().
			Str("data", cb.Data).
			Int64("user_id", userID).
			Msg("callback with nil message — session expired or message deleted")
		_ = b.AnswerCallback(ctx, cb.ID, "⏳ Session expired. Please resend the command.")
		return
	}

	// Authorization via middleware (ban check + tiered rate limit)
	if b.middleware != nil {
		result := b.middleware.AuthorizeCallback(ctx, userID)
		if !result.Allowed {
			log.Warn().Int64("user_id", userID).Str("data", cb.Data).Str("reason", result.Reason).Msg("callback denied by middleware")
			_ = b.AnswerCallback(ctx, cb.ID, result.Reason)
			return
		}
	} else if !b.isOwner(userID) {
		if allowed, retryAfter := b.userLimiter.Allow(userID); !allowed {
			// Fallback to legacy rate limiter if middleware not installed
			log.Warn().Int64("user_id", userID).Str("data", cb.Data).Msg("user rate limited (callback)")
			waitSec := int(retryAfter.Seconds())
			msg := fmt.Sprintf("⏳ Batas request tercapai. Coba lagi dalam ~%d detik.", waitSec)
			_ = b.AnswerCallback(ctx, cb.ID, msg)
			return
		}
	}

	// Find handler by prefix match
	for prefix, handler := range b.callbacks {
		if strings.HasPrefix(cb.Data, prefix) {
			cbStart := time.Now()
			cbErr := handler(ctx, chatID, msgID, userID, cb.Data)
			cbElapsed := time.Since(cbStart)

			// Structured metrics logging (see pkg/metrics for format docs)
			metrics.RecordCallback(cb.Data, userID, cbElapsed, cbErr)

			if cbErr != nil {
				log.Error().
					Err(cbErr).
					Str("callback_data", cb.Data).
					Int64("user_id", userID).
					Msg("callback handler error")

				fullFriendly := userFriendlyError(cbErr, "")
				toast := toastFromFriendly(fullFriendly)
				_ = b.AnswerCallback(ctx, cb.ID, toast)

				// If we have a message to edit, also update it with the full
				// user-friendly error so the user can read it at their own pace.
				if chatID != "" && msgID != 0 {
					_ = b.EditMessage(ctx, chatID, msgID, fullFriendly)
				}
				return
			}
			_ = b.AnswerCallback(ctx, cb.ID, "")
			return
		}
	}

	log.Warn().Str("data", cb.Data).Msg("unhandled callback")
	_ = b.AnswerCallback(ctx, cb.ID, "Unknown action")
}

// ---------------------------------------------------------------------------
// Configuration & Accessors
// ---------------------------------------------------------------------------

// DefaultChatID returns the configured default chat ID.
func (b *Bot) DefaultChatID() string {
	return b.defaultID
}

// OwnerID returns the bot owner's user ID.
func (b *Bot) OwnerID() int64 {
	return b.ownerID
}

// isOwner returns true if the given user ID matches the bot owner.
func (b *Bot) isOwner(userID int64) bool {
	return b.ownerID != 0 && userID == b.ownerID
}

// SetMiddleware installs the authorization middleware.
// Must be called before StartPolling.
func (b *Bot) SetMiddleware(mw *Middleware) {
	b.middleware = mw
}

// SetFreeTextHandler registers a handler for non-command (chatbot) messages.
// If not set, non-command messages are silently ignored (backward compatible).
func (b *Bot) SetFreeTextHandler(handler FreeTextHandler) {
	b.freeTextHandler = handler
}

// StopRateLimiter stops the legacy per-user rate limiter's background cleanup goroutine.
// Should be called during graceful shutdown.
func (b *Bot) StopRateLimiter() {
	if b.userLimiter != nil {
		b.userLimiter.Stop()
	}
}

// handleChatMessage is defined in chat.go.

// SetPostCommandHook registers a hook called after every successful command.
func (b *Bot) SetPostCommandHook(hook func(ctx context.Context, chatID string, userID int64, cmd string)) {
	b.postCommandHook = hook
}
