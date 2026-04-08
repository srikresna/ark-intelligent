package backtest

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/adapter/storage"
	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Mock COT history provider (implements COTHistoryProvider interface)
// ---------------------------------------------------------------------------

type mockCOTRepo struct {
	records  map[string][]domain.COTRecord   // contractCode -> records (newest-first)
	analyses map[string][]domain.COTAnalysis // contractCode -> analyses (newest-first)
}

func (m *mockCOTRepo) SaveRecords(_ context.Context, _ []domain.COTRecord) error { return nil }

func (m *mockCOTRepo) GetLatest(_ context.Context, contractCode string) (*domain.COTRecord, error) {
	if recs, ok := m.records[contractCode]; ok && len(recs) > 0 {
		return &recs[0], nil
	}
	return nil, nil
}

// GetHistory returns records in newest-first order (same as real COTRepo).
func (m *mockCOTRepo) GetHistory(_ context.Context, contractCode string, _ int) ([]domain.COTRecord, error) {
	recs := m.records[contractCode]
	// Return a copy in newest-first order.
	out := make([]domain.COTRecord, len(recs))
	copy(out, recs)
	return out, nil
}

func (m *mockCOTRepo) SaveAnalyses(_ context.Context, _ []domain.COTAnalysis) error { return nil }

func (m *mockCOTRepo) GetLatestAnalysis(_ context.Context, contractCode string) (*domain.COTAnalysis, error) {
	if a, ok := m.analyses[contractCode]; ok && len(a) > 0 {
		return &a[0], nil
	}
	return nil, nil
}

func (m *mockCOTRepo) GetAllLatestAnalyses(_ context.Context) ([]domain.COTAnalysis, error) {
	return nil, nil
}

func (m *mockCOTRepo) GetLatestReportDate(_ context.Context) (time.Time, error) {
	return time.Time{}, nil
}

// GetAnalysisHistory returns analyses in newest-first order (same as real COTRepo).
func (m *mockCOTRepo) GetAnalysisHistory(_ context.Context, contractCode string, _ int) ([]domain.COTAnalysis, error) {
	a := m.analyses[contractCode]
	out := make([]domain.COTAnalysis, len(a))
	copy(out, a)
	return out, nil
}

// ---------------------------------------------------------------------------
// Helper: create an in-memory BadgerDB wrapped in storage.DB
// ---------------------------------------------------------------------------

// openInMemoryDB creates a BadgerDB in-memory instance wrapped in the storage.DB type.
// We use a temp directory with InMemory option to avoid file I/O.
func openInMemoryDB(t *testing.T) *storage.DB {
	t.Helper()

	opts := badger.DefaultOptions("").
		WithInMemory(true).
		WithLoggingLevel(badger.WARNING)

	bdb, err := badger.Open(opts)
	if err != nil {
		t.Fatalf("open in-memory badger: %v", err)
	}

	// We need a storage.DB, but the Open function requires a path.
	// Use a shim: create via exported DB structure. Since storage.DB wraps
	// a *badger.DB, we'll create one using the temp dir approach to satisfy
	// the NewPriceRepo / NewSignalRepo constructors.
	tmpDir := t.TempDir()
	db, err := storage.Open(tmpDir)
	if err != nil {
		bdb.Close()
		t.Fatalf("open storage.DB: %v", err)
	}
	// We actually have two DBs open now. Close the bare one.
	bdb.Close()

	t.Cleanup(func() { db.Close() })
	return db
}

// ---------------------------------------------------------------------------
// Helper: generate test data
// ---------------------------------------------------------------------------

const testContract = "099741"
const testCurrency = "EUR"

// baseDate returns a Tuesday (COT report day) N weeks ago.
func baseDate(weeksAgo int) time.Time {
	now := time.Now().UTC().Truncate(24 * time.Hour)
	// Find last Tuesday
	for now.Weekday() != time.Tuesday {
		now = now.AddDate(0, 0, -1)
	}
	return now.AddDate(0, 0, -7*weeksAgo)
}

