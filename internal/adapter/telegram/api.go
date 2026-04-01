package telegram

import (
	"bytes"
	encbase64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"context"

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// ports.Messenger Implementation — Outbound messaging
// ---------------------------------------------------------------------------

func (b *Bot) setChatID(params map[string]any, chatID string) {
	if strings.Contains(chatID, ":") {
		parts := strings.SplitN(chatID, ":", 2)
		params["chat_id"] = parts[0]
		if threadID, err := strconv.Atoi(parts[1]); err == nil {
			params["message_thread_id"] = threadID
		}
	} else {
		params["chat_id"] = chatID
	}
}

// SendMessage sends a plain text message.
func (b *Bot) SendMessage(ctx context.Context, chatID string, text string) (int, error) {
	if chatID == "" {
		chatID = b.defaultID
	}

	b.rateLimit()

	params := map[string]any{
		"text": text,
	}
	b.setChatID(params, chatID)

	var msg sentMessage
	if err := b.apiCallWithRetry(ctx, "sendMessage", params, &msg); err != nil {
		return 0, fmt.Errorf("sendMessage: %w", err)
	}
	return msg.MessageID, nil
}

// SendHTML sends an HTML-formatted message with link preview disabled.
// When the message is split into multiple chunks, each chunk receives a
// "Page X/Y" footer so the reader knows where they are.
// Overflow chunk IDs are tracked internally so that future EditMessage calls
// on the first chunk's ID can clean up the overflow messages.
func (b *Bot) SendHTML(ctx context.Context, chatID string, html string) (int, error) {
	if chatID == "" {
		chatID = b.defaultID
	}

	b.rateLimit()

	chunks := splitMessage(html, config.TelegramMaxMessageLen)
	total := len(chunks)
	var firstMsgID int
	var overflowIDs []int

	for i, chunk := range chunks {
		text := chunk
		if total > 1 {
			text += fmt.Sprintf("\n\n<i>— %d/%d —</i>", i+1, total)
		}
		params := map[string]any{
			"text":                     text,
			"parse_mode":               "HTML",
			"disable_web_page_preview": true,
		}
		b.setChatID(params, chatID)

		var msg sentMessage
		if err := b.apiCallWithRetry(ctx, "sendMessage", params, &msg); err != nil {
			// Track whatever overflow we sent so far.
			if firstMsgID != 0 {
				b.chunks.Record(chatID, firstMsgID, overflowIDs)
			}
			return firstMsgID, fmt.Errorf("sendHTML: %w", err)
		}
		if i == 0 {
			firstMsgID = msg.MessageID
		} else {
			overflowIDs = append(overflowIDs, msg.MessageID)
		}
	}

	// Track overflow chunk IDs keyed by the first message ID.
	b.chunks.Record(chatID, firstMsgID, overflowIDs)

	// Return the last message ID for backward compatibility.
	// Callers that only care about the final message (e.g., for keyboard
	// attachment) still get the expected value.
	if len(overflowIDs) > 0 {
		return overflowIDs[len(overflowIDs)-1], nil
	}
	return firstMsgID, nil
}

// sendHTMLRaw sends a single HTML message without splitting or page indicators.
// Used internally by chunked senders that handle their own splitting.
func (b *Bot) sendHTMLRaw(ctx context.Context, chatID string, html string) (int, error) {
	b.rateLimit()
	params := map[string]any{
		"text":                     html,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}
	b.setChatID(params, chatID)
	var msg sentMessage
	if err := b.apiCallWithRetry(ctx, "sendMessage", params, &msg); err != nil {
		return 0, fmt.Errorf("sendHTMLRaw: %w", err)
	}
	return msg.MessageID, nil
}

// SendWithKeyboard sends a message with an inline keyboard.
func (b *Bot) SendWithKeyboard(ctx context.Context, chatID string, text string, kb ports.InlineKeyboard) (int, error) {
	if chatID == "" {
		chatID = b.defaultID
	}

	b.rateLimit()

	keyboard := b.buildInlineKeyboard(kb)

	params := map[string]any{
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
		"reply_markup":             keyboard,
	}
	b.setChatID(params, chatID)

	var msg sentMessage
	if err := b.apiCallWithRetry(ctx, "sendMessage", params, &msg); err != nil {
		return 0, fmt.Errorf("sendWithKeyboard: %w", err)
	}
	return msg.MessageID, nil
}

// EditMessage edits an existing message's text.
// If the text exceeds Telegram's 4096-char limit, the first chunk replaces the
// original message and remaining chunks are sent as new follow-up messages.
func (b *Bot) EditMessage(ctx context.Context, chatID string, msgID int, text string) error {
	if chatID == "" {
		chatID = b.defaultID
	}

	// Delete any previously tracked overflow chunks for this message.
	// This prevents orphaned messages when a long response is re-edited.
	if oldOverflow := b.chunks.Pop(chatID, msgID); len(oldOverflow) > 0 {
		for _, id := range oldOverflow {
			_ = b.DeleteMessage(ctx, chatID, id) // best-effort cleanup
		}
	}

	chunks := splitMessage(text, config.TelegramMaxMessageLen)

	// Edit the original message with the first chunk.
	params := map[string]any{
		"message_id":               msgID,
		"text":                     chunks[0],
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}
	b.setChatID(params, chatID)

	if err := b.apiCallNoResult(ctx, "editMessageText", params); err != nil {
		return err
	}

	// Send any remaining chunks as new messages and track their IDs.
	var overflowIDs []int
	for _, chunk := range chunks[1:] {
		id, err := b.SendHTML(ctx, chatID, chunk)
		if err != nil {
			// Record whatever we managed to send so far for future cleanup.
			b.chunks.Record(chatID, msgID, overflowIDs)
			return fmt.Errorf("editMessage overflow chunk: %w", err)
		}
		overflowIDs = append(overflowIDs, id)
	}

	// Track overflow IDs so future edits of msgID can clean them up.
	b.chunks.Record(chatID, msgID, overflowIDs)

	return nil
}

// EditWithKeyboard edits message text and keyboard.
func (b *Bot) EditWithKeyboard(ctx context.Context, chatID string, msgID int, text string, kb ports.InlineKeyboard) error {
	if chatID == "" {
		chatID = b.defaultID
	}

	keyboard := b.buildInlineKeyboard(kb)

	params := map[string]any{
		"message_id":               msgID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
		"reply_markup":             keyboard,
	}
	b.setChatID(params, chatID)

	return b.apiCallNoResult(ctx, "editMessageText", params)
}

// AnswerCallback acknowledges a callback query.
func (b *Bot) AnswerCallback(ctx context.Context, callbackID string, text string) error {
	params := map[string]any{
		"callback_query_id": callbackID,
	}
	if text != "" {
		params["text"] = text
	}

	return b.apiCallNoResult(ctx, "answerCallbackQuery", params)
}

// DeleteMessage deletes a message.
func (b *Bot) DeleteMessage(ctx context.Context, chatID string, msgID int) error {
	if chatID == "" {
		chatID = b.defaultID
	}

	params := map[string]any{
		"message_id": msgID,
	}
	b.setChatID(params, chatID)

	return b.apiCallNoResult(ctx, "deleteMessage", params)
}

// SendTyping sends a "typing" chat action indicator. This shows the user that
// the bot is processing their request. The indicator automatically expires
// after ~5 seconds.
func (b *Bot) SendTyping(ctx context.Context, chatID string) {
	if chatID == "" {
		chatID = b.defaultID
	}
	params := map[string]any{
		"action": "typing",
	}
	b.setChatID(params, chatID)
	_ = b.apiCallNoResult(ctx, "sendChatAction", params)
}

// SendChatAction sends a chat action indicator to the given chat.
// Valid actions: "typing", "upload_photo", "record_video", "upload_video",
// "record_voice", "upload_voice", "upload_document", "choose_sticker",
// "find_location", "record_video_note", "upload_video_note".
// The indicator automatically disappears after ~5 seconds or when a message is sent.
func (b *Bot) SendChatAction(ctx context.Context, chatID string, action string) error {
	if chatID == "" {
		chatID = b.defaultID
	}
	params := map[string]any{
		"action": action,
	}
	b.setChatID(params, chatID)
	return b.apiCallNoResult(ctx, "sendChatAction", params)
}

// SendLoading sends a loading message and returns the message ID so it can be
// edited later with the final result. This provides immediate feedback for
// commands that take >2 seconds.
func (b *Bot) SendLoading(ctx context.Context, chatID string, text string) (int, error) {
	b.SendTyping(ctx, chatID)
	return b.SendHTML(ctx, chatID, text)
}

// ---------------------------------------------------------------------------
// Photo Sending (for chart images)
// ---------------------------------------------------------------------------

// SendPhoto sends a photo with optional HTML caption. photoData is raw PNG bytes.
// Returns the message ID of the sent photo message.
func (b *Bot) SendPhoto(ctx context.Context, chatID string, photoData []byte, caption string) (int, error) {
	return b.sendPhotoInternal(ctx, chatID, photoData, caption, nil)
}

// SendPhotoWithKeyboard sends a photo with HTML caption and inline keyboard.
// Returns the message ID of the sent photo message.
func (b *Bot) SendPhotoWithKeyboard(ctx context.Context, chatID string, photoData []byte, caption string, kb ports.InlineKeyboard) (int, error) {
	return b.sendPhotoInternal(ctx, chatID, photoData, caption, &kb)
}

// sendPhotoInternal handles multipart/form-data POST to Telegram sendPhoto API.
func (b *Bot) sendPhotoInternal(ctx context.Context, chatID string, photoData []byte, caption string, kb *ports.InlineKeyboard) (int, error) {
	if chatID == "" {
		chatID = b.defaultID
	}

	b.rateLimit()

	// Build multipart body
	var body bytes.Buffer
	boundary := fmt.Sprintf("----TGBoundary%d", time.Now().UnixNano())

	// Helper to write a form field
	writeField := func(name, value string) {
		body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"%s\"\r\n\r\n", name))
		body.WriteString(value)
		body.WriteString("\r\n")
	}

	// chat_id (handle thread IDs)
	if strings.Contains(chatID, ":") {
		parts := strings.SplitN(chatID, ":", 2)
		writeField("chat_id", parts[0])
		writeField("message_thread_id", parts[1])
	} else {
		writeField("chat_id", chatID)
	}

	// caption
	if caption != "" {
		// Telegram photo captions are limited to 1024 characters
		if len(caption) > 1024 {
			caption = caption[:1021] + "..."
		}
		writeField("caption", caption)
		writeField("parse_mode", "HTML")
	}

	// reply_markup (inline keyboard)
	if kb != nil {
		keyboard := b.buildInlineKeyboard(*kb)
		kbJSON, err := json.Marshal(keyboard)
		if err == nil {
			writeField("reply_markup", string(kbJSON))
		}
	}

	// photo file field
	body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	body.WriteString("Content-Disposition: form-data; name=\"photo\"; filename=\"chart.png\"\r\n")
	body.WriteString("Content-Type: image/png\r\n\r\n")
	body.Write(photoData)
	body.WriteString("\r\n")

	// closing boundary
	body.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	url := fmt.Sprintf("%s/sendPhoto", b.apiBase)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return 0, fmt.Errorf("create sendPhoto request: %w", err)
	}
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sendPhoto: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return 0, fmt.Errorf("read sendPhoto response: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return 0, fmt.Errorf("parse sendPhoto response: %w", err)
	}

	if !apiResp.OK {
		return 0, &apiError{
			Code:        apiResp.ErrorCode,
			Description: apiResp.Description,
			RetryAfter:  retryAfterFromResp(apiResp),
		}
	}

	var msg sentMessage
	if len(apiResp.Result) > 0 {
		if err := json.Unmarshal(apiResp.Result, &msg); err != nil {
			return 0, fmt.Errorf("unmarshal sendPhoto result: %w", err)
		}
	}

	return msg.MessageID, nil
}

