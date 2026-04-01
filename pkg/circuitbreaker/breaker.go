// Package circuitbreaker implements the circuit breaker pattern for external API calls.
// It prevents cascading failures by temporarily blocking requests to failing services.
//
// States:
//   - Closed: requests pass through normally. Failures are counted.
//   - Open: requests are immediately rejected. After resetTimeout, transitions to HalfOpen.
//   - HalfOpen: one probe request is allowed through. Success → Closed, Failure → Open.
//
// Usage:
//
//	cb := circuitbreaker.New("cftc-api", 5, 30*time.Second)
//	err := cb.Execute(func() error { return fetchData() })
package circuitbreaker

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// State represents the current state of the circuit breaker.
type State int

const (
	Closed   State = iota // Normal operation
	Open                  // Rejecting requests
	HalfOpen              // Probing with one request
)

// String returns a human-readable state name.
func (s State) String() string {
	switch s {
	case Closed:
		return "CLOSED"
	case Open:
		return "OPEN"
	case HalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// ErrCircuitOpen is returned when Execute is called while the breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// Breaker implements the circuit breaker pattern.
type Breaker struct {
	name         string
	maxFailures  int
	resetTimeout time.Duration

	mu          sync.Mutex
	state       State
	failures    int
	lastFailure time.Time
	lastSuccess time.Time

	// Optional callback when state changes
	OnStateChange func(name string, from, to State)
}

// New creates a circuit breaker.
//   - name: identifier for logging/metrics
//   - maxFailures: consecutive failures before opening the circuit
//   - resetTimeout: how long to wait before probing (half-open)
func New(name string, maxFailures int, resetTimeout time.Duration) *Breaker {
	return &Breaker{
		name:         name,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        Closed,
	}
}

// Execute runs fn through the circuit breaker.
// Returns ErrCircuitOpen if the breaker is open.
// On success, resets failure count. On failure, increments failure count.
func (b *Breaker) Execute(fn func() error) error {
	allowed, failures, retryAfter := b.checkRequest()
	if !allowed {
		return fmt.Errorf("%s: %w (failures=%d, retry after ~%ds)",
			b.name, ErrCircuitOpen, failures,
			int(retryAfter.Seconds()))
	}

	err := fn()

	if err != nil {
		b.recordFailure()
		return err
	}

	b.recordSuccess()
	return nil
}

// State returns the current state of the circuit breaker.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentState()
}

// Name returns the circuit breaker's name.
func (b *Breaker) Name() string {
	return b.name
}

// Failures returns the current consecutive failure count.
func (b *Breaker) Failures() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.failures
}

// Reset manually resets the circuit breaker to closed state.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	old := b.state
	b.state = Closed
	b.failures = 0
	if old != Closed && b.OnStateChange != nil {
		b.OnStateChange(b.name, old, Closed)
	}
}

// checkRequest checks if a request should be allowed through.
// Returns the allow decision plus a snapshot of failure count and retry duration
// (all read under the same lock to avoid races).
func (b *Breaker) checkRequest() (allowed bool, failures int, retryAfter time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.currentState() {
	case Closed:
		return true, b.failures, 0
	case HalfOpen:
		return true, b.failures, 0 // allow one probe
	case Open:
		remaining := b.resetTimeout - time.Since(b.lastFailure)
		if remaining < 0 {
			remaining = 0
		}
		return false, b.failures, remaining
	}
	return true, b.failures, 0
}

// currentState returns the effective state, handling Open→HalfOpen timeout transition.
// Must be called with b.mu held.
func (b *Breaker) currentState() State {
	if b.state == Open && time.Since(b.lastFailure) >= b.resetTimeout {
		b.setState(HalfOpen)
	}
	return b.state
}

// recordSuccess resets failure count and transitions to Closed.
func (b *Breaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.lastSuccess = time.Now()
	if b.state != Closed {
		b.setState(Closed)
	}
}

// recordFailure increments failure count and may open the circuit.
func (b *Breaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	b.lastFailure = time.Now()

	if b.state == HalfOpen {
		b.setState(Open)
		return
	}

	if b.failures >= b.maxFailures {
		b.setState(Open)
	}
}

// setState transitions to a new state and fires the callback.
// Must be called with b.mu held.
func (b *Breaker) setState(newState State) {
	old := b.state
	b.state = newState
	if old != newState && b.OnStateChange != nil {
		// Fire callback outside lock to avoid deadlock
		go b.OnStateChange(b.name, old, newState)
	}
}
