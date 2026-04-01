package telegram

import (
	"context"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/adapter/storage"
	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// WithFeedback injects the FeedbackRepo for reaction buttons.
// When nil, feedback buttons are not shown and callbacks are ignored.
func (h *Handler) WithFeedback(repo *storage.FeedbackRepo) {
	h.feedbackRepo = repo
	if repo != nil {
		h.bot.RegisterCallback("fb:", h.cbFeedback)
		log.Info().Msg("Feedback reaction buttons enabled")
	}
}

// cbFeedback handles thumbs up/down callback data.
// Format: "fb:<type>:<key>:<rating>" e.g. "fb:cot:EUR:up", "fb:outlook:latest:down"
func (h *Handler) cbFeedback(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// data is already stripped of the "fb:" prefix by the router
	parts := strings.SplitN(data, ":", 3)
	if len(parts) < 3 {
		return h.bot.AnswerCallback(ctx, "", "Invalid feedback")
	}

	analysisType := parts[0] // "cot", "outlook", "bias", etc.
	analysisKey := parts[1]  // "EUR", "latest", etc.
	rating := parts[2]       // "up" or "down"

	if rating != "up" && rating != "down" {
		return h.bot.AnswerCallback(ctx, "", "Invalid rating")
	}

	fb := &domain.Feedback{
		UserID:       userID,
		AnalysisType: analysisType,
		AnalysisKey:  analysisKey,
		Rating:       rating,
		CreatedAt:    time.Now(),
	}

	if err := h.feedbackRepo.Save(ctx, fb); err != nil {
		log.Error().Err(err).Str("type", analysisType).Str("key", analysisKey).Msg("failed to save feedback")
		return h.bot.AnswerCallback(ctx, "", "Gagal menyimpan feedback")
	}

	toast := "\u2705 Feedback diterima!"
	return h.bot.AnswerCallback(ctx, "", toast)
}