// ---------------------------------------------------------------------------
// Proactive messaging (for alerts and scheduled reports)
// ---------------------------------------------------------------------------

// Broadcast sends an HTML message to the default chat.
// Used by the alerter and scheduled report services.
func (b *Bot) Broadcast(ctx context.Context, html string) (int, error) {
	return b.SendHTML(ctx, b.defaultID, html)
}

// BroadcastWithKeyboard sends a message with keyboard to the default chat.
func (b *Bot) BroadcastWithKeyboard(ctx context.Context, html string, kb ports.InlineKeyboard) (int, error) {
	return b.SendWithKeyboard(ctx, b.defaultID, html, kb)
}

// ---------------------------------------------------------------------------
// Telegram API helpers
// ---------------------------------------------------------------------------

// apiCall makes a Telegram Bot API call and unmarshals the result.
func (b *Bot) apiCall(ctx context.Context, method string, params map[string]any, result any) error {
	url := fmt.Sprintf("%s/%s", b.apiBase, method)

	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("api call %s: %w", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if !apiResp.OK {
		return &apiError{
			Code:        apiResp.ErrorCode,
			Description: apiResp.Description,
			RetryAfter:  retryAfterFromResp(apiResp),
		}
	}

	if result != nil && len(apiResp.Result) > 0 {
		if err := json.Unmarshal(apiResp.Result, result); err != nil {
			return fmt.Errorf("unmarshal result: %w", err)
		}
	}

	return nil
}

// apiError represents a Telegram API error with optional retry-after hint.
type apiError struct {
	Code        int
	Description string
	RetryAfter  int // seconds, 0 if not rate-limited
}

func (e *apiError) Error() string {
	return fmt.Sprintf("api error %d: %s", e.Code, e.Description)
}

// retryAfterFromResp extracts the retry_after value from a Telegram API response.
func retryAfterFromResp(resp apiResponse) int {
	if resp.Parameters != nil && resp.Parameters.RetryAfter > 0 {
		return resp.Parameters.RetryAfter
	}
	return 0
}

// maxRetries is the maximum number of retries for rate-limited API calls.
const maxRetries = 3

// apiCallWithRetry wraps apiCall with exponential backoff retry for 429 rate limits.
// Base delays: 5s, 10s, 20s (with jitter). If Telegram provides retry_after, that
// value is used instead. Gives up after maxRetries attempts.
func (b *Bot) apiCallWithRetry(ctx context.Context, method string, params map[string]any, result any) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := b.apiCall(ctx, method, params, result)
		if err == nil {
			return nil
		}

		ae, ok := err.(*apiError)
		if !ok || ae.Code != 429 {
			return err // non-retryable error
		}
		lastErr = err

		if attempt == maxRetries {
			break
		}

		// Determine wait duration: use Telegram's retry_after if provided,
		// otherwise exponential backoff (5s, 10s, 20s) + jitter.
		waitSec := ae.RetryAfter
		if waitSec <= 0 {
			waitSec = 5 * (1 << attempt) // 5, 10, 20
		}
		jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
		wait := time.Duration(waitSec)*time.Second + jitter

		log.Warn().
			Str("method", method).
			Int("attempt", attempt+1).
			Int("retry_after", waitSec).
			Dur("wait", wait).
			Msg("rate limited (429), retrying")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	return fmt.Errorf("rate limited after %d retries: %w", maxRetries, lastErr)
}