// generatePriceRecords creates 52 weeks of price data for the test contract.
// Prices oscillate around 1.08 with small weekly changes.
// Important: we need prices that extend to cover evaluation horizons (+28 days)
// of the oldest signals, so we generate extra weeks on the recent end.
func generatePriceRecords(weeks int) []domain.PriceRecord {
	records := make([]domain.PriceRecord, weeks)
	for i := 0; i < weeks; i++ {
		weeksAgo := weeks - 1 - i // 0 = oldest, weeks-1 = most recent
		date := baseDate(weeksAgo)
		// Oscillate around 1.08
		close := 1.08 + 0.005*math.Sin(float64(i)*0.3)
		records[i] = domain.PriceRecord{
			ContractCode: testContract,
			Symbol:       "EUR/USD",
			Date:         date,
			Open:         close - 0.002,
			High:         close + 0.005,
			Low:          close - 0.005,
			Close:        close,
			Source:       "test",
		}
	}
	return records
}

// generateCOTRecords creates 52 weeks of COT records with enough variation
// to trigger signals (extreme positioning, divergence, smart money, etc.).
func generateCOTRecords(weeks int) []domain.COTRecord {
	records := make([]domain.COTRecord, weeks)
	for i := 0; i < weeks; i++ {
		weeksAgo := weeks - 1 - i
		date := baseDate(weeksAgo)

		// Create positioning that triggers SMART_MONEY and EXTREME signals.
		// Alternate between extreme bull and bear every ~13 weeks.
		cycle := float64(i) / 13.0
		levFundLong := 120000 + 30000*math.Sin(cycle*math.Pi)
		levFundShort := 80000 - 20000*math.Sin(cycle*math.Pi)
		dealerLong := 90000 - 25000*math.Sin(cycle*math.Pi)   // Inverse of lev fund
		dealerShort := 110000 + 25000*math.Sin(cycle*math.Pi) // Inverse of lev fund

		records[i] = domain.COTRecord{
			ContractCode:  testContract,
			ContractName:  "Euro FX",
			ReportDate:    date,
			OpenInterest:  600000 + 10000*math.Sin(float64(i)*0.2),
			DealerLong:    dealerLong,
			DealerShort:   dealerShort,
			LevFundLong:   levFundLong,
			LevFundShort:  levFundShort,
			AssetMgrLong:  50000,
			AssetMgrShort: 45000,
			SmallLong:     30000,
			SmallShort:    25000,
			OtherLong:     20000,
			OtherShort:    18000,
			Top4Long:      45,
			Top4Short:     40,
			Top8Long:      65,
			Top8Short:     60,
			// WoW changes to trigger smart money signals
			DealerLongChg:   -5000 * math.Sin(cycle*math.Pi),
			DealerShortChg:  5000 * math.Sin(cycle*math.Pi),
			LevFundLongChg:  8000 * math.Sin(cycle*math.Pi),
			LevFundShortChg: -3000 * math.Sin(cycle*math.Pi),
		}
	}
	return records
}

