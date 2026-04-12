// Package keyring provides a thread-safe rotating API key pool.
// It uses atomic round-robin selection so all goroutines share the same
// counter without mutex overhead.
package keyring

import (
	"errors"
	"sync/atomic"
)

// ErrNoKeys is returned when the keyring has no keys configured.
var ErrNoKeys = errors.New("keyring: no API keys configured")

// Keyring holds a pool of API keys with atomic round-robin selection.
type Keyring struct {
	keys  []string
	index uint64
}

// New creates a Keyring from a slice of keys.
// Returns an empty keyring (not an error) if keys is nil/empty.
func New(keys []string) *Keyring {
	return &Keyring{keys: keys}
}

// Next returns the next key in round-robin order.
// Returns ErrNoKeys if no keys are configured.
func (k *Keyring) Next() (string, error) {
	if len(k.keys) == 0 {
		return "", ErrNoKeys
	}
	i := atomic.AddUint64(&k.index, 1)
	return k.keys[int(i-1)%len(k.keys)], nil
}

// MustNext returns the next key.
// Returns ErrNoKeys if no keys are configured (no panic - safe for production).
func (k *Keyring) MustNext() (string, error) {
	return k.Next()
}

// Len returns the number of keys.
func (k *Keyring) Len() int { return len(k.keys) }

// IsEmpty returns true if no keys are configured.
func (k *Keyring) IsEmpty() bool { return len(k.keys) == 0 }
