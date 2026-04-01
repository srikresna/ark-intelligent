package telegram

import (
	"testing"
	"time"
)

func TestProgressIndicator_EmptySteps(t *testing.T) {
	p := NewProgress(nil, "123", nil)
	if p.MessageID() != 0 {
		t.Error("expected 0 msgID before Start")
	}
	// Start with nil bot and empty steps should not panic.
	id := p.Start(nil)
	if id != 0 {
		t.Errorf("expected 0 from Start with empty steps, got %d", id)
	}
}

func TestProgressIndicator_WithInterval(t *testing.T) {
	p := NewProgress(nil, "123", []string{"step1"}, WithInterval(5*time.Second))
	if p.interval != 5*time.Second {
		t.Errorf("expected 5s interval, got %v", p.interval)
	}
}

func TestProgressIndicator_StopIdempotent(t *testing.T) {
	p := NewProgress(nil, "123", []string{"step1"})
	// Stop without Start should not panic.
	p.Stop(nil)
	p.Stop(nil) // second call should be safe
}

func TestProgressIndicator_StopNoDelete(t *testing.T) {
	p := NewProgress(nil, "123", []string{"step1"})
	p.StopNoDelete()
	// Verify done flag is set.
	p.mu.Lock()
	if !p.done {
		t.Error("expected done=true after StopNoDelete")
	}
	p.mu.Unlock()
}

func TestProgressIndicator_DefaultInterval(t *testing.T) {
	p := NewProgress(nil, "123", []string{"a", "b"})
	if p.interval != 3*time.Second {
		t.Errorf("expected default 3s interval, got %v", p.interval)
	}
}