// generateCOTAnalyses creates analyses that will trigger signal detection.
// We make some with extreme COTIndex and large CommNetChange to guarantee
// SMART_MONEY and EXTREME_POSITIONING signals fire.
func generateCOTAnalyses(weeks int) []domain.COTAnalysis {
	analyses := make([]domain.COTAnalysis, weeks)
	contract := domain.COTContract{
		Code:       testContract,
		Name:       "Euro FX",
		Symbol:     "6E",
		Currency:   testCurrency,
		ReportType: "TFF",
	}

	for i := 0; i < weeks; i++ {
		weeksAgo := weeks - 1 - i
		date := baseDate(weeksAgo)

		cycle := float64(i) / 13.0
		cotIndex := 50 + 45*math.Sin(cycle*math.Pi)     // Ranges 5..95
		cotIndexComm := 50 - 45*math.Sin(cycle*math.Pi) // Inverse of spec

		analyses[i] = domain.COTAnalysis{
			Contract:   contract,
			ReportDate: date,

			// Core positioning
			NetPosition:    40000 * math.Sin(cycle*math.Pi),
			NetChange:      8000 * math.Sin(cycle*math.Pi),
			LevFundNet:     40000 * math.Sin(cycle*math.Pi),
			CommercialNet:  -35000 * math.Sin(cycle*math.Pi),
			SmallSpecNet:   5000 * math.Sin(cycle*math.Pi),
			NetSmallSpec:   5000 * math.Sin(cycle*math.Pi),
			NetCommercial:  -35000 * math.Sin(cycle*math.Pi),
			LongShortRatio: 1.5 + 0.5*math.Sin(cycle*math.Pi),
			PctOfOI:        10 + 5*math.Sin(cycle*math.Pi),
			CommPctOfOI:    15 - 5*math.Sin(cycle*math.Pi),
			CommLSRatio:    0.8 - 0.3*math.Sin(cycle*math.Pi),
			CommNetChange:  -12000 * math.Sin(cycle*math.Pi), // Large enough (>5000)

			// COT Index - oscillate to extremes
			COTIndex:     cotIndex,
			COTIndexComm: cotIndexComm,

			// Signals and extremes
			IsExtremeBull:    cotIndex > 90,
			IsExtremeBear:    cotIndex < 10,
			CommExtremeBull:  cotIndexComm < 20,
			CommExtremeBear:  cotIndexComm > 80,
			CommercialSignal: "SIGNIFICANT_MOVE",
			SpeculatorSignal: "BUILDING",
			SmallSpecSignal:  "CROWDED",

			// WillcoIndex for Z-score confirmation
			WillcoIndex: 50 + 35*math.Sin(cycle*math.Pi),

			// Divergence
			DivergenceFlag:      i%5 == 0, // Every 5th week
			SmartDumbDivergence: i%5 == 0,

			// OI analysis
			OpenInterestChg:   5000,
			OIPctChange:       0.8,
			OITrend:           "RISING",
			Top4Concentration: 42 + 15*math.Sin(cycle*math.Pi), // Sometimes > 50
			Top8Concentration: 60 + 10*math.Sin(cycle*math.Pi),
			SpreadPctOfOI:     8,

			// Momentum
			SpecMomentum4W:   10000 * math.Sin(cycle*math.Pi),
			SpecMomentum8W:   8000 * math.Sin(cycle*math.Pi),
			CommMomentum4W:   -8000 * math.Sin(cycle*math.Pi),
			MomentumDir:      domain.MomentumBuilding,
			ConsecutiveWeeks: 3,

			// Advanced
			ShortTermBias:  "BULLISH",
			CrowdingIndex:  40 + 40*math.Sin(cycle*math.Pi), // Sometimes > 70
			SentimentScore: 60,
			SignalStrength: domain.SignalStrong,

			// Institutional
			AssetMgrZScore: 1.5 * math.Sin(cycle*math.Pi),

			// Trader concentration
			TotalTraders:        100,
			TraderConcentration: "NORMAL",
			ThinMarketAlert:     false,
		}
	}
	return analyses
}

// toNewestFirst reverses a slice (assumes oldest-first input).
func toNewestFirstRecords(recs []domain.COTRecord) []domain.COTRecord {
	out := make([]domain.COTRecord, len(recs))
	for i, r := range recs {
		out[len(recs)-1-i] = r
	}
	return out
}

func toNewestFirstAnalyses(a []domain.COTAnalysis) []domain.COTAnalysis {
	out := make([]domain.COTAnalysis, len(a))
	for i, r := range a {
		out[len(a)-1-i] = r
	}
	return out
}

// ===========================================================================
// INTEGRATION TEST
// ===========================================================================

