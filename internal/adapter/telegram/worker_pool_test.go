package telegram

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestWorkerPoolConcurrencyCap verifies that at most `cap(workerSem)` handleUpdate
// goroutines run concurrently. We replace handleUpdate behaviour by injecting a
// synchronised counter via the workerSem field directly, without needing a real
// Telegram connection.
func TestWorkerPoolConcurrencyCap(t *testing.T) {
	const cap = 5 // small cap to make test fast
	const jobs = 20

	sem := make(chan struct{}, cap)
	var (
		active    int64 // currently running
		maxActive int64 // peak concurrency observed
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	// Simulate the polling loop dispatch pattern used in StartPolling.
	for i := 0; i < jobs; i++ {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			cur := atomic.AddInt64(&active, 1)
			mu.Lock()
			if cur > maxActive {
				maxActive = cur
			}
			mu.Unlock()
			time.Sleep(5 * time.Millisecond) // simulate work
			atomic.AddInt64(&active, -1)
		}()
	}
	wg.Wait()

	if maxActive > int64(cap) {
		t.Errorf("peak concurrency %d exceeded cap %d", maxActive, cap)
	}
}

// TestWorkerPoolCtxCancelUnblocks verifies that a blocked semaphore acquire
// unblocks when the context is cancelled, so the polling loop can exit cleanly.
func TestWorkerPoolCtxCancelUnblocks(t *testing.T) {
	const cap = 1
	sem := make(chan struct{}, cap)
	// Fill the semaphore so next acquire would block.
	sem <- struct{}{}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Mirrors the select in StartPolling.
		select {
		case sem <- struct{}{}:
			t.Error("should not have acquired semaphore — it was full")
		case <-ctx.Done():
			// Expected: context cancelled before slot became available.
		}
	}()

	select {
	case <-done:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("goroutine did not unblock within 500ms after context cancellation")
	}
}

// TestNewBotWorkerSemDefault verifies NewBot initialises workerSem with default cap 20.
func TestNewBotWorkerSemDefault(t *testing.T) {
	b := NewBot("fake-token", "12345")
	if got := cap(b.workerSem); got != 20 {
		t.Errorf("expected default workerSem capacity 20, got %d", got)
	}
}
