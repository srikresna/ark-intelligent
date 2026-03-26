// Package fred provides integration with the FRED (Federal Reserve Economic Data) API.
// FRED is operated by the St. Louis Fed and provides free access to thousands of
// macroeconomic data series via a public REST API.
//
// Free API key available at: https://fred.stlouisfed.org/docs/api/api_key.html
// Set FRED_API_KEY environment variable. Without a key, the API still works for
// basic requests but may be rate-limited.
package fred

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("fred")

// SeriesTrend holds a time-series value with trend direction.
type SeriesTrend struct {
	Latest    float64
	Previous  float64
	Delta     float64
	Direction string // "UP", "DOWN", "FLAT"
}

// trendArrow returns a display arrow for a trend direction.
func (t SeriesTrend) Arrow() string {
	switch t.Direction {
	case "UP":
		return "↑"
	case "DOWN":
		return "↓"
	default:
		return "→"
	}
}

// computeTrend calculates trend direction given two values and a threshold.
func computeTrend(latest, previous, threshold float64) SeriesTrend {
	delta := latest - previous
	direction := "FLAT"
	if math.Abs(delta) >= threshold {
		if delta > 0 {
			direction = "UP"
		} else {
			direction = "DOWN"
		}
	}
	return SeriesTrend{Latest: latest, Previous: previous, Delta: delta, Direction: direction}
}