func TestIntegrationBacktestPipeline(t *testing.T) {
	ctx := context.Background()

	// --- Step 1: Create in-memory BadgerDB ---
	db := openInMemoryDB(t)
	t.Log("STEP 1: BadgerDB opened (temp dir)")

	// --- Step 2: Create repos ---
	priceRepo := storage.NewPriceRepo(db)
	signalRepo := storage.NewSignalRepo(db)
	t.Log("STEP 2: PriceRepo and SignalRepo created")

	// --- Step 3: Seed price data ---
	// Generate 60 weeks to cover evaluation horizons (+4 weeks beyond COT data).
	priceRecords := generatePriceRecords(60)
	err := priceRepo.SavePrices(ctx, priceRecords)
	if err != nil {
		t.Fatalf("SavePrices failed: %v", err)
	}
	t.Logf("STEP 3: Seeded %d price records", len(priceRecords))
	t.Logf("  Price date range: %s to %s",
		priceRecords[0].Date.Format("2006-01-02"),
		priceRecords[len(priceRecords)-1].Date.Format("2006-01-02"))
	t.Logf("  Sample prices: first=%.5f, mid=%.5f, last=%.5f",
		priceRecords[0].Close, priceRecords[30].Close, priceRecords[len(priceRecords)-1].Close)

	// --- Step 3b: Verify PriceRepo.GetPriceAt works ---
	t.Log("STEP 3b: Testing PriceRepo.GetPriceAt()")
	testDate := priceRecords[25].Date
	foundPrice, err := priceRepo.GetPriceAt(ctx, testContract, testDate)
	if err != nil {
		t.Fatalf("GetPriceAt failed: %v", err)
	}
	if foundPrice == nil {
		t.Fatal("GetPriceAt returned nil for a date with data")
	}
	t.Logf("  GetPriceAt(%s) -> Close=%.5f (expected ~%.5f)",
		testDate.Format("2006-01-02"), foundPrice.Close, priceRecords[25].Close)

	// Test with a date +3 days offset (should still find within 7 day window)
	offsetDate := priceRecords[20].Date.AddDate(0, 0, 3)
	foundPrice2, err := priceRepo.GetPriceAt(ctx, testContract, offsetDate)
	if err != nil {
		t.Fatalf("GetPriceAt (offset) failed: %v", err)
	}
	if foundPrice2 == nil {
		t.Fatal("GetPriceAt returned nil for date +3 days from a record")
	}
	t.Logf("  GetPriceAt(%s, +3d offset) -> Close=%.5f, Date=%s",
		offsetDate.Format("2006-01-02"), foundPrice2.Close, foundPrice2.Date.Format("2006-01-02"))

	// --- Step 3c: Verify PriceRepo.GetHistory works ---
	t.Log("STEP 3c: Testing PriceRepo.GetHistory()")
	history, err := priceRepo.GetHistory(ctx, testContract, 52)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	t.Logf("  GetHistory(52 weeks) returned %d records", len(history))
	if len(history) == 0 {
		t.Fatal("GetHistory returned 0 records")
	}
	// Should be newest-first
	if len(history) > 1 && history[0].Date.Before(history[1].Date) {
		t.Error("GetHistory is NOT in newest-first order")
	} else {
		t.Log("  Confirmed newest-first ordering")
	}

	// --- Step 4: Create COT mock repo ---
	cotRecords := generateCOTRecords(52)
	cotAnalyses := generateCOTAnalyses(52)

	t.Logf("STEP 4: Generated %d COT records and %d analyses", len(cotRecords), len(cotAnalyses))
	t.Logf("  COT date range: %s to %s",
		cotRecords[0].ReportDate.Format("2006-01-02"),
		cotRecords[len(cotRecords)-1].ReportDate.Format("2006-01-02"))

	// Mock returns newest-first (same convention as real COTRepo).
	mockCOT := &mockCOTRepo{
		records: map[string][]domain.COTRecord{
			testContract: toNewestFirstRecords(cotRecords),
		},
		analyses: map[string][]domain.COTAnalysis{
			testContract: toNewestFirstAnalyses(cotAnalyses),
		},
	}

	// Verify mock returns data
	mockHistory, _ := mockCOT.GetHistory(ctx, testContract, 52)
	t.Logf("  Mock GetHistory returned %d records (newest-first)", len(mockHistory))
	mockAnalyses, _ := mockCOT.GetAnalysisHistory(ctx, testContract, 52)
	t.Logf("  Mock GetAnalysisHistory returned %d analyses (newest-first)", len(mockAnalyses))

	// --- Step 5: Run Bootstrapper ---
	t.Log("STEP 5: Running Bootstrapper")
	bootstrapper := NewBootstrapper(mockCOT, priceRepo, signalRepo, signalRepo)
	created, err := bootstrapper.Run(ctx)
	if err != nil {
		t.Fatalf("Bootstrapper.Run failed: %v", err)
	}
	t.Logf("  Bootstrapper created %d signals", created)

	// Check what signals exist
	allSignals, err := signalRepo.GetAllSignals(ctx)
	if err != nil {
		t.Fatalf("GetAllSignals failed: %v", err)
	}
	t.Logf("  Total signals in DB: %d", len(allSignals))

	if len(allSignals) == 0 {
		t.Fatal("FAIL: Bootstrapper created 0 signals. This is the reported bug.")
	}

	// --- Step 5b: Analyze bootstrapped signals ---
	t.Log("STEP 5b: Analyzing bootstrapped signals")
	zeroEntryCount := 0
	nonZeroEntryCount := 0
	signalTypes := map[string]int{}
	var sampleSignals []domain.PersistedSignal

	for i, sig := range allSignals {
		signalTypes[sig.SignalType]++
		if sig.EntryPrice == 0 {
			zeroEntryCount++
		} else {
			nonZeroEntryCount++
		}
		if i < 5 {
			sampleSignals = append(sampleSignals, sig)
		}
	}

	t.Logf("  Signals with EntryPrice > 0: %d", nonZeroEntryCount)
	t.Logf("  Signals with EntryPrice == 0: %d (these would be purged)", zeroEntryCount)
	t.Logf("  Signal types: %v", signalTypes)

	for i, sig := range sampleSignals {
		t.Logf("  Sample signal %d: type=%s, dir=%s, entry=%.5f, date=%s, contract=%s",
			i, sig.SignalType, sig.Direction, sig.EntryPrice,
			sig.ReportDate.Format("2006-01-02"), sig.ContractCode)
	}

	// --- Step 5c: Assert entry prices > 0 ---
	if zeroEntryCount > 0 {
		t.Errorf("BUG DETECTED: %d signals have EntryPrice == 0 (should all be > 0 after bootstrap fix)", zeroEntryCount)
	} else {
		t.Log("  PASS: All signals have EntryPrice > 0")
	}

	// --- Step 6: Test PurgeInvalidSignals ---
	t.Log("STEP 6: Testing PurgeInvalidSignals")
	purged, err := signalRepo.PurgeInvalidSignals(ctx)
	if err != nil {
		t.Fatalf("PurgeInvalidSignals failed: %v", err)
	}
	t.Logf("  Purged %d signals with EntryPrice==0", purged)

	afterPurge, _ := signalRepo.GetAllSignals(ctx)
	t.Logf("  Signals remaining after purge: %d", len(afterPurge))

	// --- Step 7: Test GetPendingSignals ---
	t.Log("STEP 7: Testing GetPendingSignals")
	pending, err := signalRepo.GetPendingSignals(ctx)
	if err != nil {
		t.Fatalf("GetPendingSignals failed: %v", err)
	}
	t.Logf("  Pending signals (need evaluation): %d out of %d total", len(pending), len(afterPurge))

	if len(pending) == 0 && len(afterPurge) > 0 {
		t.Log("  WARNING: 0 pending signals. Checking why...")
		now := time.Now()
		for i, sig := range afterPurge {
			if i >= 5 {
				break
			}
			age := now.Sub(sig.ReportDate)
			needs := sig.NeedsEvaluation(now)
			t.Logf("    Signal %d: entry=%.5f, age=%s, needs_eval=%v, outcome1w=%q, date=%s",
				i, sig.EntryPrice, age.Round(time.Hour), needs, sig.Outcome1W, sig.ReportDate.Format("2006-01-02"))
		}
		if len(afterPurge) > 0 && afterPurge[0].EntryPrice == 0 {
			t.Error("  BUG: Signals have EntryPrice==0, so NeedsEvaluation returns false!")
		}
	}

	// --- Step 8: Run Evaluator ---
	t.Log("STEP 8: Running Evaluator")
	evaluator := NewEvaluator(signalRepo, priceRepo)
	evaluated, err := evaluator.EvaluatePending(ctx)
	if err != nil {
		t.Fatalf("EvaluatePending failed: %v", err)
	}
	t.Logf("  Evaluator evaluated %d signals", evaluated)

	// --- Step 8b: Detailed evaluation diagnostics ---
	if evaluated == 0 && len(pending) > 0 {
		t.Log("DIAGNOSTIC: 0 signals evaluated despite pending signals. Investigating...")

		for i, sig := range pending {
			if i >= 5 {
				break
			}
			now := time.Now()
			age := now.Sub(sig.ReportDate)

			// Check if price exists at +1W
			target1W := sig.ReportDate.AddDate(0, 0, 7)
			price1W, err := priceRepo.GetPriceAt(ctx, sig.ContractCode, target1W)
			priceInfo := "nil"
			if err != nil {
				priceInfo = fmt.Sprintf("ERROR: %v", err)
			} else if price1W != nil {
				priceInfo = fmt.Sprintf("%.5f (date=%s)", price1W.Close, price1W.Date.Format("2006-01-02"))
			}

			t.Logf("  Signal %d: contract=%s, report=%s, age=%s, entry=%.5f, 1W_target=%s, 1W_price=%s",
				i, sig.ContractCode, sig.ReportDate.Format("2006-01-02"),
				age.Round(time.Hour), sig.EntryPrice,
				target1W.Format("2006-01-02"), priceInfo)

			// Check if the issue is that NeedsEvaluation was false
			t.Logf("    NeedsEvaluation(now)=%v, Outcome1W=%q, Outcome2W=%q, Outcome4W=%q",
				sig.NeedsEvaluation(now), sig.Outcome1W, sig.Outcome2W, sig.Outcome4W)
		}
	}

	// --- Step 8c: Verify evaluated signals ---
	allAfterEval, err := signalRepo.GetAllSignals(ctx)
	if err != nil {
		t.Fatalf("GetAllSignals after eval failed: %v", err)
	}

	evaluatedCount := 0
	outcomeWin := 0
	outcomeLoss := 0
	for _, sig := range allAfterEval {
		if sig.Outcome1W != "" && sig.Outcome1W != domain.OutcomePending {
			evaluatedCount++
			if sig.Outcome1W == domain.OutcomeWin {
				outcomeWin++
			} else {
				outcomeLoss++
			}
		}
	}

	t.Logf("STEP 8c: Post-evaluation results:")
	t.Logf("  Total signals: %d", len(allAfterEval))
	t.Logf("  With 1W outcome: %d (WIN=%d, LOSS=%d)", evaluatedCount, outcomeWin, outcomeLoss)

	// Show some evaluated samples
	sampleCount := 0
	for _, sig := range allAfterEval {
		if sig.Outcome1W != "" && sig.Outcome1W != domain.OutcomePending && sampleCount < 3 {
			t.Logf("  Evaluated sample: type=%s, dir=%s, entry=%.5f, 1W_price=%.5f, return=%.4f%%, outcome=%s",
				sig.SignalType, sig.Direction, sig.EntryPrice, sig.Price1W, sig.Return1W, sig.Outcome1W)
			sampleCount++
		}
	}

	// --- Step 9: Final assertions ---
	t.Log("STEP 9: Final assertions")

	if created == 0 {
		t.Error("FAIL: Bootstrapper created 0 signals")
	} else {
		t.Logf("  PASS: Bootstrapper created %d signals", created)
	}

	if nonZeroEntryCount == 0 {
		t.Error("FAIL: All signals have EntryPrice == 0")
	} else {
		t.Logf("  PASS: %d signals have EntryPrice > 0", nonZeroEntryCount)
	}

	if evaluated == 0 {
		t.Errorf("FAIL: 0 signals evaluated (pending=%d, total=%d). This is the reported bug!", len(pending), len(allAfterEval))

		// Extra diagnostics: check for any prices in the DB for this contract
		allPrices, _ := priceRepo.GetHistory(ctx, testContract, 104)
		t.Logf("  DIAGNOSTIC: Price records in DB for %s: %d", testContract, len(allPrices))
		if len(allPrices) > 0 {
			t.Logf("    Latest price: %s (%.5f)", allPrices[0].Date.Format("2006-01-02"), allPrices[0].Close)
			t.Logf("    Oldest price: %s (%.5f)", allPrices[len(allPrices)-1].Date.Format("2006-01-02"), allPrices[len(allPrices)-1].Close)
		}

		// Check if the issue is that the bootstrapper only works for
		// contracts in DefaultPriceSymbolMappings
		foundMapping := false
		for _, m := range domain.DefaultPriceSymbolMappings {
			if m.ContractCode == testContract {
				foundMapping = true
				t.Logf("  Contract %s IS in DefaultPriceSymbolMappings (currency=%s)", testContract, m.Currency)
				break
			}
		}
		if !foundMapping {
			t.Logf("  BUG: Contract %s NOT in DefaultPriceSymbolMappings!", testContract)
		}
	} else {
		t.Logf("  PASS: %d signals evaluated", evaluated)
	}

	if evaluatedCount == 0 {
		t.Error("FAIL: After evaluation, no signals have Outcome1W set")
	} else {
		t.Logf("  PASS: %d signals have Outcome1W set", evaluatedCount)
	}

	// Summary
	t.Log("")
	t.Log("=== SUMMARY ===")
	t.Logf("  Price records seeded:    %d", len(priceRecords))
	t.Logf("  COT records seeded:      %d", len(cotRecords))
	t.Logf("  COT analyses seeded:     %d", len(cotAnalyses))
	t.Logf("  Signals bootstrapped:    %d", created)
	t.Logf("  Signals with entry > 0:  %d", nonZeroEntryCount)
	t.Logf("  Signals with entry == 0: %d", zeroEntryCount)
	t.Logf("  Purged (entry==0):       %d", purged)
	t.Logf("  Pending for evaluation:  %d", len(pending))
	t.Logf("  Actually evaluated:      %d", evaluated)
	t.Logf("  With 1W outcome:         %d", evaluatedCount)
}

