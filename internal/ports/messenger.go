package ports

import "context"

// ---------------------------------------------------------------------------
// Messenger — Telegram message delivery interface
// ---------------------------------------------------------------------------

// InlineKeyboard represents a Telegram inline keyboard for callbacks.
type InlineKeyboard struct {
	Rows [][]InlineButton `json:"rows"`
}

// InlineButton represents a single inline keyboard button.
type InlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

// Messenger defines the interface for sending messages via Telegram.
// Implementations handle the raw Telegram Bot API communication.
type Messenger interface {
	// SendMessage sends a plain text message to a chat.
	SendMessage(ctx context.Context, chatID string, text string) (int, error)

	// SendHTML sends an HTML-formatted message.
	SendHTML(ctx context.Context, chatID string, html string) (int, error)

	// SendWithKeyboard sends a message with an inline keyboard.
	SendWithKeyboard(ctx context.Context, chatID string, text string, kb InlineKeyboard) (int, error)

	// EditMessage edits an existing message's text.
	EditMessage(ctx context.Context, chatID string, msgID int, text string) error

	// EditWithKeyboard edits an existing message's text and keyboard.
	EditWithKeyboard(ctx context.Context, chatID string, msgID int, text string, kb InlineKeyboard) error

	// AnswerCallback acknowledges an inline keyboard callback.
	AnswerCallback(ctx context.Context, callbackID string, text string) error

	// DeleteMessage deletes a message.
	DeleteMessage(ctx context.Context, chatID string, msgID int) error
}