// apiCallNoResult makes a Telegram API call without parsing the result.
func (b *Bot) apiCallNoResult(ctx context.Context, method string, params map[string]any) error {
	return b.apiCallWithRetry(ctx, method, params, nil)
}

// buildInlineKeyboard converts ports.InlineKeyboard to Telegram API format.
func (b *Bot) buildInlineKeyboard(kb ports.InlineKeyboard) map[string]any {
	rows := make([][]map[string]string, 0, len(kb.Rows))
	for _, row := range kb.Rows {
		btnRow := make([]map[string]string, 0, len(row))
		for _, btn := range row {
			btnMap := map[string]string{"text": btn.Text}
			if btn.CallbackData != "" {
				btnMap["callback_data"] = btn.CallbackData
			}
			if btn.URL != "" {
				btnMap["url"] = btn.URL
			}
			btnRow = append(btnRow, btnMap)
		}
		rows = append(rows, btnRow)
	}
	return map[string]any{"inline_keyboard": rows}
}

// rateLimit ensures we don't exceed Telegram's rate limits.
// Min 35ms between messages to same chat (~28 msg/sec, under 30 limit).
func (b *Bot) rateLimit() {
	b.sendMu.Lock()
	defer b.sendMu.Unlock()

	sinceLastSend := time.Since(b.lastSend)
	if sinceLastSend < config.TelegramRateLimitDelay {
		time.Sleep(config.TelegramRateLimitDelay - sinceLastSend)
	}
	b.lastSend = time.Now()
}