// MacroData holds the latest values for all tracked FRED series.
type MacroData struct {
	// Yield curve
	Yield2Y         float64     // DGS2  — 2-Year Treasury Constant Maturity Rate
	Yield5Y         float64     // DGS5  — 5-Year Treasury Constant Maturity Rate
	Yield10Y        float64     // DGS10 — 10-Year Treasury Constant Maturity Rate
	Yield30Y        float64     // DGS30 — 30-Year Treasury Constant Maturity Rate
	Yield3M         float64     // DGS3MO — 3-Month Treasury (for 3M-10Y spread)
	YieldSpread     float64     // DGS10 - DGS2 (positive = normal, negative = inverted)
	Spread3M10Y     float64     // DGS10 - DGS3MO (better recession predictor)
	Spread2Y30Y     float64     // DGS30 - DGS2 (long-end term premium)
	YieldSpreadTrend SeriesTrend // trend: is spread steepening or flattening?

	// Inflation
	Breakeven5Y  float64     // T10YIE — 10-Year Breakeven Inflation Rate
	CorePCE      float64     // PCEPILFE — Core PCE Price Index (YoY %)
	CPI          float64     // CPIAUCSL — Consumer Price Index (YoY %)
	CorePCETrend SeriesTrend // trend: inflation rising or falling?
	CPITrend     SeriesTrend

	// Financial stress & liquidity
	NFCI      float64     // NFCI — National Financial Conditions Index (negative = loose)
	TedSpread float64     // TEDRATE — TED Spread (credit risk proxy, bps)
	NFCITrend SeriesTrend

	// Short-term rates & liquidity
	SOFR float64 // SOFR — Secured Overnight Financing Rate (%)
	IORB float64 // IORB — Interest on Reserve Balances (Fed's true policy floor)

	// Labor market
	InitialClaims float64     // ICSA — Initial Jobless Claims (raw units)
	UnemployRate  float64     // UNRATE — Civilian Unemployment Rate (%)
	ClaimsTrend   SeriesTrend // trend: claims rising or falling?

	// Monetary policy
	FedFundsRate float64 // FEDFUNDS — Effective Federal Funds Rate (%)
	M2Growth     float64 // M2SL — computed YoY growth % (NOT level)
	M2GrowthTrend SeriesTrend

	// Growth
	GDPGrowth float64 // A191RL1Q225SBEA — Real GDP Growth Rate (QoQ annualized %)

	// Recession indicators
	SahmRule float64 // SAHMCURRENT — Sahm Rule Recession Indicator (>0.5 = recession)

	// Fed balance sheet
	FedBalSheet      float64     // WALCL — Fed Total Assets (billions USD)
	FedBalSheetTrend SeriesTrend // trend: QE (expanding) or QT (contracting)

	// USD strength
	DXY float64 // DTWEXBGS — Nominal Broad U.S. Dollar Index

	// VIX — real-time risk sentiment
	VIX      float64     // VIXCLS — CBOE Volatility Index
	VIXTrend SeriesTrend // trend: VIX rising or falling?

	// Wage growth — sticky inflation indicator
	WageGrowth     float64     // AHETPI — Average Hourly Earnings YoY%
	WageGrowthTrend SeriesTrend

	// Forward inflation expectations
	ForwardInflation float64 // T5YIFR — 5Y5Y Forward Inflation Expectation Rate

	// ISM New Orders — leading growth indicator
	ISMNewOrders     float64     // NAPMNOI — ISM Manufacturing New Orders Index
	ISMNewOrdersTrend SeriesTrend

	// Nonfarm Payrolls — labor breadth
	NFP       float64     // PAYEMS — Nonfarm Payrolls (level, thousands)
	NFPChange float64     // MoM change (thousands)
	NFPTrend  SeriesTrend

	// Consumer Sentiment — leading growth proxy
	ConsumerSentiment     float64     // UMCSENT — UMich Consumer Sentiment
	ConsumerSentimentTrend SeriesTrend

	// Sentiment surveys (populated separately via sentiment package)
	CNNFearGreed float64 // 0-100 (0=Extreme Fear, 100=Extreme Greed)
	AAIIBullBear float64 // Bull/Bear ratio (>1 = bullish sentiment)

	// --- Extended Labor Market ---
	JOLTSOpenings      float64     // JTSJOL — JOLTS Job Openings (thousands)
	JOLTSOpeningsTrend SeriesTrend
	JOLTSQuitRate      float64     // JTSQUR — JOLTS Quit Rate (%)
	JOLTSQuitRateTrend SeriesTrend
	ContinuingClaims      float64     // CCSA — Continuing Claims
	ContinuingClaimsTrend SeriesTrend
	U6Unemployment     float64     // LNS13025703 — U-6 Unemployment Rate (%)
	EmpPopRatio        float64     // EMRATIO — Employment-Population Ratio (%)

	// --- Extended Inflation ---
	MedianCPI          float64     // MEDCPIM158SFRBCLE — Cleveland Fed Median CPI (%)
	MedianCPITrend     SeriesTrend
	StickyCPI          float64     // CORESTICKM159SFRBATL — Atlanta Fed Sticky CPI (%)
	StickyCPITrend     SeriesTrend
	PPICommodities     float64     // PPIACO — PPI All Commodities (YoY%)
	PPICommoditiesTrend SeriesTrend
	MichInflExp1Y      float64     // MICH — Michigan Inflation Expectations 1Y (%)
	ClevelandInfExp1Y  float64     // EXPINF1YR — Cleveland Fed Expected Inflation 1Y (%)
	ClevelandInfExp10Y float64     // EXPINF10YR — Cleveland Fed Expected Inflation 10Y (%)

	// --- Extended Yield Curve ---
	Yield1Y     float64 // DGS1 — 1-Year Treasury
	Yield7Y     float64 // DGS7 — 7-Year Treasury
	Yield20Y    float64 // DGS20 — 20-Year Treasury
	RealYield10Y float64 // DFII10 — 10Y TIPS Real Yield (%)
	RealYield5Y  float64 // DFII5 — 5Y TIPS Real Yield (%)
	Spread10Y2Y  float64 // T10Y2Y — pre-computed by FRED
	Spread10Y3M  float64 // T10Y3M — pre-computed by FRED

	// --- Credit & Financial Conditions ---
	BBBSpread         float64     // BAMLC0A4CBBB — BBB Corporate Spread (%)
	AAASpread         float64     // BAMLC0A1CAAA — AAA Corporate Spread (%)
	StLouisStress     float64     // STLFSI4 — St. Louis Financial Stress Index
	StLouisStressTrend SeriesTrend
	ReverseRepo       float64     // RRPONTSYD — Reverse Repo (billions)

	// --- Housing & Consumer ---
	HousingStarts      float64     // HOUST — Housing Starts (thousands, ann.)
	HousingStartsTrend SeriesTrend
	BuildingPermits      float64     // PERMIT — Building Permits (thousands, ann.)
	BuildingPermitsTrend SeriesTrend
	CaseShillerHPI     float64     // CSUSHPINSA — Case-Shiller Home Price Index
	MortgageRate30Y    float64     // MORTGAGE30US — 30Y Mortgage Rate (%)
	RetailSalesExFood  float64     // RSXFS — Retail Sales Ex Food (YoY%)
	SavingsRate        float64     // PSAVERT — Personal Savings Rate (%)

	// --- VIX Term Structure ---
	VIX3M         float64 // VXVCLS — VIX3M (3-Month VIX)
	VIXTermRatio  float64 // Computed: VIX / VIX3M
	VIXTermRegime string  // BACKWARDATION, FLAT, CONTANGO

	// --- Global Macro: Eurozone ---
	EZ_CPI          float64 // CP0000EZ19M086NEST — Eurozone HICP (YoY%)
	EZ_GDP          float64 // CLVMNACSCAB1GQEA19 — Eurozone Real GDP (QoQ%)
	EZ_Unemployment float64 // LRHUTTTTEZM156S — Eurozone Unemployment (%)
	EZ_Rate         float64 // IR3TIB01EZM156N — Eurozone 3M Interbank Rate (%)

	// --- Global Macro: UK ---
	UK_CPI          float64 // GBRCPIALLMINMEI — UK CPI (YoY%)
	UK_Unemployment float64 // LRHUTTTTGBM156S — UK Unemployment (%)

	// --- Global Macro: Japan ---
	JP_CPI          float64 // JPNCPIALLMINMEI — Japan CPI (YoY%)
	JP_Unemployment float64 // LRHUTTTTJPM156S — Japan Unemployment (%)
	JP_10Y          float64 // IRLTLT01JPM156N — Japan 10Y Bond Yield (%)

	// --- Global Macro: Australia ---
	AU_CPI          float64 // AUSCPIALLQINMEI — Australia CPI (QoQ%)
	AU_Unemployment float64 // LRHUTTTTAUM156S — Australia Unemployment (%)

	// --- Global Macro: Canada ---
	CA_CPI          float64 // CANCPIALLMINMEI — Canada CPI (YoY%)
	CA_Unemployment float64 // LRHUTTTTCAM156S — Canada Unemployment (%)

	// --- Global Macro: New Zealand ---
	NZ_CPI          float64 // NZLCPIALLQINMEI — NZ CPI (QoQ%)

	// --- CBOE Put/Call (populated separately) ---
	PutCallTotal  float64 // CBOE Total Put/Call Ratio
	PutCallEquity float64 // CBOE Equity Put/Call Ratio
	PutCallIndex  float64 // CBOE Index Put/Call Ratio

	FetchedAt time.Time
}

