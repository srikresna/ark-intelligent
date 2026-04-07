# TASK-TEST-003: Unit Tests for format_cot.go Output Formatters

## Metadata
- **Task ID**: TASK-TEST-003
- **Priority**: HIGH
- **Estimated Effort**: 4-5 hours
- **Assigned To**: Dev-A (Agent-3)
- **Status**: In Progress
- **Branch**: feat/TASK-TEST-003-format-cot-tests
- **Created**: 2026-04-07

## Objective
Implement comprehensive unit tests for `internal/adapter/telegram/format_cot.go` COT (Commitment of Traders) output formatters to improve test coverage and ensure formatting correctness.

## Background
The `format_cot.go` file contains ~1394 lines of code responsible for formatting COT analysis data for Telegram display. It includes functions for:
- COT overview and detail formatting
- Currency strength rankings
- Conviction score rendering
- Signal interpretation
- Price-COT divergence/alerts
- Best pair recommendations

Currently, this file has **zero test coverage**.

## Acceptance Criteria

### Required Test Coverage (15+ tests)

1. **Test `cotIdxLabel`** - COT Index label conversion (5 test cases)
   - Extreme Long (>=80)
   - Bullish (>=60)
   - Neutral (>=40)
   - Bearish (>=20)
   - Extreme Short (<20)

2. **Test `cotLabel`** - COT label for ranking (same thresholds as above)

3. **Test `convictionMiniBar`** - Conviction bar rendering (5+ test cases)
   - Score 0, 25, 50, 75, 100
   - Direction LONG and SHORT
   - Verify bar fill calculation

4. **Test `FormatCOTOverview`** - Overview formatting (3+ test cases)
   - Empty analyses
   - Analyses with conviction data
   - Ungrouped contracts handling

5. **Test `FormatCOTDetail`** - Detail formatting (3+ test cases)
   - TFF report type formatting
   - DISAGGREGATED report type formatting
   - Alert conditions rendering

6. **Test `FormatRanking`** - Currency strength ranking (3+ test cases)
   - Basic ranking output
   - Best pairs generation
   - Empty input handling

7. **Test `buildBestPairs`** - Pair recommendations (3+ test cases)
   - Normal spread pairs
   - No meaningful spread fallback
   - Insufficient entries

8. **Test `pairDirection`** - Pair direction logic (3+ test cases)
   - Favored currency as base
   - Favored currency as quote
   - Cross pairs

9. **Test `formatPairName`** - Pair name formatting (5+ test cases)
   - USD as quote (EURUSD)
   - USD as base (USDJPY)
   - Cross pairs (EURGBP)
   - USD vs non-standard

10. **Test `commercialSignalLabel`** - Signal labels (3+ test cases)
    - TFF suffix
    - DISAGGREGATED suffix
    - Default case

11. **Test `signalConfluenceInterpretation`** - Signal interpretation (5+ test cases)
    - Strong agreement both sides
    - Normal agreement
    - Divergence for commodities
    - Divergence for forex
    - Commercial neutral cases

## Technical Notes

### File Location
- **Target**: `internal/adapter/telegram/format_cot.go` (~1394 lines)
- **Test File**: `internal/adapter/telegram/format_cot_test.go`

### Dependencies
- Uses domain types from `internal/domain`
- Uses COT service types from `internal/service/cot`
- Uses FRED service types from `internal/service/fred`
- Uses price service types from `internal/service/price`

### Key Types Needed
```go
// From internal/domain
type COTAnalysis struct {
    Contract           COTContract
    ReportDate         time.Time
    NetPosition        int
    NetChange          int
    COTIndex           float64
    COTIndexComm       float64
    PctOfOI            float64
    CommPctOfOI        float64
    LongShortRatio     float64
    SpeculatorSignal   string
    CommercialSignal   string
    SpecMomentum4W     int
    SpecMomentum8W     int
    ConsecutiveWeeks   int
    MomentumDir        string
    SentimentScore     float64
    CommercialNet      int
    OpenInterestChg    int
    OITrend            string
    OI4WTrend          string
    OI4WMomentum       float64
    SpreadPctOfOI      float64
    TotalTraders       int
    TraderConcentration string
    ShortTermBias      string
    CrowdingIndex      float64
    DivergenceFlag     bool
    SmartDumbDivergence bool
    CommExtremeBull    bool
    CommExtremeBear    bool
    AssetMgrAlert      bool
    AssetMgrZScore     float64
    ThinMarketAlert    bool
    ThinMarketDesc     string
    CategoryDivergence bool
    CategoryDivergenceDesc string
    DealerAlert        bool
    LevFundAlert       bool
    ManagedMoneyAlert  bool
    SwapDealerAlert    bool
    DealerZScore       float64
    LevFundZScore      float64
    AssetMgrZScore     float64
    ManagedMoneyZScore float64
    SwapDealerZScore   float64
    LevFundLongTraders int
    LevFundShortTraders int
    DealerShortTraders int
    AssetMgrLongTraders int
    AssetMgrShortTraders int
    MMoneyLongTraders  int
    MMoneyShortTraders int
}
```

## Implementation Plan

### Phase 1: Setup (15 min)
1. Create test file with package declaration
2. Add imports for required types
3. Create test helper functions for mock data

### Phase 2: Simple Functions (1 hour)
1. Test `cotIdxLabel`
2. Test `cotLabel`
3. Test `convictionMiniBar`
4. Test `pairDirection`
5. Test `formatPairName`

### Phase 3: Complex Formatters (2-3 hours)
1. Test `FormatCOTOverview`
2. Test `FormatCOTDetail`
3. Test `FormatRanking`
4. Test `buildBestPairs`

### Phase 4: Signal Functions (1 hour)
1. Test `commercialSignalLabel`
2. Test `signalConfluenceInterpretation`

### Phase 5: Validation (30 min)
1. Run `go build ./...`
2. Run `go test ./internal/adapter/telegram/...`
3. Verify all tests pass

## Definition of Done
- [x] Task claimed in STATUS.md
- [ ] Test file created with 15+ test functions
- [ ] All tests pass (`go test ./internal/adapter/telegram/...`)
- [ ] Build passes (`go build ./...`)
- [ ] Vet passes (`go vet ./...`)
- [ ] PR created with validation evidence

## Progress Log

### 2026-04-07
- **03:22 UTC**: Task claimed by Dev-A
- **03:22 UTC**: Feature branch `feat/TASK-TEST-003-format-cot-tests` created
- **03:22 UTC**: Task spec created
