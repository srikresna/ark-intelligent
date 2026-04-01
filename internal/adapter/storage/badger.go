package storage

import (
	"context"
	"fmt"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("storage")

// DB wraps a BadgerDB instance with lifecycle management.
type DB struct {
	db     *badger.DB
	stopGC chan struct{}
}

// Open creates or opens a BadgerDB at the given path.
// It starts a background goroutine for periodic value log GC.
func Open(path string) (*DB, error) {
	opts := badger.DefaultOptions(path).
		WithLoggingLevel(badger.WARNING).
		WithNumVersionsToKeep(1).
		WithCompactL0OnClose(true).
		WithValueLogFileSize(64 << 20).
		WithNumMemtables(2).
		WithNumLevelZeroTables(2).
		WithNumLevelZeroTablesStall(4).
		WithBlockSize(4096).
		WithBloomFalsePositive(0.01)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("badger open %s: %w", path, err)
	}

	d := &DB{
		db:     db,
		stopGC: make(chan struct{}),
	}
	go d.runGC()

	log.Info().Str("path", path).Msg("BadgerDB opened")
	return d, nil
}

// Close stops the GC goroutine and closes the database.
func (d *DB) Close() error {
	close(d.stopGC)
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("badger close: %w", err)
	}
	log.Info().Msg("BadgerDB closed")
	return nil
}

// Badger returns the underlying badger.DB for direct access.
func (d *DB) Badger() *badger.DB {
	return d.db
}

// runGC performs periodic value log garbage collection.
func (d *DB) runGC() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopGC:
			return
		case <-ticker.C:
			d.gc()
		}
	}
}

// gc runs one cycle of value log GC, repeating until no more gain.
func (d *DB) gc() {
	for {
		err := d.db.RunValueLogGC(0.5)
		if err != nil {
			return
		}
	}
}

// DropAll removes all data from the database.
// It respects the provided context for cancellation/timeout to prevent
// indefinite blocking when concurrent readers hold transactions.
func (d *DB) DropAll(ctx context.Context) error {
	done := make(chan error, 1)
	go func() { done <- d.db.DropAll() }()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("badger drop all: %w", err)
		}
		log.Info().Msg("all data dropped")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("badger drop all: %w", ctx.Err())
	}
}

// Size returns the LSM and value log sizes in bytes.
func (d *DB) Size() (lsm, vlog int64) {
	return d.db.Size()
}

// --- Key Schema ---
//
// All keys follow a hierarchical prefix scheme for efficient range scans:
//
// Events:
//   evt:{YYYYMMDD}:{eventID}                    -> JSON(FFEvent)
//   evthist:{currency}:{eventName}:{YYYYMMDD}   -> JSON(FFEventDetail)
//   evtrev:{currency}:{YYYYMMDD}:{eventID}      -> JSON(EventRevision)
//
// COT:
//   cot:{contractCode}:{YYYYMMDD}               -> JSON(COTRecord)
//   cotanl:{contractCode}:{YYYYMMDD}            -> JSON(COTAnalysis)
//
// User Preferences:
//   prefs:{userID}                              -> JSON(UserPrefs)
//
// Key design principles:
// 1. Date in YYYYMMDD for lexicographic time ordering
// 2. Prefix grouping for efficient range scans
// 3. Colon separator for readability and parsing