// fredResponse is the JSON structure returned by the FRED observations endpoint.
type fredResponse struct {
	Observations []struct {
		Date  string `json:"date"`
		Value string `json:"value"`
	} `json:"observations"`
}

// parsedObs holds parsed non-missing FRED observations in descending order.
type parsedObs []float64

// FetchMacroData fetches the latest values for all tracked series from FRED.
// It fetches all series in parallel (max 10 concurrent) and is resilient to
// individual failures. If FRED_API_KEY is not set, it uses an empty string.
func FetchMacroData(ctx context.Context) (*MacroData, error) {
	apiKey := os.Getenv("FRED_API_KEY")
	data := &MacroData{FetchedAt: time.Now()}
	client := &http.Client{Timeout: 15 * time.Second}

	// Define all series to fetch
	type fetchJob struct {
		id    string
		limit int
	}

	jobs := []fetchJob{
		// Existing yield curve
		{"DGS2", 5}, {"DGS5", 5}, {"DGS10", 5}, {"DGS30", 5}, {"DGS3MO", 5},
		// Extended yield curve
		{"DGS1", 5}, {"DGS7", 5}, {"DGS20", 5},
		{"DFII10", 5}, {"DFII5", 5},
		{"T10Y2Y", 5}, {"T10Y3M", 5},
		// Inflation
		{"T10YIE", 5}, {"PCEPILFE", 14}, {"CPIAUCSL", 14}, {"T5YIFR", 5}, {"AHETPI", 14},
		// Extended inflation
		{"MEDCPIM158SFRBCLE", 3}, {"CORESTICKM159SFRBATL", 3},
		{"PPIACO", 14}, {"MICH", 3},
		{"EXPINF1YR", 3}, {"EXPINF10YR", 3},
		// Financial stress
		{"NFCI", 3}, {"BAMLH0A0HYM2", 5},
		// Extended credit
		{"BAMLC0A4CBBB", 5}, {"BAMLC0A1CAAA", 5},
		{"STLFSI4", 3}, {"RRPONTSYD", 3},
		// Short-term rates
		{"SOFR", 5}, {"IORB", 5},
		// VIX + term structure
		{"VIXCLS", 5}, {"VXVCLS", 5},
		// Labor
		{"ICSA", 3}, {"UNRATE", 5}, {"PAYEMS", 3},
		// Extended labor
		{"JTSJOL", 3}, {"JTSQUR", 3},
		{"CCSA", 3}, {"LNS13025703", 5}, {"EMRATIO", 5},
		// Monetary policy
		{"FEDFUNDS", 5}, {"M2SL", 14},
		// Growth
		{"A191RL1Q225SBEA", 5}, {"SAHMCURRENT", 5}, {"NAPMNOI", 3}, {"UMCSENT", 3},
		// Fed balance sheet
		{"WALCL", 3},
		// USD
		{"DTWEXBGS", 5},
		// Housing & Consumer
		{"HOUST", 3}, {"PERMIT", 3}, {"CSUSHPINSA", 3},
		{"MORTGAGE30US", 3}, {"RSXFS", 14}, {"PSAVERT", 3},
		// Global - Eurozone
		{"CP0000EZ19M086NEST", 14}, {"CLVMNACSCAB1GQEA19", 5},
		{"LRHUTTTTEZM156S", 5}, {"IR3TIB01EZM156N", 5},
		// Global - UK
		{"GBRCPIALLMINMEI", 14}, {"LRHUTTTTGBM156S", 5},
		// Global - Japan
		{"JPNCPIALLMINMEI", 14}, {"LRHUTTTTJPM156S", 5}, {"IRLTLT01JPM156N", 5},
		// Global - Australia
		{"AUSCPIALLQINMEI", 5}, {"LRHUTTTTAUM156S", 5},
		// Global - Canada
		{"CANCPIALLMINMEI", 14}, {"LRHUTTTTCAM156S", 5},
		// Global - NZ
		{"NZLCPIALLQINMEI", 5},
	}

	// Parallel fetch with semaphore
	type fetchResult struct {
		id  string
		obs parsedObs
	}

	results := make([]fetchResult, len(jobs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // max 10 concurrent

	for i, job := range jobs {
		wg.Add(1)
		go func(idx int, j fetchJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			obs := fetchSeries(ctx, client, j.id, apiKey, j.limit)
			results[idx] = fetchResult{id: j.id, obs: obs}
		}(i, job)
	}
	wg.Wait()

	// Build lookup map
	obsMap := make(map[string]parsedObs)
	for _, r := range results {
		if len(r.obs) > 0 {
			obsMap[r.id] = r.obs
		}
	}

	// --- Map results to MacroData fields ---
	// Helper for single-value assignment
	single := func(id string) float64 {
		if obs, ok := obsMap[id]; ok && len(obs) > 0 {
			return obs[0]
		}
		return 0
	}

	// Helper for trend computation
	trend := func(id string, threshold float64) (float64, SeriesTrend) {
		obs := obsMap[id]
		if len(obs) >= 2 {
			return obs[0], computeTrend(obs[0], obs[1], threshold)
		}
		if len(obs) >= 1 {
			return obs[0], SeriesTrend{}
		}
		return 0, SeriesTrend{}
	}

	// Helper for YoY% from index series (needs 13+ observations)
	yoy := func(id string) (float64, SeriesTrend) {
		obs := obsMap[id]
		if len(obs) >= 13 && obs[12] != 0 {
			val := (obs[0] - obs[12]) / obs[12] * 100
			t := computeTrend(obs[0], obs[1], 0.05)
			return val, t
		}
		if len(obs) >= 1 {
			return obs[0], SeriesTrend{}
		}
		return 0, SeriesTrend{}
	}

	// Yield curve
	data.Yield2Y = single("DGS2")
	data.Yield5Y = single("DGS5")
	data.Yield10Y = single("DGS10")
	data.Yield30Y = single("DGS30")
	data.Yield3M = single("DGS3MO")
	data.Yield1Y = single("DGS1")
	data.Yield7Y = single("DGS7")
	data.Yield20Y = single("DGS20")
	data.RealYield10Y = single("DFII10")
	data.RealYield5Y = single("DFII5")
	data.Spread10Y2Y = single("T10Y2Y")
	data.Spread10Y3M = single("T10Y3M")

	// Inflation
	data.Breakeven5Y = single("T10YIE")
	data.ForwardInflation = single("T5YIFR")
	data.CorePCE, data.CorePCETrend = yoy("PCEPILFE")
	data.CPI, data.CPITrend = yoy("CPIAUCSL")
	data.WageGrowth, data.WageGrowthTrend = yoy("AHETPI")
	// Extended inflation
	data.MedianCPI, data.MedianCPITrend = trend("MEDCPIM158SFRBCLE", 0.1)
	data.StickyCPI, data.StickyCPITrend = trend("CORESTICKM159SFRBATL", 0.1)
	data.PPICommodities, data.PPICommoditiesTrend = yoy("PPIACO")
	data.MichInflExp1Y = single("MICH")
	data.ClevelandInfExp1Y = single("EXPINF1YR")
	data.ClevelandInfExp10Y = single("EXPINF10YR")

	// Financial stress
	data.NFCI, data.NFCITrend = trend("NFCI", 0.02)
	data.TedSpread = single("BAMLH0A0HYM2") // HY OAS as credit stress proxy
	// Extended credit
	data.BBBSpread = single("BAMLC0A4CBBB")
	data.AAASpread = single("BAMLC0A1CAAA")
	data.StLouisStress, data.StLouisStressTrend = trend("STLFSI4", 0.1)
	data.ReverseRepo = single("RRPONTSYD")

	// Short-term rates
	data.SOFR = single("SOFR")
	data.IORB = single("IORB")

	// VIX + term structure
	data.VIX, data.VIXTrend = trend("VIXCLS", 1.0)
	data.VIX3M = single("VXVCLS")
	if data.VIX > 0 && data.VIX3M > 0 {
		data.VIXTermRatio = data.VIX / data.VIX3M
		switch {
		case data.VIXTermRatio > 1.0:
			data.VIXTermRegime = "BACKWARDATION"
		case data.VIXTermRatio > 0.9:
			data.VIXTermRegime = "FLAT"
		default:
			data.VIXTermRegime = "CONTANGO"
		}
	}

	// Labor
	data.UnemployRate = single("UNRATE")
	data.InitialClaims, data.ClaimsTrend = trend("ICSA", 5_000)
	// NFP
	if obs := obsMap["PAYEMS"]; len(obs) >= 2 {
		data.NFP = obs[0]
		data.NFPChange = obs[0] - obs[1]
		data.NFPTrend = computeTrend(obs[0], obs[1], 50)
	} else if len(obs) >= 1 {
		data.NFP = obs[0]
	}
	// Extended labor
	data.JOLTSOpenings, data.JOLTSOpeningsTrend = trend("JTSJOL", 50)
	data.JOLTSQuitRate, data.JOLTSQuitRateTrend = trend("JTSQUR", 0.1)
	data.ContinuingClaims, data.ContinuingClaimsTrend = trend("CCSA", 10_000)
	data.U6Unemployment = single("LNS13025703")
	data.EmpPopRatio = single("EMRATIO")

	// Monetary policy
	data.FedFundsRate = single("FEDFUNDS")
	// M2
	if obs := obsMap["M2SL"]; len(obs) >= 2 {
		var yoyBase float64
		if len(obs) >= 13 {
			yoyBase = obs[12]
		} else {
			yoyBase = obs[len(obs)-1]
		}
		if yoyBase != 0 {
			data.M2Growth = (obs[0] - yoyBase) / yoyBase * 100
		}
		data.M2GrowthTrend = computeTrend(obs[0], obs[1], 50)
	}

	// Growth & recession
	data.GDPGrowth = single("A191RL1Q225SBEA")
	data.SahmRule = single("SAHMCURRENT")
	data.ISMNewOrders, data.ISMNewOrdersTrend = trend("NAPMNOI", 0.5)
	data.ConsumerSentiment, data.ConsumerSentimentTrend = trend("UMCSENT", 1.0)

	// Fed balance sheet
	data.FedBalSheet, data.FedBalSheetTrend = trend("WALCL", 50)

	// USD
	data.DXY = single("DTWEXBGS")

	// Housing & Consumer
	data.HousingStarts, data.HousingStartsTrend = trend("HOUST", 10)
	data.BuildingPermits, data.BuildingPermitsTrend = trend("PERMIT", 10)
	data.CaseShillerHPI = single("CSUSHPINSA")
	data.MortgageRate30Y = single("MORTGAGE30US")
	data.RetailSalesExFood, _ = yoy("RSXFS")
	data.SavingsRate = single("PSAVERT")

	// Global - Eurozone
	data.EZ_CPI, _ = yoy("CP0000EZ19M086NEST")
	data.EZ_GDP = single("CLVMNACSCAB1GQEA19")
	data.EZ_Unemployment = single("LRHUTTTTEZM156S")
	data.EZ_Rate = single("IR3TIB01EZM156N")

	// Global - UK
	data.UK_CPI, _ = yoy("GBRCPIALLMINMEI")
	data.UK_Unemployment = single("LRHUTTTTGBM156S")

	// Global - Japan
	data.JP_CPI, _ = yoy("JPNCPIALLMINMEI")
	data.JP_Unemployment = single("LRHUTTTTJPM156S")
	data.JP_10Y = single("IRLTLT01JPM156N")

	// Global - Australia
	data.AU_CPI = single("AUSCPIALLQINMEI")
	data.AU_Unemployment = single("LRHUTTTTAUM156S")

	// Global - Canada
	data.CA_CPI, _ = yoy("CANCPIALLMINMEI")
	data.CA_Unemployment = single("LRHUTTTTCAM156S")

	// Global - NZ
	data.NZ_CPI = single("NZLCPIALLQINMEI")

	// --- Derived metrics ---
	data.YieldSpread = data.Yield10Y - data.Yield2Y
	if data.Yield3M > 0 && data.Yield10Y > 0 {
		data.Spread3M10Y = data.Yield10Y - data.Yield3M
	}
	if data.Yield2Y > 0 && data.Yield30Y > 0 {
		data.Spread2Y30Y = data.Yield30Y - data.Yield2Y
	}
	if data.YieldSpread != 0 {
		data.YieldSpreadTrend = SeriesTrend{Latest: data.YieldSpread, Direction: "FLAT"}
	}

	// Sanitize: replace any NaN/Inf with 0 to prevent propagation through
	// regime classification, conviction scoring, and AI prompts.
	sanitizeFloat(&data.Yield2Y)
	sanitizeFloat(&data.Yield5Y)
	sanitizeFloat(&data.Yield10Y)
	sanitizeFloat(&data.Yield30Y)
	sanitizeFloat(&data.Yield3M)
	sanitizeFloat(&data.YieldSpread)
	sanitizeFloat(&data.Spread3M10Y)
	sanitizeFloat(&data.Spread2Y30Y)
	sanitizeFloat(&data.CorePCE)
	sanitizeFloat(&data.CPI)
	sanitizeFloat(&data.Breakeven5Y)
	sanitizeFloat(&data.FedFundsRate)
	sanitizeFloat(&data.SOFR)
	sanitizeFloat(&data.IORB)
	sanitizeFloat(&data.NFCI)
	sanitizeFloat(&data.InitialClaims)
	sanitizeFloat(&data.UnemployRate)
	sanitizeFloat(&data.SahmRule)
	sanitizeFloat(&data.GDPGrowth)
	sanitizeFloat(&data.M2Growth)
	sanitizeFloat(&data.FedBalSheet)
	sanitizeFloat(&data.DXY)
	sanitizeFloat(&data.TedSpread)
	sanitizeFloat(&data.VIX)
	sanitizeFloat(&data.WageGrowth)
	sanitizeFloat(&data.ForwardInflation)
	sanitizeFloat(&data.ISMNewOrders)
	sanitizeFloat(&data.NFP)
	sanitizeFloat(&data.NFPChange)
	sanitizeFloat(&data.ConsumerSentiment)

	// Extended Labor Market
	sanitizeFloat(&data.JOLTSOpenings)
	sanitizeFloat(&data.JOLTSQuitRate)
	sanitizeFloat(&data.ContinuingClaims)
	sanitizeFloat(&data.U6Unemployment)
	sanitizeFloat(&data.EmpPopRatio)

	// Extended Inflation
	sanitizeFloat(&data.MedianCPI)
	sanitizeFloat(&data.StickyCPI)
	sanitizeFloat(&data.PPICommodities)
	sanitizeFloat(&data.MichInflExp1Y)
	sanitizeFloat(&data.ClevelandInfExp1Y)
	sanitizeFloat(&data.ClevelandInfExp10Y)

	// Extended Yield Curve
	sanitizeFloat(&data.Yield1Y)
	sanitizeFloat(&data.Yield7Y)
	sanitizeFloat(&data.Yield20Y)
	sanitizeFloat(&data.RealYield10Y)
	sanitizeFloat(&data.RealYield5Y)
	sanitizeFloat(&data.Spread10Y2Y)
	sanitizeFloat(&data.Spread10Y3M)

	// Credit & Financial Conditions
	sanitizeFloat(&data.BBBSpread)
	sanitizeFloat(&data.AAASpread)
	sanitizeFloat(&data.StLouisStress)
	sanitizeFloat(&data.ReverseRepo)

	// Housing & Consumer
	sanitizeFloat(&data.HousingStarts)
	sanitizeFloat(&data.BuildingPermits)
	sanitizeFloat(&data.CaseShillerHPI)
	sanitizeFloat(&data.MortgageRate30Y)
	sanitizeFloat(&data.RetailSalesExFood)
	sanitizeFloat(&data.SavingsRate)

	// VIX Term Structure
	sanitizeFloat(&data.VIX3M)
	sanitizeFloat(&data.VIXTermRatio)

	// Global Macro: Eurozone
	sanitizeFloat(&data.EZ_CPI)
	sanitizeFloat(&data.EZ_GDP)
	sanitizeFloat(&data.EZ_Unemployment)
	sanitizeFloat(&data.EZ_Rate)

	// Global Macro: UK
	sanitizeFloat(&data.UK_CPI)
	sanitizeFloat(&data.UK_Unemployment)

	// Global Macro: Japan
	sanitizeFloat(&data.JP_CPI)
	sanitizeFloat(&data.JP_Unemployment)
	sanitizeFloat(&data.JP_10Y)

	// Global Macro: Australia
	sanitizeFloat(&data.AU_CPI)
	sanitizeFloat(&data.AU_Unemployment)

	// Global Macro: Canada
	sanitizeFloat(&data.CA_CPI)
	sanitizeFloat(&data.CA_Unemployment)

	// Global Macro: New Zealand
	sanitizeFloat(&data.NZ_CPI)

	// CBOE Put/Call
	sanitizeFloat(&data.PutCallTotal)
	sanitizeFloat(&data.PutCallEquity)
	sanitizeFloat(&data.PutCallIndex)

	return data, nil
}

// fetchSeries fetches up to `limit` non-missing observations for a FRED series.
// Returns values in descending chronological order (obs[0] = most recent).
func fetchSeries(ctx context.Context, client *http.Client, seriesID, apiKey string, limit int) parsedObs {
	url := buildFREDURL(seriesID, apiKey, limit)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Error().Str("series", seriesID).Err(err).Msg("failed to build request")
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error().Str("series", seriesID).Err(err).Msg("request failed")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Error().Str("series", seriesID).Int("status", resp.StatusCode).Msg("FRED API non-2xx response")
		return nil
	}

	var result fredResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Error().Str("series", seriesID).Err(err).Msg("decode failed")
		return nil
	}

	var values []float64
	for _, obs := range result.Observations {
		if obs.Value == "." || obs.Value == "" {
			continue
		}
		v, parseErr := strconv.ParseFloat(obs.Value, 64)
		if parseErr != nil {
			continue
		}
		values = append(values, v)
	}
	return values
}

// buildFREDURL constructs the FRED API observations URL for a series.
func buildFREDURL(seriesID, apiKey string, limit int) string {
	base := fmt.Sprintf(
		"https://api.stlouisfed.org/fred/series/observations?series_id=%s&file_type=json&limit=%d&sort_order=desc",
		seriesID,
		limit,
	)
	if apiKey != "" {
		base += "&api_key=" + apiKey
	}
	return base
}

// sanitizeFloat replaces NaN or Inf with 0 to prevent propagation.
func sanitizeFloat(v *float64) {
	if math.IsNaN(*v) || math.IsInf(*v, 0) {
		*v = 0
	}
}
