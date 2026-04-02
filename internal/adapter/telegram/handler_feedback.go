package telegram

// handler_feedback.go — 👍/👎 reaction feedback on analysis messages (TASK-051)

import (
	"context"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/adapter/storage"
	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// WithFeedback wires the feedback repository and registers the fb: callback.
func (h *Handler) WithFeedback(repo *storage.FeedbackRepo) {
	h.feedbackRepo = repo
	h.bot.RegisterCallback("fb:", h.cbFeedback)
	log.Info().Msg("Feedback buttons enabled (👍/👎)")
}

// feedbackEnabled returns true when the feedback repo is wired.
func (h *Handler) feedbackEnabled() bool {
	return h.feedbackRepo != nil
}

// cbFeedback handles fb:<type>:<key>:<rating> callbacks.
// Example: "fb:cot:099741:up", "fb:outlook:latest:down"
func (h *Handler) cbFeedback(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// Strip "fb:" prefix
	rest := strings.TrimPrefix(data, "fb:")

	// Parse: <type>:<key>:<rating>
	parts := strings.SplitN(rest, ":", 3)
	if len(parts) != 3 {
		return nil
	}

	analysisType := parts[0]
	analysisKey := parts[1]
	rating := parts[2]

	if rating != "up" && rating != "down" {
		return nil
	}

	if h.feedbackRepo == nil {
		return nil
	}

	fb := &domain.Feedback{
		UserID:       userID,
		AnalysisType: analysisType,
		AnalysisKey:  analysisKey,
		Rating:       rating,
		CreatedAt:    time.Now(),
	}

	if err := h.feedbackRepo.Save(ctx, fb); err != nil {
		log.Error().Err(err).
			Str("type", analysisType).
			Str("key", analysisKey).
			Str("rating", rating).
			Int64("user_id", userID).
			Msg("failed to save feedback")
		return nil // swallow — don't break UX
	}

	log.Info().
		Str("type", analysisType).
		Str("key", analysisKey).
		Str("rating", rating).
		Int64("user_id", userID).
		Msg("feedback saved")

	// The bot dispatcher calls AnswerCallback(cb.ID, "") on nil return,
	// which dismisses the loading spinner. Feedback is silently recorded.
	return nil
}
