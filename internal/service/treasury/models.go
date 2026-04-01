// Package treasury provides integration with the US TreasuryDirect API
// for bond auction results. Bid-to-cover ratios and indirect bidder percentages
// are important signals for FX and bond markets — weak auctions can trigger
// USD weakness and yield spikes.
//
// API: https://www.treasurydirect.gov/TA_WS/securities/search
// No authentication required. US government public data.
// Cache TTL: 12 hours (auctions are periodic).
package treasury

import "time"

// AuctionResult represents a single Treasury auction from the TreasuryDirect API.
type AuctionResult struct {
	CUSIP           string    `json:"cusip"`
	SecurityType    string    `json:"securityType"` // Bill, Note, Bond, TIPS, FRN
	SecurityTerm    string    `json:"securityTerm"` // "4-Week", "10-Year", etc.
	AuctionDate     time.Time `json:"-"`
	AuctionDateStr  string    `json:"auctionDate"` // "01/15/2026"
	IssueDate       string    `json:"issueDate"`
	MaturityDate    string    `json:"maturityDate"`
	HighYield       string    `json:"highYield"`             // Clearing rate as string
	BidToCoverRatio string    `json:"bidToCoverRatio"`       // e.g. "2.45"
	DirectBidder    string    `json:"percentageDealer"`      // Direct bidder %
	IndirectBidder  string    `json:"percentageIndirect"`    // Indirect bidder % (foreign CBs)
	AllottedAmt     string    `json:"totalAccepted"`         // Allotted amount
	OfferingAmt     string    `json:"offeringAmount"`        // Total offering
	CompetitiveTend string    `json:"competitiveTendered"`   // Total competitive bids
}

// ParsedAuction is a cleaned/parsed version of AuctionResult with numeric fields.
type ParsedAuction struct {
	SecurityType    string
	SecurityTerm    string
	AuctionDate     time.Time
	HighYield       float64
	BidToCover      float64
	DirectPct       float64
	IndirectPct     float64
	AllottedAmt     float64
	OfferingAmt     float64
}

// AuctionAnalysis contains trend analysis computed over recent auctions.
type AuctionAnalysis struct {
	SecurityTerm     string
	LatestBidToCover float64
	AvgBidToCover    float64
	BidToCoverTrend  string  // "IMPROVING", "DETERIORATING", "STABLE"
	LatestIndirect   float64
	AvgIndirect      float64
	IndirectTrend    string  // "RISING", "FALLING", "STABLE"
	DemandSignal     string  // "STRONG", "NORMAL", "WEAK"
	Count            int
}

// TreasuryData is the top-level result returned by the treasury service.
type TreasuryData struct {
	Auctions  []ParsedAuction
	Analyses  []AuctionAnalysis
	FetchedAt time.Time
}
