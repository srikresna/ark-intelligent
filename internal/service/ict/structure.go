package ict

// DetectStructure identifies BOS (Break of Structure) and CHoCH (Change of Character)
// events from a pre-computed list of swing points.
//
// Rules:
//   BOS:   In an uptrend, price closes above the previous swing high (continuation).
//          In a downtrend, price closes below the previous swing low.
//   CHoCH: In an uptrend, price closes below the most recent swing low (reversal).
//          In a downtrend, price closes above the most recent swing high.
//
// To avoid flooding the output, only one CHoCH per trend transition is emitted.
func DetectStructure(swings []swingPoint) []StructureEvent {
	if len(swings) < 4 {
		return nil
	}

	var events []StructureEvent

	// Determine initial bias from first two confirmed swing points.
	var lastHigh, lastLow *swingPoint

	for i := range swings {
		sp := &swings[i]
		if sp.isHigh {
			if lastHigh == nil {
				lastHigh = sp
				continue
			}
			if lastLow != nil {
				// Both pivots established — evaluate structure.
				prevHigh := lastHigh
				if sp.level > prevHigh.level {
					// Higher High — BOS in bullish direction (continuation of uptrend).
					events = appendUniqueStructure(events, StructureEvent{
						Type:      "BOS",
						Direction: "BULLISH",
						Level:     prevHigh.level,
						BarIndex:  sp.barIndex,
					})
				} else if sp.level < prevHigh.level && lastLow != nil {
					// Lower High after a Lower Low — potential CHoCH bearish.
					// Only emit if the previous event was not already a bearish CHoCH.
					if !lastEventIs(events, "CHOCH", "BEARISH") {
						events = append(events, StructureEvent{
							Type:      "CHOCH",
							Direction: "BEARISH",
							Level:     sp.level,
							BarIndex:  sp.barIndex,
						})
					}
				}
			}
			lastHigh = sp
		} else {
			// Swing Low
			if lastLow == nil {
				lastLow = sp
				continue
			}
			if lastHigh != nil {
				prevLow := lastLow
				if sp.level < prevLow.level {
					// Lower Low — BOS in bearish direction.
					events = appendUniqueStructure(events, StructureEvent{
						Type:      "BOS",
						Direction: "BEARISH",
						Level:     prevLow.level,
						BarIndex:  sp.barIndex,
					})
				} else if sp.level > prevLow.level && lastHigh != nil {
					// Higher Low after a Higher High — potential CHoCH bullish.
					if !lastEventIs(events, "CHOCH", "BULLISH") {
						events = append(events, StructureEvent{
							Type:      "CHOCH",
							Direction: "BULLISH",
							Level:     sp.level,
							BarIndex:  sp.barIndex,
						})
					}
				}
			}
			lastLow = sp
		}
	}

	// Return last 6 events for readability.
	if len(events) > 6 {
		events = events[len(events)-6:]
	}
	return events
}

// appendUniqueStructure appends a BOS event only if not identical to the previous BOS.
func appendUniqueStructure(events []StructureEvent, e StructureEvent) []StructureEvent {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == "BOS" && events[i].Direction == e.Direction {
			if e.Level != 0 && abs64(events[i].Level-e.Level)/abs64(e.Level) < 0.001 {
				return events // duplicate, skip
			}
			break
		}
	}
	return append(events, e)
}

// lastEventIs checks if the most recent event of a given Type+Direction already exists.
func lastEventIs(events []StructureEvent, typ, direction string) bool {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == typ {
			return events[i].Direction == direction
		}
	}
	return false
}

// currentBias derives the structural bias from the last CHoCH or BOS event.
func currentBias(events []StructureEvent) string {
	for i := len(events) - 1; i >= 0; i-- {
		return events[i].Direction
	}
	return "NEUTRAL"
}
