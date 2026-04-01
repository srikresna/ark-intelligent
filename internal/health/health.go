// Package health provides an HTTP health check endpoint.
// It exposes /health and /ready for container orchestration (Docker, K8s).
//
//   - /health — liveness check (always 200 if process is running)
//   - /ready  — readiness check (200 if DB is accessible and bot is polling)
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("health")

// Checker holds references needed for readiness checks.
type Checker struct {
	dbPing  func() error // checks DB accessibility
	started time.Time
	ready   atomic.Bool
}

// New creates a health checker. dbPing should return nil if the DB is healthy.
func New(dbPing func() error) *Checker {
	c := &Checker{
		dbPing:  dbPing,
		started: time.Now(),
	}
	c.ready.Store(true)
	return c
}

// SetReady updates the readiness state.
func (c *Checker) SetReady(ready bool) {
	c.ready.Store(ready)
}

// Start launches the HTTP health server on the given address (e.g., ":8080").
// Blocks until the context is cancelled, then shuts down gracefully.
func (c *Checker) Start(ctx context.Context, addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", c.handleHealth)
	mux.HandleFunc("/ready", c.handleReady)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("health server started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("health server error")
		}
	}()

	<-ctx.Done()

	shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	log.Info().Msg("health server stopped")
}

// handleHealth is the liveness probe — always 200 if process is running.
func (c *Checker) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "alive",
		"uptime": time.Since(c.started).String(),
	})
}

// handleReady is the readiness probe — checks DB and service state.
func (c *Checker) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	checks := map[string]string{}
	healthy := true

	// Check overall readiness flag
	if !c.ready.Load() {
		checks["service"] = "not ready"
		healthy = false
	} else {
		checks["service"] = "ok"
	}

	// Check DB
	if c.dbPing != nil {
		if err := c.dbPing(); err != nil {
			checks["db"] = err.Error()
			healthy = false
		} else {
			checks["db"] = "ok"
		}
	}

	status := http.StatusOK
	statusStr := "ready"
	if !healthy {
		status = http.StatusServiceUnavailable
		statusStr = "not ready"
	}

	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": statusStr,
		"checks": checks,
		"uptime": time.Since(c.started).String(),
	})
}