// ===========================================================================
// UNIT-LEVEL TESTS FOR INDIVIDUAL REPOS
// ===========================================================================

func TestPriceRepoSaveAndGetPriceAt(t *testing.T) {
	ctx := context.Background()
	db := openInMemoryDB(t)
	repo := storage.NewPriceRepo(db)

	// Save a single record
	date := time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC) // a Tuesday
	records := []domain.PriceRecord{
		{
			ContractCode: testContract,
			Symbol:       "EUR/USD",
			Date:         date,
			Open:         1.0800,
			High:         1.0850,
			Low:          1.0750,
			Close:        1.0825,
			Source:       "test",
		},
	}

	err := repo.SavePrices(ctx, records)
	if err != nil {
		t.Fatalf("SavePrices: %v", err)
	}

	// Exact date lookup
	found, err := repo.GetPriceAt(ctx, testContract, date)
	if err != nil {
		t.Fatalf("GetPriceAt exact: %v", err)
	}
	if found == nil {
		t.Fatal("GetPriceAt exact returned nil")
	}
	if found.Close != 1.0825 {
		t.Errorf("Close = %f, want 1.0825", found.Close)
	}
	t.Logf("Exact match: date=%s, close=%.4f", found.Date.Format("2006-01-02"), found.Close)

	// +3 day offset (within 7 day window)
	found2, err := repo.GetPriceAt(ctx, testContract, date.AddDate(0, 0, 3))
	if err != nil {
		t.Fatalf("GetPriceAt +3d: %v", err)
	}
	if found2 == nil {
		t.Fatal("GetPriceAt +3d returned nil (should find within 7-day window)")
	}
	t.Logf("+3d match: query=%s, found=%s, close=%.4f",
		date.AddDate(0, 0, 3).Format("2006-01-02"),
		found2.Date.Format("2006-01-02"), found2.Close)

	// -3 day offset (within 7 day window backward)
	found3, err := repo.GetPriceAt(ctx, testContract, date.AddDate(0, 0, -3))
	if err != nil {
		t.Fatalf("GetPriceAt -3d: %v", err)
	}
	if found3 == nil {
		t.Fatal("GetPriceAt -3d returned nil (should find within 7-day backward window)")
	}
	t.Logf("-3d match: query=%s, found=%s, close=%.4f",
		date.AddDate(0, 0, -3).Format("2006-01-02"),
		found3.Date.Format("2006-01-02"), found3.Close)

	// +10 day offset (outside 7-day window) should return nil
	found4, err := repo.GetPriceAt(ctx, testContract, date.AddDate(0, 0, 10))
	if err != nil {
		t.Fatalf("GetPriceAt +10d: %v", err)
	}
	if found4 != nil {
		t.Errorf("GetPriceAt +10d should return nil (outside window), got date=%s",
			found4.Date.Format("2006-01-02"))
	} else {
		t.Log("+10d: correctly returned nil (outside 7-day window)")
	}
}

