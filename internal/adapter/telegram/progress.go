package telegram

// progress.go — Multi-step progress indicator for long-running commands.
// Replaces static "Computing..." messages with animated step-by-step updates,
// giving users visible feedback that the bot is actively processing.

import (
	"context"
	"sync"
	"time"
)

// ProgressIndicator manages a multi-step loading message that auto-advances
// every few seconds. It sends an initial message and then edits it with
// successive step texts, so the user sees the bot is still working.
type ProgressIndicator struct {
	bot    *Bot
	chatID string
	steps  []string

	interval time.Duration

	mu    sync.Mutex
	msgID int
	done  bool
}

// ProgressOption configures a ProgressIndicator.
type ProgressOption func(*ProgressIndicator)

// WithInterval sets the step advancement interval (default 3s).
func WithInterval(d time.Duration) ProgressOption {
	return func(p *ProgressIndicator) { p.interval = d }
}

// NewProgress creates a ProgressIndicator. Call Start to begin, then Stop when
// the work is complete. If steps is empty or has one entry, it degrades to a
// plain SendLoading call (no ticker).
func NewProgress(bot *Bot, chatID string, steps []string, opts ...ProgressOption) *ProgressIndicator {
	p := &ProgressIndicator{
		bot:      bot,
		chatID:   chatID,
		steps:    steps,
		interval: 3 * time.Second,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Start sends the first step message and launches a background goroutine that
// edits subsequent steps on the configured interval. The context controls
// cancellation of the background ticker.
func (p *ProgressIndicator) Start(ctx context.Context) int {
	if len(p.steps) == 0 {
		return 0
	}

	// Send initial step (with typing indicator).
	msgID, _ := p.bot.SendLoading(ctx, p.chatID, p.steps[0])
	p.mu.Lock()
	p.msgID = msgID
	p.mu.Unlock()

	if len(p.steps) <= 1 || msgID == 0 {
		return msgID
	}

	// Advance through remaining steps in background.
	go p.run(ctx)
	return msgID
}

// run is the background loop that edits the message with successive steps.
func (p *ProgressIndicator) run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	step := 1 // step 0 already sent

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.mu.Lock()
			if p.done {
				p.mu.Unlock()
				return
			}
			msgID := p.msgID
			p.mu.Unlock()

			if step < len(p.steps) {
				_ = p.bot.EditMessage(ctx, p.chatID, msgID, p.steps[step])
				step++
			}
			// After last step, keep ticking but don't edit anymore —
			// the caller will Stop() when the work finishes.
			if step >= len(p.steps) {
				return
			}
		}
	}
}

// Stop marks the indicator as done and deletes the loading message.
// Safe to call multiple times.
func (p *ProgressIndicator) Stop(ctx context.Context) {
	p.mu.Lock()
	if p.done {
		p.mu.Unlock()
		return
	}
	p.done = true
	msgID := p.msgID
	p.mu.Unlock()

	if msgID > 0 {
		_ = p.bot.DeleteMessage(ctx, p.chatID, msgID)
	}
}

// StopNoDelete marks the indicator as done without deleting the message.
// Useful when the caller wants to edit the loading message into the result.
func (p *ProgressIndicator) StopNoDelete() {
	p.mu.Lock()
	p.done = true
	p.mu.Unlock()
}

// MessageID returns the sent message ID (0 if Start was never called or failed).
func (p *ProgressIndicator) MessageID() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.msgID
}
