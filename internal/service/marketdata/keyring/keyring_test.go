package keyring

import (
	"testing"
)

func TestKeyring_New(t *testing.T) {
	tests := []struct {
		name      string
		keys      []string
		wantLen   int
		wantEmpty bool
	}{
		{
			name:      "with keys",
			keys:      []string{"key1", "key2", "key3"},
			wantLen:   3,
			wantEmpty: false,
		},
		{
			name:      "empty keys",
			keys:      []string{},
			wantLen:   0,
			wantEmpty: true,
		},
		{
			name:      "nil keys",
			keys:      nil,
			wantLen:   0,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := New(tt.keys)
			if got := k.Len(); got != tt.wantLen {
				t.Errorf("New().Len() = %v, want %v", got, tt.wantLen)
			}
			if got := k.IsEmpty(); got != tt.wantEmpty {
				t.Errorf("New().IsEmpty() = %v, want %v", got, tt.wantEmpty)
			}
		})
	}
}

func TestKeyring_Next(t *testing.T) {
	t.Run("round robin with keys", func(t *testing.T) {
		keys := []string{"key1", "key2", "key3"}
		k := New(keys)

		// Should cycle through keys in order
		for i := 0; i < 6; i++ {
			key, err := k.Next()
			if err != nil {
				t.Errorf("Next() unexpected error: %v", err)
			}
			expected := keys[i%len(keys)]
			if key != expected {
				t.Errorf("Next() = %v, want %v", key, expected)
			}
		}
	})

	t.Run("error when empty", func(t *testing.T) {
		k := New([]string{})
		_, err := k.Next()
		if err != ErrNoKeys {
			t.Errorf("Next() error = %v, want ErrNoKeys", err)
		}
	})
}

// TestKeyring_MustNext_SafeForProduction verifies the critical fix:
// MustNext must NOT panic when keyring is empty (PHI-SEC-001)
func TestKeyring_MustNext_SafeForProduction(t *testing.T) {
	t.Run("returns key without panic when keys exist", func(t *testing.T) {
		k := New([]string{"api-key-1"})
		key, err := k.MustNext()
		if err != nil {
			t.Errorf("MustNext() unexpected error: %v", err)
		}
		if key != "api-key-1" {
			t.Errorf("MustNext() = %v, want api-key-1", key)
		}
	})

	t.Run("returns error without panic when empty - CRITICAL FIX", func(t *testing.T) {
		k := New([]string{})
		// This should NOT panic - it should return an error gracefully
		key, err := k.MustNext()
		if err != ErrNoKeys {
			t.Errorf("MustNext() error = %v, want ErrNoKeys", err)
		}
		if key != "" {
			t.Errorf("MustNext() key = %v, want empty string", key)
		}
		// If we reach here without panic, the fix is working
	})

	t.Run("round robin through MustNext", func(t *testing.T) {
		keys := []string{"a", "b", "c"}
		k := New(keys)

		for i := 0; i < 9; i++ {
			key, err := k.MustNext()
			if err != nil {
				t.Errorf("MustNext() unexpected error at iteration %d: %v", i, err)
			}
			expected := keys[i%len(keys)]
			if key != expected {
				t.Errorf("MustNext() at iteration %d = %v, want %v", i, key, expected)
			}
		}
	})
}

func TestKeyring_ConcurrentAccess(t *testing.T) {
	k := New([]string{"key1", "key2", "key3"})

	// Run concurrent goroutines to test thread safety
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			defer func() { done <- true }()
			_, err := k.Next()
			if err != nil {
				t.Errorf("Concurrent Next() error: %v", err)
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

func BenchmarkKeyring_Next(b *testing.B) {
	k := New([]string{"key1", "key2", "key3", "key4", "key5"})
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = k.Next()
		}
	})
}
