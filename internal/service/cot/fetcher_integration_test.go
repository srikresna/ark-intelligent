//go:build integration
// +build integration

package cot

import (
	"context"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// TestSocrataFetchAndFieldIntegrity verifies that CFTC Socrata API returns
// non-zero values for all critical fields including change_in_*, traders_*, and spread positions.
// Run: go test -tags=integration -run TestSocrataFetchAndFieldIntegrity -v ./internal/service/cot/
func TestSocrataFetchAndFieldIntegrity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fetcher := NewFetcher()

	// Test TFF contract (EUR FX)
	t.Run("TFF_EUR_FX", func(t *testing.T) {
		eurContract := domain.COTContract{
			Code: "099741", Name: "Euro FX", Symbol: "6E",
			Currency: "EUR", ReportType: "TFF",
		}

		records, err := fetcher.FetchHistory(ctx, eurContract, 2)
		if err != nil {
			t.Fatalf("FetchHistory TFF failed: %v", err)
		}
		if len(records) == 0 {
			t.Fatal("No TFF records returned")
		}

		r := records[0]
		t.Logf("TFF EUR Report Date: %s", r.ReportDate.Format("2006-01-02"))

		// Core positions must be non-zero
		assertNonZero(t, "OpenInterest", r.OpenInterest)
		assertNonZero(t, "DealerLong", r.DealerLong)
		assertNonZero(t, "DealerShort", r.DealerShort)
		assertNonZero(t, "LevFundLong", r.LevFundLong)
		assertNonZero(t, "LevFundShort", r.LevFundShort)
		assertNonZero(t, "AssetMgrLong", r.AssetMgrLong)
		assertNonZero(t, "AssetMgrShort", r.AssetMgrShort)

		// HIGH PRIORITY 1: change_in_* fields must be captured from API
		t.Log("--- change_in_* fields (HIGH PRIORITY 1) ---")
		logField(t, "DealerLongChg", r.DealerLongChg)
		logField(t, "DealerShortChg", r.DealerShortChg)
		logField(t, "AssetMgrLongChg", r.AssetMgrLongChg)
		logField(t, "AssetMgrShortChg", r.AssetMgrShortChg)
		logField(t, "LevFundLongChg", r.LevFundLongChg)
		logField(t, "LevFundShortChg", r.LevFundShortChg)
		logField(t, "OIChangeAPI", r.OIChangeAPI)
		logField(t, "SmallLongChgAPI", r.SmallLongChgAPI)
		logField(t, "SmallShortChgAPI", r.SmallShortChgAPI)
		logField(t, "OtherLongChg", r.OtherLongChg)
		logField(t, "OtherShortChg", r.OtherShortChg)

		// NetChange should be computed from API change fields
		logField(t, "NetChange", r.NetChange)
		assertNonZero(t, "NetChange (from API LevFund changes)", r.NetChange)

		// At least some change fields should be non-zero (data moves every week)
		anyChange := r.DealerLongChg != 0 || r.DealerShortChg != 0 ||
			r.LevFundLongChg != 0 || r.LevFundShortChg != 0
		if !anyChange {
			t.Error("ALL change_in_* fields are zero — API mapping may be broken")
		}

		// HIGH PRIORITY 2: traders_* fields must be non-zero
		t.Log("--- traders_* fields (HIGH PRIORITY 2) ---")
		logFieldInt(t, "DealerLongTraders", r.DealerLongTraders)
		logFieldInt(t, "DealerShortTraders", r.DealerShortTraders)
		logFieldInt(t, "AssetMgrLongTraders", r.AssetMgrLongTraders)
		logFieldInt(t, "AssetMgrShortTraders", r.AssetMgrShortTraders)
		logFieldInt(t, "LevFundLongTraders", r.LevFundLongTraders)
		logFieldInt(t, "LevFundShortTraders", r.LevFundShortTraders)
		logFieldInt(t, "TotalTraders", r.TotalTraders)

		assertNonZeroInt(t, "TotalTraders (was broken by dup JSON tag)", r.TotalTraders)
		assertNonZeroInt(t, "DealerShortTraders", r.DealerShortTraders)
		assertNonZeroInt(t, "LevFundLongTraders", r.LevFundLongTraders)

		// MEDIUM: Spread positions
		t.Log("--- Spread positions (MEDIUM) ---")
		logField(t, "DealerSpread", r.DealerSpread)
		logField(t, "AssetMgrSpread", r.AssetMgrSpread)
		logField(t, "LevFundSpread", r.LevFundSpread)
		logField(t, "OtherSpread", r.OtherSpread)

		totalSpread := r.GetTotalSpread("TFF")
		t.Logf("  TotalSpread: %.0f", totalSpread)
		if totalSpread == 0 {
			t.Log("  WARN: TotalSpread is 0 — check spread field mapping")
		}

		// Concentration
		t.Log("--- Concentration ---")
		logField(t, "Top4Long", r.Top4Long)
		logField(t, "Top4Short", r.Top4Short)
	})

	// Test DISAGG contract (Gold)
	t.Run("DISAGG_Gold", func(t *testing.T) {
		goldContract := domain.COTContract{
			Code: "088691", Name: "Gold", Symbol: "GC",
			Currency: "XAU", ReportType: "DISAGGREGATED",
		}

		records, err := fetcher.FetchHistory(ctx, goldContract, 2)
		if err != nil {
			t.Fatalf("FetchHistory DISAGG failed: %v", err)
		}
		if len(records) == 0 {
			t.Fatal("No DISAGG records returned")
		}

		r := records[0]
		t.Logf("DISAGG Gold Report Date: %s", r.ReportDate.Format("2006-01-02"))

		assertNonZero(t, "ManagedMoneyLong", r.ManagedMoneyLong)
		assertNonZero(t, "ManagedMoneyShort", r.ManagedMoneyShort)
		assertNonZero(t, "ProdMercLong", r.ProdMercLong)
		assertNonZero(t, "SwapDealerLong", r.SwapDealerLong)

		// Change fields
		t.Log("--- DISAGG change_in_* fields ---")
		logField(t, "ManagedMoneyLongChg", r.ManagedMoneyLongChg)
		logField(t, "ManagedMoneyShortChg", r.ManagedMoneyShortChg)
		logField(t, "ProdMercLongChg", r.ProdMercLongChg)
		logField(t, "SwapLongChg", r.SwapLongChg)
		logField(t, "SmallLongChgAPI", r.SmallLongChgAPI)

		// Trader counts
		t.Log("--- DISAGG traders_* fields ---")
		logFieldInt(t, "MMoneyLongTraders", r.MMoneyLongTraders)
		logFieldInt(t, "MMoneyShortTraders", r.MMoneyShortTraders)
		logFieldInt(t, "ProdMercLongTraders", r.ProdMercLongTraders)
		logFieldInt(t, "TotalTradersDisag", r.TotalTradersDisag)

		assertNonZeroInt(t, "TotalTradersDisag", r.TotalTradersDisag)
		assertNonZeroInt(t, "MMoneyLongTraders", r.MMoneyLongTraders)

		// Spread positions
		t.Log("--- DISAGG Spread ---")
		logField(t, "ManagedMoneySpread", r.ManagedMoneySpread)
		logField(t, "ProdMercSpread", r.ProdMercSpread)
		logField(t, "SwapDealerSpread", r.SwapDealerSpread)

		totalSpread := r.GetTotalSpread("DISAGGREGATED")
		t.Logf("  TotalSpread DISAGG: %.0f", totalSpread)
	})

	// Test FetchLatest (all contracts)
	t.Run("FetchLatest_AllContracts", func(t *testing.T) {
		records, err := fetcher.FetchLatest(ctx, domain.DefaultCOTContracts)
		if err != nil {
			t.Fatalf("FetchLatest failed: %v", err)
		}

		t.Logf("FetchLatest returned %d records", len(records))
		if len(records) < 8 {
			t.Errorf("Expected at least 8 contracts, got %d", len(records))
		}

		for _, r := range records {
			t.Logf("  %s (%s): OI=%.0f, TotalTraders=%d, NetChange=%.0f",
				r.ContractName, r.ContractCode, r.OpenInterest, r.TotalTraders, r.NetChange)
		}
	})
}

func assertNonZero(t *testing.T, name string, val float64) {
	t.Helper()
	if val == 0 {
		t.Errorf("%s is 0 — expected non-zero value from API", name)
	}
}

func assertNonZeroInt(t *testing.T, name string, val int) {
	t.Helper()
	if val == 0 {
		t.Errorf("%s is 0 — expected non-zero value from API", name)
	}
}

func logField(t *testing.T, name string, val float64) {
	t.Helper()
	status := "✓"
	if val == 0 {
		status = "⚠ ZERO"
	}
	t.Logf("  %-25s %12.0f %s", name, val, status)
}

func logFieldInt(t *testing.T, name string, val int) {
	t.Helper()
	status := "✓"
	if val == 0 {
		status = "⚠ ZERO"
	}
	t.Logf("  %-25s %12d %s", name, val, status)
}
