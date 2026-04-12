package saferun_test

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/saferun"
	"github.com/rs/zerolog"
)

func TestGo_NoPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	var wg sync.WaitGroup
	wg.Add(1)

	saferun.Go(context.Background(), "test-ok", logger, func() {
		defer wg.Done()
		// no panic — should produce no log
	})

	wg.Wait()

	if buf.Len() != 0 {
		t.Errorf("expected no log output, got: %s", buf.String())
	}
}

func TestGo_RecoversPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	// The panic will trigger the deferred recover in saferun.Go,
	// which logs AFTER the panicking function returns.
	// We use a small sleep to let the goroutine finish entirely.
	saferun.Go(context.Background(), "test-panic", logger, func() {
		panic("boom")
	})

	// Give the goroutine time to panic + recover + log.
	time.Sleep(100 * time.Millisecond)

	output := buf.String()
	if output == "" {
		t.Fatal("expected log output for recovered panic")
	}
	if !bytes.Contains(buf.Bytes(), []byte("PANIC recovered")) {
		t.Errorf("expected 'PANIC recovered' in log, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("test-panic")) {
		t.Errorf("expected goroutine name 'test-panic' in log, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("boom")) {
		t.Errorf("expected panic value 'boom' in log, got: %s", output)
	}
}
