package telegram

import (
	"sync"
	"time"
)

// deepLinkIntent stores a pending command to auto-execute after onboarding.
type deepLinkIntent struct {
	Command string // e.g. "cot"
	Args    string // e.g. "EUR"
	Expires time.Time
}

// deepLinkCache is a thread-safe TTL cache for deep link command intents.
type deepLinkCache struct {
	mu    sync.Mutex
	items map[int64]*deepLinkIntent
}

func newDeepLinkCache() *deepLinkCache {
	return &deepLinkCache{items: make(map[int64]*deepLinkIntent)}
}

// Set stores a deep link intent for a user with a 10-minute TTL.
func (c *deepLinkCache) Set(userID int64, command, args string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[userID] = &deepLinkIntent{
		Command: command,
		Args:    args,
		Expires: time.Now().Add(10 * time.Minute),
	}
}

// Pop retrieves and removes a deep link intent for a user.
// Returns nil if no intent exists or it has expired.
func (c *deepLinkCache) Pop(userID int64) *deepLinkIntent {
	c.mu.Lock()
	defer c.mu.Unlock()
	intent, ok := c.items[userID]
	if !ok {
		return nil
	}
	delete(c.items, userID)
	if time.Now().After(intent.Expires) {
		return nil
	}
	return intent
}