// splitMessage splits a long message into chunks that fit Telegram's 4096 char limit.
// Splits on newlines when possible to preserve formatting.
// Handles unclosed <pre>, <b>, <i>, <code> tags at split boundaries by closing them
// in the current chunk and reopening them in the next chunk.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}

		// Find the best split point (last newline before maxLen)
		splitAt := maxLen
		if idx := strings.LastIndex(text[:maxLen], "\n"); idx > maxLen/2 {
			splitAt = idx + 1 // Include the newline in the first chunk
		}

		chunk := text[:splitAt]
		text = text[splitAt:]

		// Detect unclosed HTML tags and fix them at the split boundary.
		// We track <pre>, <b>, <i>, <code> — the tags Telegram supports.
		openTags := detectUnclosedTags(chunk)
		if len(openTags) > 0 {
			// Close open tags in reverse order at end of this chunk
			for i := len(openTags) - 1; i >= 0; i-- {
				chunk += "</" + openTags[i] + ">"
			}
			// Reopen tags at start of next chunk
			var reopen string
			for _, tag := range openTags {
				reopen += "<" + tag + ">"
			}
			text = reopen + text
		}

		chunks = append(chunks, chunk)
	}

	return chunks
}

// detectUnclosedTags returns a list of HTML tag names that are opened but not
// closed in the given text. Only tracks tags that Telegram's HTML parser supports.
func detectUnclosedTags(text string) []string {
	tracked := map[string]bool{"pre": true, "b": true, "i": true, "code": true}
	var stack []string

	for i := 0; i < len(text); i++ {
		if text[i] != '<' {
			continue
		}
		end := strings.IndexByte(text[i:], '>')
		if end < 0 {
			break
		}
		tag := text[i+1 : i+end]

		if strings.HasPrefix(tag, "/") {
			// Closing tag: pop from stack if it matches
			name := tag[1:]
			if tracked[name] && len(stack) > 0 && stack[len(stack)-1] == name {
				stack = stack[:len(stack)-1]
			}
		} else {
			// Opening tag (strip attributes, though Telegram tags rarely have them)
			name := tag
			if spIdx := strings.IndexByte(name, ' '); spIdx > 0 {
				name = name[:spIdx]
			}
			if tracked[name] {
				stack = append(stack, name)
			}
		}

		i += end // advance past '>'
	}

	return stack
}

