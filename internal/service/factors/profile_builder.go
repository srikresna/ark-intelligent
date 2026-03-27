package factors

// profile_builder.go provides a ProfileService that reads from existing
// repositories to assemble AssetProfile slices for the Factor Engine.

import (
	"context"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var pblog = logger.Component("factor-profiles")

// ProfileService implements AssetProfileBuilder for the Telegram handler.
type ProfileService struct {
	dailyPriceRepo pricesvc.DailyPriceStore // daily OHLCV (newest-first)
	cotRepo        ports.COTRepository
}

// NewProfileService creates a ProfileService.
func NewProfileService(dailyPriceRepo pricesvc.DailyPriceStore, cotRepo ports.COTRepository) *ProfileService {
	return &ProfileService{
		dailyPriceRepo: dailyPriceRepo,
		cotRepo:        cotRepo,
	}
}

// BuildProfiles assembles AssetProfile slices from stored daily price and COT data.
func (s *ProfileService) BuildProfiles(ctx context.Context) ([]AssetProfile, error) {
	profiles := make([]AssetProfile, 0, len(domain.DefaultCOTContracts))

	for _, contract := range domain.DefaultCOTContracts {
		prices, err := s.dailyPriceRepo.GetDailyHistory(ctx, contract.Code, 300)
		if err != nil || len(prices) < 30 {
			continue
		}

		// Sort newest-first (GetDailyHistory may return any order)
		sort.Slice(prices, func(i, j int) bool {
			return prices[i].Date.After(prices[j].Date)
		})

		closes := make([]float64, len(prices))
		for i, p := range prices {
			closes[i] = p.Close
		}

		// Fetch latest COT analysis
		latestCOT, err := s.cotRepo.GetLatestAnalysis(ctx, contract.Code)
		cotIdx := 0.0
		smNet := 0.0
		crowding := 0.0
		specMom4w := 0.0
		if err == nil && latestCOT != nil {
			cotIdx = latestCOT.COTIndex
			smNet = latestCOT.NetPosition
			crowding = latestCOT.CrowdingIndex
			specMom4w = latestCOT.SpecMomentum4W
		}

		isCrypto := contract.Currency == "BTC" || contract.Currency == "ETH"

		profiles = append(profiles, AssetProfile{
			ContractCode:   contract.Code,
			Currency:       contract.Currency,
			Name:           contract.Name,
			ReportType:     contract.ReportType,
			IsCrypto:       isCrypto,
			IsInverse:      contract.Inverse,
			DailyCloses:    closes,
			COTIndex:       cotIdx,
			SmartMoneyNet:  smNet,
			CrowdingIndex:  crowding,
			SpecMomentum4W: specMom4w,
		})
	}

	pblog.Info().Int("count", len(profiles)).Msg("asset profiles built")
	return profiles, nil
}

// GetMacroRegime returns the current FRED macro regime name.
func (s *ProfileService) GetMacroRegime(ctx context.Context) string {
	data, err := fred.GetCachedOrFetch(ctx)
	if err != nil || data == nil {
		return ""
	}
	regime := fred.ClassifyMacroRegime(data)
	return regime.Name
}

// GetCOTBias returns a map of contractCode → bias string ("BULLISH"/"BEARISH"/"NEUTRAL").
func (s *ProfileService) GetCOTBias(ctx context.Context) map[string]string {
	result := make(map[string]string)
	analyses, err := s.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil {
		return result
	}
	for _, a := range analyses {
		switch {
		case a.IsExtremeBull:
			result[a.Contract.Code] = "BULLISH"
		case a.IsExtremeBear:
			result[a.Contract.Code] = "BEARISH"
		case a.ShortTermBias == "BULLISH":
			result[a.Contract.Code] = "BULLISH"
		case a.ShortTermBias == "BEARISH":
			result[a.Contract.Code] = "BEARISH"
		default:
			result[a.Contract.Code] = "NEUTRAL"
		}
	}
	return result
}

// GetVolRegime returns volatility regime per contractCode.
func (s *ProfileService) GetVolRegime(ctx context.Context) map[string]string {
	result := make(map[string]string)
	for _, contract := range domain.DefaultCOTContracts {
		prices, err := s.dailyPriceRepo.GetDailyHistory(ctx, contract.Code, 40)
		if err != nil || len(prices) < 20 {
			continue
		}
		sort.Slice(prices, func(i, j int) bool {
			return prices[i].Date.After(prices[j].Date)
		})
		recent := dailyPriceVarProxy(prices[:10])
		hist := dailyPriceVarProxy(prices[:30])
		if hist == 0 {
			continue
		}
		ratio := recent / hist
		switch {
		case ratio > 1.3:
			result[contract.Code] = "EXPANDING"
		case ratio < 0.7:
			result[contract.Code] = "CONTRACTING"
		default:
			result[contract.Code] = "NORMAL"
		}
	}
	return result
}

// GetCarryBps returns carry in bps per contractCode.
// Placeholder — future integration with RateDifferentialEngine.
func (s *ProfileService) GetCarryBps(_ context.Context) map[string]float64 {
	return make(map[string]float64)
}

// GetTransitionProb returns transition probability and from/to regime strings.
func (s *ProfileService) GetTransitionProb(ctx context.Context) (float64, string, string) {
	data, err := fred.GetCachedOrFetch(ctx)
	if err != nil || data == nil {
		return 0, "", ""
	}
	current := fred.ClassifyMacroRegime(data).Name

	history, err := fred.FetchHistoricalRegimes(ctx, 4)
	if err != nil || len(history) < 2 {
		return 0.10, current, current
	}

	// Count how many of the last 4 weeks match current regime
	matchCount := 0
	dominant := ""
	counts := make(map[string]int)
	for _, r := range history {
		counts[r]++
	}
	for r, c := range counts {
		if c > matchCount {
			matchCount = c
			dominant = r
		}
	}
	if dominant != current {
		prob := float64(len(history)-counts[current]) / float64(len(history))
		return prob, dominant, current
	}
	return 0.10, current, current
}

// dailyPriceVarProxy computes realized variance as a simple vol proxy (newest-first input).
func dailyPriceVarProxy(prices []domain.DailyPrice) float64 {
	if len(prices) < 2 {
		return 0
	}
	sum := 0.0
	for i := 0; i < len(prices)-1; i++ {
		prev := prices[i+1].Close
		if prev == 0 {
			continue
		}
		r := (prices[i].Close - prev) / prev
		sum += r * r
	}
	return sum / float64(len(prices)-1)
}
