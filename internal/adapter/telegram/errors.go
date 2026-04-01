package telegram

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

// userFriendlyError translates a technical error into a user-friendly message
// with an actionable suggestion. The original error should be logged separately
// via zerolog for debugging; the user sees only a clean, helpful message.
func userFriendlyError(err error, command string) string {
	if err == nil {
		return ""
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	// Timeout / context deadline
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "context canceled") {
		return "⏱ <b>Request timeout</b>\n\nServer membutuhkan waktu lebih lama dari biasanya. Coba lagi dalam beberapa saat.\n\n💡 <i>Tip: " + suggestRetry(command) + "</i>"
	}

	// Data not found / key not found
	if strings.Contains(lower, "not found") || strings.Contains(lower, "no data") || strings.Contains(lower, "key not found") || strings.Contains(lower, "no record") {
		return "📭 <b>Data belum tersedia</b>\n\nData yang diminta belum ada di sistem. Kemungkinan belum di-fetch atau belum dipublish.\n\n💡 <i>Tip: Coba lagi nanti atau gunakan command lain.</i>"
	}

	// Insufficient data
	if strings.Contains(lower, "insufficient") || strings.Contains(lower, "not enough") {
		return "📊 <b>Data belum cukup</b>\n\nBelum ada cukup data historis untuk menjalankan analisis ini.\n\n💡 <i>Tip: Coba lagi setelah data terkumpul lebih banyak.</i>"
	}

	// Network / connection errors
	if strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host") || strings.Contains(lower, "dial tcp") || strings.Contains(lower, "unreachable") {
		return "🌐 <b>Koneksi gagal</b>\n\nTidak bisa terhubung ke server data eksternal. Kemungkinan ada gangguan jaringan.\n\n💡 <i>Tip: Coba lagi dalam 1-2 menit.</i>"
	}

	// API rate limit / quota
	if strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") || strings.Contains(lower, "429") || strings.Contains(lower, "quota") {
		return "🚦 <b>Batas request tercapai</b>\n\nTerlalu banyak request dalam waktu singkat. Sistem perlu jeda sebentar.\n\n💡 <i>Tip: Tunggu 1-2 menit lalu coba lagi.</i>"
	}

	// Chart / rendering errors (checked before AI to avoid "failed" matching "ai")
	if strings.Contains(lower, "chart") || strings.Contains(lower, "render") || strings.Contains(lower, "script") {
		return "📈 <b>Gagal membuat chart</b>\n\nTerjadi masalah saat rendering visualisasi.\n\n💡 <i>Tip: " + suggestRetry(command) + "</i>"
	}

	// AI / generation errors
	if strings.Contains(lower, " ai ") || strings.Contains(lower, "gemini") || strings.Contains(lower, "claude") || strings.Contains(lower, "generation failed") || strings.Contains(lower, "openai") || strings.Contains(lower, "llm") {
		return "🤖 <b>AI sedang tidak tersedia</b>\n\nLayanan AI sedang mengalami gangguan. Analisis template tetap tersedia.\n\n💡 <i>Tip: " + suggestRetry(command) + "</i>"
	}

	// Permission / auth errors
	if strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "403") || strings.Contains(lower, "401") {
		return "🔒 <b>Akses ditolak</b>\n\nKamu tidak memiliki izin untuk fitur ini, atau API key sedang bermasalah.\n\n💡 <i>Tip: Hubungi admin jika ini tidak seharusnya terjadi.</i>"
	}

	// BadgerDB errors
	if strings.Contains(lower, "badger") {
		return "💾 <b>Gangguan penyimpanan</b>\n\nDatabase internal sedang bermasalah. Data kamu aman.\n\n💡 <i>Tip: Coba lagi dalam beberapa saat. Hubungi admin jika terus terjadi.</i>"
	}

	// Generic fallback
	return "⚠️ <b>Terjadi kesalahan</b>\n\nMaaf, terjadi kesalahan yang tidak terduga.\n\n💡 <i>Tip: " + suggestRetry(command) + " Hubungi admin jika terus terjadi.</i>"
}

// suggestRetry returns a retry suggestion based on the command context.
func suggestRetry(command string) string {
	if command == "" {
		return "Coba ulangi command yang sama."
	}
	return "Coba ulangi dengan /" + command + "."
}

// sendUserError logs the technical error and sends a user-friendly message.
// This is the main entry point for error handling in handlers.
func (h *Handler) sendUserError(ctx context.Context, chatID string, err error, command string) {
	log.Error().
		Err(err).
		Str("chat_id", chatID).
		Str("command", command).
		Msg("handler error")

	friendly := userFriendlyError(err, command)
	if _, sendErr := h.bot.SendHTML(ctx, chatID, friendly); sendErr != nil {
		log.Error().Err(sendErr).Str("chat_id", chatID).Msg("failed to send error message")
	}
}

// editUserError logs the technical error and edits an existing message with a user-friendly error.
func (h *Handler) editUserError(ctx context.Context, chatID string, msgID int, err error, command string) {
	log.Error().
		Err(err).
		Str("chat_id", chatID).
		Str("command", command).
		Msg("handler error")

	friendly := userFriendlyError(err, command)
	if editErr := h.bot.EditMessage(ctx, chatID, msgID, friendly); editErr != nil {
		log.Error().Err(editErr).Str("chat_id", chatID).Msg("failed to edit error message")
	}
}

// stripHTML removes HTML tags from a string, yielding plain text.
var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	return htmlTagRe.ReplaceAllString(s, "")
}

// toastFromFriendly converts a userFriendlyError HTML message into a compact
// plain-text toast suitable for Telegram's AnswerCallbackQuery (≤200 chars).
// It strips HTML tags, collapses whitespace/newlines, and truncates with "...".
func toastFromFriendly(friendly string) string {
	plain := stripHTML(friendly)
	// Collapse newlines and multiple spaces into single space.
	plain = strings.Join(strings.Fields(plain), " ")
	plain = strings.TrimSpace(plain)

	const maxToastLen = 200
	runes := []rune(plain)
	if len(runes) > maxToastLen {
		runes = runes[:maxToastLen-1]
		plain = string(runes) + "\u2026"
	} else {
		plain = string(runes)
	}
	return plain
}

// callbackFriendlyError returns a compact (≤200 chars) plain-text error message
// suitable for display in a Telegram AnswerCallback toast notification.
// Telegram's AnswerCallbackQuery `text` field is limited to 200 characters and
// must be plain text (no HTML/Markdown).
//
// It routes through userFriendlyError for consistency, then strips HTML and truncates.
func callbackFriendlyError(err error) string {
	if err == nil {
		return ""
	}
	return toastFromFriendly(userFriendlyError(err, ""))
}

// sessionExpiredMessage returns a unified session-expired message for any command.
// All handlers should use this instead of hardcoding their own expired text.
// The output uses consistent emoji (⏳), Indonesian language, and <code> tags.
func sessionExpiredMessage(command string) string {
	return "⏳ <b>Sesi berakhir</b>\n\nData sudah expired. Ketik <code>/" + command + "</code> untuk memulai ulang."
}

