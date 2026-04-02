package sec

import (
	"math"
	"sort"
	"strings"
)

// topHoldings returns the top N holdings by value (descending).
func topHoldings(holdings []Holding, n int) []Holding {
	if len(holdings) == 0 {
		return nil
	}

	sorted := make([]Holding, len(holdings))
	copy(sorted, holdings)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}

// computeChanges computes quarter-over-quarter portfolio changes.
// Returns all significant changes, new positions, and exits separately.
func computeChanges(current, previous []Holding) (changes, newPositions, exits []PortfolioChange) {
	// Build lookup maps by CUSIP+PutCall for matching.
	type holdKey struct {
		cusip   string
		putCall string
	}

	currMap := make(map[holdKey]Holding, len(current))
	for _, h := range current {
		key := holdKey{cusip: strings.TrimSpace(h.CUSIPClass), putCall: h.PutCall}
		// Aggregate if same CUSIP+PutCall appears multiple times.
		if existing, ok := currMap[key]; ok {
			existing.Value += h.Value
			existing.Shares += h.Shares
			currMap[key] = existing
		} else {
			currMap[key] = h
		}
	}

	prevMap := make(map[holdKey]Holding, len(previous))
	for _, h := range previous {
		key := holdKey{cusip: strings.TrimSpace(h.CUSIPClass), putCall: h.PutCall}
		if existing, ok := prevMap[key]; ok {
			existing.Value += h.Value
			existing.Shares += h.Shares
			prevMap[key] = existing
		} else {
			prevMap[key] = h
		}
	}

	// Find new positions and changes in existing positions.
	for key, curr := range currMap {
		prev, existed := prevMap[key]

		change := PortfolioChange{
			Issuer:     curr.Issuer,
			TitleClass: curr.TitleClass,
			CurrValue:  curr.Value,
			CurrShares: curr.Shares,
		}

		if !existed {
			change.ChangeType = "NEW"
			change.ValueChange = curr.Value
			if curr.Value > 0 {
				change.PctChange = 100 // convention for new positions
			}
			newPositions = append(newPositions, change)
		} else {
			change.PrevValue = prev.Value
			change.PrevShares = prev.Shares
			change.ValueChange = curr.Value - prev.Value

			if prev.Value > 0 {
				change.PctChange = ((curr.Value - prev.Value) / prev.Value) * 100
			}

			// Classify change type based on share changes.
			shareDelta := curr.Shares - prev.Shares
			if prev.Shares > 0 {
				sharePctChange := math.Abs(shareDelta / prev.Shares * 100)
				if sharePctChange < 2 {
					change.ChangeType = "UNCHANGED"
				} else if shareDelta > 0 {
					change.ChangeType = "INCREASE"
				} else {
					change.ChangeType = "DECREASE"
				}
			} else {
				if shareDelta > 0 {
					change.ChangeType = "INCREASE"
				} else {
					change.ChangeType = "UNCHANGED"
				}
			}
		}

		changes = append(changes, change)
	}

	// Find exits (positions in previous but not in current).
	for key, prev := range prevMap {
		if _, exists := currMap[key]; !exists {
			exits = append(exits, PortfolioChange{
				Issuer:      prev.Issuer,
				TitleClass:  prev.TitleClass,
				ChangeType:  "EXIT",
				PrevValue:   prev.Value,
				PrevShares:  prev.Shares,
				ValueChange: -prev.Value,
				PctChange:   -100,
			})
		}
	}

	// Sort changes by absolute value change (most significant first).
	sort.Slice(changes, func(i, j int) bool {
		return math.Abs(changes[i].ValueChange) > math.Abs(changes[j].ValueChange)
	})
	sort.Slice(newPositions, func(i, j int) bool {
		return newPositions[i].CurrValue > newPositions[j].CurrValue
	})
	sort.Slice(exits, func(i, j int) bool {
		return exits[i].PrevValue > exits[j].PrevValue
	})

	return changes, newPositions, exits
}