func TestSignalRepoPurgeAndPending(t *testing.T) {
	ctx := context.Background()
	db := openInMemoryDB(t)
	repo := storage.NewSignalRepo(db)

	now := time.Now().UTC()

	// Create signals: some with entry price 0, some with valid prices
	signals := []domain.PersistedSignal{
		{
			ContractCode: testContract,
			Currency:     testCurrency,
			SignalType:   "SMART_MONEY",
			Direction:    "BULLISH",
			Strength:     4,
			Confidence:   75,
			ReportDate:   now.AddDate(0, 0, -14), // 2 weeks ago
			DetectedAt:   now.AddDate(0, 0, -14),
			EntryPrice:   1.0825, // Valid
		},
		{
			ContractCode: testContract,
			Currency:     testCurrency,
			SignalType:   "EXTREME_POSITIONING",
			Direction:    "BEARISH",
			Strength:     3,
			Confidence:   60,
			ReportDate:   now.AddDate(0, 0, -21), // 3 weeks ago
			DetectedAt:   now.AddDate(0, 0, -21),
			EntryPrice:   0, // Invalid — should be purged
		},
		{
			ContractCode: testContract,
			Currency:     testCurrency,
			SignalType:   "DIVERGENCE",
			Direction:    "BULLISH",
			Strength:     3,
			Confidence:   50,
			ReportDate:   now.AddDate(0, 0, -3), // 3 days ago — too recent for eval
			DetectedAt:   now.AddDate(0, 0, -3),
			EntryPrice:   1.0800,
		},
	}

	err := repo.SaveSignals(ctx, signals)
	if err != nil {
		t.Fatalf("SaveSignals: %v", err)
	}

	all, _ := repo.GetAllSignals(ctx)
	t.Logf("Saved %d signals", len(all))

	// Test purge
	purged, err := repo.PurgeInvalidSignals(ctx)
	if err != nil {
		t.Fatalf("PurgeInvalidSignals: %v", err)
	}
	if purged != 1 {
		t.Errorf("Expected 1 purged, got %d", purged)
	}
	t.Logf("Purged %d signals (expected 1)", purged)

	remaining, _ := repo.GetAllSignals(ctx)
	if len(remaining) != 2 {
		t.Errorf("Expected 2 remaining, got %d", len(remaining))
	}

	// Test pending: only the 2-week old signal should be pending (3-day old is too recent)
	pending, err := repo.GetPendingSignals(ctx)
	if err != nil {
		t.Fatalf("GetPendingSignals: %v", err)
	}
	t.Logf("Pending signals: %d", len(pending))

	for _, p := range pending {
		t.Logf("  Pending: type=%s, entry=%.5f, age=%s, outcome1w=%q",
			p.SignalType, p.EntryPrice, now.Sub(p.ReportDate).Round(time.Hour), p.Outcome1W)
	}

	if len(pending) != 1 {
		t.Errorf("Expected 1 pending signal (the 2-week old one), got %d", len(pending))
	}
}

