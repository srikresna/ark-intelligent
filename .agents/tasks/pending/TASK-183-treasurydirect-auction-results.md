# TASK-183: TreasuryDirect Auction API — Bond Auction Results

**Priority:** medium
**Type:** data
**Estimated:** M
**Area:** internal/service/treasury/ (new)

## Deskripsi

Integrasi TreasuryDirect API untuk US Treasury auction results. Bid-to-cover ratio dan indirect bidder % adalah sinyal penting untuk FX dan bond markets.

## Endpoint

`GET https://www.treasurydirect.gov/TA_WS/securities/search?type=Note&pagesize=10&format=json`

Auth: None. US government public data.

## Data Points

- Auction date, security type (Bill, Note, Bond, TIPS)
- High yield (clearing rate)
- Bid-to-cover ratio (demand strength: >2.5 strong, <2.0 weak)
- Direct bidder % (domestic institutions)
- Indirect bidder % (foreign central banks — USD demand proxy)
- Allotted amount

## File Changes

- `internal/service/treasury/client.go` — NEW: TreasuryDirect API client
- `internal/service/treasury/models.go` — NEW: AuctionResult, AuctionAnalysis types
- `internal/service/treasury/analyzer.go` — NEW: Bid-to-cover trend, indirect bidder trend
- `internal/adapter/telegram/handler.go` — Add /auction command routing
- `internal/adapter/telegram/formatter.go` — Add auction results formatting

## Acceptance Criteria

- [ ] Fetch latest 10 Treasury auction results by type
- [ ] Compute bid-to-cover trend (improving/deteriorating)
- [ ] Track indirect bidder % trend (foreign demand for USD)
- [ ] /auction command shows latest results + trend analysis
- [ ] Alert on weak auction (bid-to-cover < 2.0)
- [ ] Cache with 12h TTL (auctions are periodic)