// SendWithKeyboardChunked sends a potentially long HTML message with a keyboard.
// If the message exceeds 4000 chars, it sends intermediate chunks as plain HTML
// messages and attaches the keyboard only to the last chunk.
// Returns the message ID of the last sent message. Overflow chunk IDs are
// tracked so that future edits can clean up the extra messages.
func (b *Bot) SendWithKeyboardChunked(ctx context.Context, chatID string, html string, kb ports.InlineKeyboard) (int, error) {
	if chatID == "" {
		chatID = b.defaultID
	}

	chunks := splitMessage(html, 4000)
	total := len(chunks)

	if total == 1 {
		return b.SendWithKeyboard(ctx, chatID, chunks[0], kb)
	}

	// Send all but the last chunk as plain HTML with page indicators.
	var firstID int
	var overflowIDs []int
	for i, chunk := range chunks[:total-1] {
		text := chunk + fmt.Sprintf("\n\n<i>— %d/%d —</i>", i+1, total)
		id, err := b.sendHTMLRaw(ctx, chatID, text)
		if err != nil {
			if firstID != 0 {
				b.chunks.Record(chatID, firstID, overflowIDs)
			}
			return firstID, err
		}
		if i == 0 {
			firstID = id
		} else {
			overflowIDs = append(overflowIDs, id)
		}
	}

	// Last chunk gets the keyboard + page indicator.
	lastChunk := chunks[total-1] + fmt.Sprintf("\n\n<i>— %d/%d —</i>", total, total)
	id, err := b.SendWithKeyboard(ctx, chatID, lastChunk, kb)
	if err != nil {
		b.chunks.Record(chatID, firstID, overflowIDs)
		return firstID, err
	}
	overflowIDs = append(overflowIDs, id)

	// Track overflow keyed by first message ID.
	b.chunks.Record(chatID, firstID, overflowIDs)
	return id, nil
}