func TestPriceRepoGetHistory(t *testing.T) {
	ctx := context.Background()
	db := openInMemoryDB(t)
	repo := storage.NewPriceRepo(db)

	// Generate and save 20 weeks of prices
	records := generatePriceRecords(20)
	err := repo.SavePrices(ctx, records)
	if err != nil {
		t.Fatalf("SavePrices: %v", err)
	}

	// Fetch history
	history, err := repo.GetHistory(ctx, testContract, 20)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}

	t.Logf("GetHistory(20) returned %d records", len(history))
	if len(history) == 0 {
		t.Fatal("GetHistory returned 0 records")
	}

	// Verify ordering (newest first)
	for i := 1; i < len(history); i++ {
		if history[i].Date.After(history[i-1].Date) {
			t.Errorf("Not newest-first at index %d: %s > %s",
				i, history[i].Date.Format("2006-01-02"), history[i-1].Date.Format("2006-01-02"))
		}
	}
	t.Log("  Ordering: newest-first confirmed")

	// Verify all prices have valid Close
	for _, r := range history {
		if r.Close <= 0 {
			t.Errorf("Price record on %s has Close=%.5f (expected > 0)",
				r.Date.Format("2006-01-02"), r.Close)
		}
	}
	t.Log("  All records have Close > 0")
}
