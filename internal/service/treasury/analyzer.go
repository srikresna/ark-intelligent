package treasury

import (
	"math"
	"sort"
)

// bidToCoverThresholds define demand strength levels.
const (
	strongDemandBTC = 2.5  // Bid-to-cover >= 2.5 is strong demand
	weakDemandBTC   = 2.0  // Bid-to-cover < 2.0 is weak demand
	trendThreshold  = 0.15 // >15% change in avg = trending
)

// analyzeAuctions computes trend analysis grouped by security term.
// Groups by term (e.g. "10-Year", "2-Year") and computes bid-to-cover
// and indirect bidder trends over recent auctions.
func analyzeAuctions(auctions []ParsedAuction) []AuctionAnalysis {
	// Group by security term.
	byTerm := make(map[string][]ParsedAuction)
	for _, a := range auctions {
		if a.SecurityTerm == "" {
			continue
		}
		byTerm[a.SecurityTerm] = append(byTerm[a.SecurityTerm], a)
	}

	var analyses []AuctionAnalysis
	for term, group := range byTerm {
		if len(group) < 2 {
			continue // need at least 2 data points for trend
		}

		// Sort by date ascending for trend analysis.
		sort.Slice(group, func(i, j int) bool {
			return group[i].AuctionDate.Before(group[j].AuctionDate)
		})

		a := analyzeGroup(term, group)
		analyses = append(analyses, a)
	}

	// Sort analyses by importance: Notes/Bonds first, then by term length.
	sort.Slice(analyses, func(i, j int) bool {
		return termPriority(analyses[i].SecurityTerm) < termPriority(analyses[j].SecurityTerm)
	})

	return analyses
}

// analyzeGroup computes trends for a group of auctions of the same term.
func analyzeGroup(term string, group []ParsedAuction) AuctionAnalysis {
	latest := group[len(group)-1]

	// Compute averages (excluding latest for comparison).
	var sumBTC, sumIndirect float64
	var countBTC, countIndirect int
	for i := 0; i < len(group)-1; i++ {
		if group[i].BidToCover > 0 {
			sumBTC += group[i].BidToCover
			countBTC++
		}
		if group[i].IndirectPct > 0 {
			sumIndirect += group[i].IndirectPct
			countIndirect++
		}
	}

	avgBTC := 0.0
	if countBTC > 0 {
		avgBTC = sumBTC / float64(countBTC)
	}
	avgIndirect := 0.0
	if countIndirect > 0 {
		avgIndirect = sumIndirect / float64(countIndirect)
	}

	// Bid-to-cover trend.
	btcTrend := "STABLE"
	if avgBTC > 0 {
		change := (latest.BidToCover - avgBTC) / avgBTC
		if change > trendThreshold {
			btcTrend = "IMPROVING"
		} else if change < -trendThreshold {
			btcTrend = "DETERIORATING"
		}
	}

	// Indirect bidder trend.
	indTrend := "STABLE"
	if avgIndirect > 0 {
		change := (latest.IndirectPct - avgIndirect) / avgIndirect
		if change > trendThreshold {
			indTrend = "RISING"
		} else if change < -trendThreshold {
			indTrend = "FALLING"
		}
	}

	// Overall demand signal.
	demandSignal := "NORMAL"
	if latest.BidToCover >= strongDemandBTC {
		demandSignal = "STRONG"
	} else if latest.BidToCover > 0 && latest.BidToCover < weakDemandBTC {
		demandSignal = "WEAK"
	}

	return AuctionAnalysis{
		SecurityTerm:     term,
		LatestBidToCover: latest.BidToCover,
		AvgBidToCover:    math.Round(avgBTC*100) / 100,
		BidToCoverTrend:  btcTrend,
		LatestIndirect:   latest.IndirectPct,
		AvgIndirect:      math.Round(avgIndirect*100) / 100,
		IndirectTrend:    indTrend,
		DemandSignal:     demandSignal,
		Count:            len(group),
	}
}

// termPriority returns a sort key — lower = more important.
func termPriority(term string) int {
	// Order: 2Y, 3Y, 5Y, 7Y, 10Y, 20Y, 30Y, then bills/others.
	priorities := map[string]int{
		"2-Year":  1,
		"3-Year":  2,
		"5-Year":  3,
		"7-Year":  4,
		"10-Year": 5,
		"20-Year": 6,
		"30-Year": 7,
	}
	if p, ok := priorities[term]; ok {
		return p
	}
	return 100 // bills and others go last
}
