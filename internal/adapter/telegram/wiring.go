package telegram

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// compile-time interface check
var _ ports.Messenger = (*Bot)(nil)

// NewBot creates a new Telegram bot with the given token and default chat ID.
// The default chat ID is also used to identify the bot owner (exempt from rate limits).
// For private chats, chat ID == user ID. For groups (negative IDs), owner derivation
// is skipped — the owner must be set via OWNER_ID env or identified at runtime.
func NewBot(token, defaultChatID string) *Bot {
	// Check dedicated OWNER_ID env var first, then fall back to defaultChatID.
	var ownerID int64
	if ownerStr := strings.TrimSpace(os.Getenv("OWNER_ID")); ownerStr != "" {
		if parsed, err := strconv.ParseInt(ownerStr, 10, 64); err == nil && parsed > 0 {
			ownerID = parsed
		}
	} else {
		// Legacy: parse owner ID from default chat ID.
		// Only treat it as an owner ID if it's a positive number (private chat).
		rawID := strings.Split(defaultChatID, ":")[0]
		if parsed, err := strconv.ParseInt(rawID, 10, 64); err == nil && parsed > 0 {
			ownerID = parsed
		}
	}

	return &Bot{
		token:     token,
		defaultID: defaultChatID,
		ownerID:   ownerID,
		apiBase:   fmt.Sprintf("https://api.telegram.org/bot%s", token),
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // long-polling timeout + buffer
		},
		commands:    make(map[string]CommandHandler),
		callbacks:   make(map[string]CallbackHandler),
		userLimiter: newUserRateLimiter(),
	}
}
