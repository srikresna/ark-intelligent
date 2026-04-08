// Package main provides storage layer wiring for dependency injection.
// This file extracts storage initialization from main.go per TECH-012 ADR.
package main

import (
	"context"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/adapter/storage"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/finviz"
	"github.com/arkcode369/ark-intelligent/internal/service/sentiment"
)

// StorageDeps holds all storage layer dependencies initialized by InitializeStorage.
// This struct centralizes repository access and database handles for clean DI.
type StorageDeps struct {
	DB               *storage.DB
	EventRepo        *storage.EventRepo
	COTRepo          *storage.COTRepo
	PrefsRepo        *storage.PrefsRepo
	NewsRepo         *storage.NewsRepo
	CacheRepo        *storage.CacheRepo
	UserRepo         *storage.UserRepo
	PriceRepo        *storage.PriceRepo
	SignalRepo       *storage.SignalRepo
	ImpactRepo       *storage.ImpactRepo
	DailyPriceRepo   *storage.DailyPriceRepo
	IntradayRepo     *storage.IntradayRepo
	FeedbackRepo     *storage.FeedbackRepo
	FREDRepo         *storage.FREDRepo
	MemoryRepo       *storage.MemoryRepo
	ConversationRepo *storage.ConversationRepo
}

// StorageConfig holds configuration parameters for storage initialization.
type StorageConfig struct {
	DataDir          string
	ChatHistoryLimit int
	ChatHistoryTTL   time.Duration
	MemoryTTL        time.Duration
}

// DefaultStorageConfig returns sensible defaults for storage configuration.
func DefaultStorageConfig(dataDir string) StorageConfig {
	return StorageConfig{
		DataDir:          dataDir,
		ChatHistoryLimit: 50,                  // Default conversation history limit
		ChatHistoryTTL:   24 * time.Hour * 30, // 30 days
		MemoryTTL:        24 * time.Hour * 30, // 30 days
	}
}

// InitializeStorage sets up the entire storage layer: database, all repositories,
// and cache persistence. Returns StorageDeps with initialized handles.
// This is Step 2 of TECH-012 DI refactor — extracted from main.go section 3.
func InitializeStorage(cfg StorageConfig) (*StorageDeps, error) {
	// Open database
	db, err := storage.Open(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	// Initialize all repositories
	deps := &StorageDeps{
		DB:               db,
		EventRepo:        storage.NewEventRepo(db),
		COTRepo:          storage.NewCOTRepo(db),
		PrefsRepo:        storage.NewPrefsRepo(db),
		NewsRepo:         storage.NewNewsRepo(db),
		CacheRepo:        storage.NewCacheRepo(db),
		UserRepo:         storage.NewUserRepo(db),
		PriceRepo:        storage.NewPriceRepo(db),
		SignalRepo:       storage.NewSignalRepo(db),
		ImpactRepo:       storage.NewImpactRepo(db),
		DailyPriceRepo:   storage.NewDailyPriceRepo(db),
		IntradayRepo:     storage.NewIntradayRepo(db),
		FeedbackRepo:     storage.NewFeedbackRepo(db),
		FREDRepo:         storage.NewFREDRepo(db),
		MemoryRepo:       storage.NewMemoryRepo(db, cfg.MemoryTTL),
		ConversationRepo: storage.NewConversationRepo(db, cfg.ChatHistoryLimit, cfg.ChatHistoryTTL),
	}

	// Initialize sentiment cache persistence (BadgerDB-backed)
	// Saves Firecrawl API quota by avoiding re-fetches on every restart.
	sentiment.InitSentimentCache(db.Badger())

	// Initialize FinViz cache persistence
	finviz.InitCache(db.Badger())

	return deps, nil
}

// CloseStorage gracefully closes the database connection.
// Should be called during shutdown sequence.
func CloseStorage(db *storage.DB) error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// SetupFREDPersistence configures the FRED persistence hook.
// This bridges FRED service snapshots to the repository layer.
func SetupFREDPersistence(deps *StorageDeps) {
	fredPersistence := fred.NewPersistenceService(&fredPersistAdapter{repo: deps.FREDRepo})
	fred.SetPostFetchHook(func(ctx context.Context, data *fred.MacroData) {
		if err := fredPersistence.PersistSnapshot(ctx, data); err != nil {
			// Log via global logger since we don't have context logger here
			log.Warn().Err(err).Msg("FRED snapshot persistence failed (non-fatal)")
		}
	})
}

// LogStorageSize logs the current database size in human-readable format.
func LogStorageSize(db *storage.DB) {
	if db == nil {
		return
	}
	lsm, vlog := db.Size()
	total := lsm + vlog
	if total > 1<<20 {
		log.Info().
			Float64("total_mb", float64(total)/(1<<20)).
			Float64("lsm_mb", float64(lsm)/(1<<20)).
			Float64("vlog_mb", float64(vlog)/(1<<20)).
			Msg("Storage size")
	} else {
		log.Info().
			Int64("total_kb", total>>10).
			Int64("lsm_kb", lsm>>10).
			Int64("vlog_kb", vlog>>10).
			Msg("Storage size")
	}
}

// fredPersistAdapter adapts storage.FREDRepo (ports.FREDRepository) to the
// fred.FREDPersister interface, bridging the ports ↔ fred type boundary.
type fredPersistAdapter struct {
	repo *storage.FREDRepo
}

func (a *fredPersistAdapter) SaveSnapshots(ctx context.Context, obs []fred.FREDObservation) error {
	portObs := make([]ports.FREDObservation, len(obs))
	for i, o := range obs {
		portObs[i] = ports.FREDObservation{SeriesID: o.SeriesID, Date: o.Date, Value: o.Value}
	}
	return a.repo.SaveSnapshots(ctx, portObs)
}
