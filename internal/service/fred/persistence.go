package fred

import (
	"context"
	"time"
)

// FREDObservation represents a single persisted FRED data point.
// Defined locally to avoid import cycle with the ports package.
type FREDObservation struct {
	SeriesID string
	Date     time.Time
	Value    float64
}

// FREDPersister defines the persistence interface for FRED snapshots.
// This mirrors ports.FREDRepository but is defined locally to avoid
// an import cycle (ports → fred → ports).
type FREDPersister interface {
	SaveSnapshots(ctx context.Context, observations []FREDObservation) error
}

// PersistenceService persists FRED data snapshots to BadgerDB after each fetch.
type PersistenceService struct {
	repo FREDPersister
}

// NewPersistenceService creates a new FRED persistence service.
func NewPersistenceService(repo FREDPersister) *PersistenceService {
	return &PersistenceService{repo: repo}
}

// PersistSnapshot saves the current MacroData values to BadgerDB.
// Should be called after every successful FetchMacroData.
func (ps *PersistenceService) PersistSnapshot(ctx context.Context, data *MacroData) error {
	if data == nil || ps.repo == nil {
		return nil
	}

	now := time.Now()

	// Build observations from all non-zero MacroData fields
	var obs []FREDObservation

	addObs := func(seriesID string, value float64) {
		if value != 0 {
			obs = append(obs, FREDObservation{
				SeriesID: seriesID,
				Date:     now,
				Value:    value,
			})
		}
	}

	// Yield curve
	addObs("DGS2", data.Yield2Y)
	addObs("DGS5", data.Yield5Y)
	addObs("DGS10", data.Yield10Y)
	addObs("DGS30", data.Yield30Y)
	addObs("DGS3MO", data.Yield3M)
	addObs("DGS1", data.Yield1Y)
	addObs("DGS7", data.Yield7Y)
	addObs("DGS20", data.Yield20Y)
	addObs("DFII10", data.RealYield10Y)
	addObs("DFII5", data.RealYield5Y)
	addObs("T10Y2Y", data.Spread10Y2Y)
	addObs("T10Y3M", data.Spread10Y3M)

	// Inflation
	addObs("T10YIE", data.Breakeven5Y)
	addObs("PCEPILFE", data.CorePCE)
	addObs("CPIAUCSL", data.CPI)
	addObs("T5YIFR", data.ForwardInflation)
	addObs("AHETPI", data.WageGrowth)
	addObs("MEDCPIM158SFRBCLE", data.MedianCPI)
	addObs("CORESTICKM159SFRBATL", data.StickyCPI)
	addObs("PPIACO", data.PPICommodities)
	addObs("MICH", data.MichInflExp1Y)
	addObs("EXPINF1YR", data.ClevelandInfExp1Y)
	addObs("EXPINF10YR", data.ClevelandInfExp10Y)

	// Financial stress
	addObs("NFCI", data.NFCI)
	addObs("BAMLH0A0HYM2", data.TedSpread)
	addObs("BAMLC0A4CBBB", data.BBBSpread)
	addObs("BAMLC0A1CAAA", data.AAASpread)
	addObs("STLFSI4", data.StLouisStress)
	addObs("RRPONTSYD", data.ReverseRepo)

	// Rates
	addObs("SOFR", data.SOFR)
	addObs("IORB", data.IORB)
	addObs("FEDFUNDS", data.FedFundsRate)

	// VIX
	addObs("VIXCLS", data.VIX)
	addObs("VXVCLS", data.VIX3M)

	// Labor
	addObs("ICSA", data.InitialClaims)
	addObs("UNRATE", data.UnemployRate)
	addObs("PAYEMS", data.NFP)
	addObs("JTSJOL", data.JOLTSOpenings)
	addObs("JTSQUR", data.JOLTSQuitRate)
	addObs("JTSHIR", data.JOLTSHiringRate)
	addObs("CCSA", data.ContinuingClaims)
	addObs("LNS13025703", data.U6Unemployment)
	addObs("EMRATIO", data.EmpPopRatio)

	// Growth
	addObs("A191RL1Q225SBEA", data.GDPGrowth)
	addObs("SAHMCURRENT", data.SahmRule)
	addObs("NAPMNOI", data.ISMNewOrders)
	addObs("UMCSENT", data.ConsumerSentiment)

	// M2 & Fed balance
	addObs("M2SL_YOY", data.M2Growth)
	addObs("WALCL", data.FedBalSheet)
	addObs("WDTGAL", data.TGABalance)

	// USD
	addObs("DTWEXBGS", data.DXY)

	// Housing
	addObs("HOUST", data.HousingStarts)
	addObs("PERMIT", data.BuildingPermits)
	addObs("CSUSHPINSA", data.CaseShillerHPI)
	addObs("MORTGAGE30US", data.MortgageRate30Y)
	addObs("RSXFS", data.RetailSalesExFood)
	addObs("PSAVERT", data.SavingsRate)

	// Global
	addObs("CP0000EZ19M086NEST", data.EZ_CPI)
	addObs("CLVMNACSCAB1GQEA19", data.EZ_GDP)
	addObs("LRHUTTTTEZM156S", data.EZ_Unemployment)
	addObs("IR3TIB01EZM156N", data.EZ_Rate)
	addObs("GBRCPIALLMINMEI", data.UK_CPI)
	addObs("LRHUTTTTGBM156S", data.UK_Unemployment)
	addObs("JPNCPIALLMINMEI", data.JP_CPI)
	addObs("LRHUTTTTJPM156S", data.JP_Unemployment)
	addObs("IRLTLT01JPM156N", data.JP_10Y)
	addObs("AUSCPIALLQINMEI", data.AU_CPI)
	addObs("LRHUTTTTAUM156S", data.AU_Unemployment)
	addObs("CANCPIALLMINMEI", data.CA_CPI)
	addObs("LRHUTTTTCAM156S", data.CA_Unemployment)
	addObs("NZLCPIALLQINMEI", data.NZ_CPI)

	if len(obs) == 0 {
		return nil
	}

	log.Info().Int("count", len(obs)).Msg("Persisting FRED snapshots to BadgerDB")
	return ps.repo.SaveSnapshots(ctx, obs)
}