// EditWithKeyboardChunked edits the given message with the first chunk and sends
// additional chunks as new messages. Keyboard is attached to the last message.
// Old overflow chunks are cleaned up automatically via the chunk tracker.
func (b *Bot) EditWithKeyboardChunked(ctx context.Context, chatID string, msgID int, html string, kb ports.InlineKeyboard) error {
	if chatID == "" {
		chatID = b.defaultID
	}

	// Delete any previously tracked overflow chunks for this message.
	if oldOverflow := b.chunks.Pop(chatID, msgID); len(oldOverflow) > 0 {
		for _, id := range oldOverflow {
			_ = b.DeleteMessage(ctx, chatID, id)
		}
	}

	chunks := splitMessage(html, 4000)
	total := len(chunks)

	if total == 1 {
		return b.EditWithKeyboard(ctx, chatID, msgID, chunks[0], kb)
	}

	// Edit the original message with the first chunk + page indicator.
	first := chunks[0] + fmt.Sprintf("\n\n<i>— 1/%d —</i>", total)
	// Use apiCallNoResult directly to avoid EditMessage's own chunk-pop
	// (we already popped above).
	params := map[string]any{
		"message_id":               msgID,
		"text":                     first,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}
	b.setChatID(params, chatID)
	if err := b.apiCallNoResult(ctx, "editMessageText", params); err != nil {
		return err
	}

	// Send intermediate chunks and track their IDs.
	var overflowIDs []int
	for i, chunk := range chunks[1 : total-1] {
		text := chunk + fmt.Sprintf("\n\n<i>— %d/%d —</i>", i+2, total)
		id, err := b.sendHTMLRaw(ctx, chatID, text)
		if err != nil {
			b.chunks.Record(chatID, msgID, overflowIDs)
			return err
		}
		overflowIDs = append(overflowIDs, id)
	}

	// Last chunk as new message with keyboard + page indicator.
	lastChunk := chunks[total-1] + fmt.Sprintf("\n\n<i>— %d/%d —</i>", total, total)
	id, err := b.SendWithKeyboard(ctx, chatID, lastChunk, kb)
	if err != nil {
		b.chunks.Record(chatID, msgID, overflowIDs)
		return err
	}
	overflowIDs = append(overflowIDs, id)

	// Track all overflow IDs for future cleanup.
	b.chunks.Record(chatID, msgID, overflowIDs)
	return nil
}

// ---------------------------------------------------------------------------
// File Download (for multimodal chatbot support)
// ---------------------------------------------------------------------------

// telegramFile represents the response from Telegram's getFile API.
type telegramFile struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
	FileSize int    `json:"file_size,omitempty"`
}

// getFilePath resolves a Telegram file_id to a downloadable file_path.
func (b *Bot) getFilePath(ctx context.Context, fileID string) (string, error) {
	params := map[string]any{
		"file_id": fileID,
	}
	var f telegramFile
	if err := b.apiCallWithRetry(ctx, "getFile", params, &f); err != nil {
		return "", fmt.Errorf("getFile: %w", err)
	}
	return f.FilePath, nil
}

// maxFileDownloadBytes limits file downloads to 5MB.
// Claude supports up to ~5MB per image in base64. Telegram Bot API allows up to 20MB,
// but larger files would waste bandwidth and get rejected by the AI API.
const maxFileDownloadBytes = 5 * 1024 * 1024

// downloadFileBase64 downloads a Telegram file by file_id and returns its base64-encoded data and MIME type.
// Files exceeding 5MB are rejected to stay within Claude's input limits.
func (b *Bot) downloadFileBase64(ctx context.Context, fileID string) (string, string, error) {
	filePath, err := b.getFilePath(ctx, fileID)
	if err != nil {
		return "", "", err
	}

	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.token, filePath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create download request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download file: status %d", resp.StatusCode)
	}

	// Read up to maxFileDownloadBytes + 1 byte to detect oversized files
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFileDownloadBytes+1))
	if err != nil {
		return "", "", fmt.Errorf("read file: %w", err)
	}
	if len(data) > maxFileDownloadBytes {
		return "", "", fmt.Errorf("file too large: exceeds %dMB limit", maxFileDownloadBytes/(1024*1024))
	}

	// Detect MIME type from Content-Type header or file extension
	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = detectMIMEType(filePath)
	}

	encoded := base64Encode(data)
	return encoded, mimeType, nil
}

// detectMIMEType guesses MIME type from file extension.
func detectMIMEType(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(lower, ".svg"):
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// base64Encode encodes raw bytes to a standard base64 string.
func base64Encode(data []byte) string {
	return encbase64.StdEncoding.EncodeToString(data)
}
