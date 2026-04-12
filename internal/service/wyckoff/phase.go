package wyckoff

import "github.com/arkcode369/ark-intelligent/internal/service/ta"

// ─────────────────────────────────────────────────────────────────────────────
// Phase identification
// Takes the detected event set and assigns Wyckoff phases A → E.
// Bars are oldest-first (index 0 = oldest).
// ─────────────────────────────────────────────────────────────────────────────

// buildPhases constructs WyckoffPhase records from the event set.
// It returns phases in order A, B, C, D, E (only those that have events).
func buildPhases(events []WyckoffEvent, n int) []WyckoffPhase {
	// Phase boundaries determined by key events:
	//  A: PS → ST  (or BC → AR_D for distribution)
	//  B: after A until Spring / UTAD
	//  C: Spring or UTAD event
	//  D: SOS or SOW
	//  E: beyond SOS/SOW

	indexByName := func(name EventName) int {
		for _, e := range events {
			if e.Name == name {
				return e.BarIndex
			}
		}
		return -1
	}

	phases := []WyckoffPhase{}

	// ── Accumulation layout ──────────────────────────────────────────────────
	psIdx := indexByName(EventPS)
	scIdx := indexByName(EventSC)
	arIdx := indexByName(EventAR)
	stIdx := indexByName(EventST)
	springIdx := indexByName(EventSpring)
	sosIdx := indexByName(EventSOS)
	lpsIdx := indexByName(EventLPS)

	// ── Distribution layout ──────────────────────────────────────────────────
	bcIdx := indexByName(EventBC)
	arDistIdx := indexByName(EventARDist)
	utadIdx := indexByName(EventUTAD)
	sowIdx := indexByName(EventSOW)

	lastBar := n - 1
	if lastBar < 0 {
		lastBar = 0
	}

	isAccum := scIdx >= 0 || springIdx >= 0 || sosIdx >= 0
	isDist := bcIdx >= 0 || utadIdx >= 0 || sowIdx >= 0

	if isAccum && !isDist {
		// Phase A
		phA := WyckoffPhase{Phase: "A", Start: 0, End: 0}
		if psIdx >= 0 {
			phA.Start = psIdx
		}
		end := stIdx
		if end < 0 {
			end = arIdx
		}
		if end < 0 {
			end = lastBar
		}
		phA.End = end
		phA.Events = eventsInRange(events, phA.Start, phA.End)
		if len(phA.Events) > 0 {
			phases = append(phases, phA)
		}

		// Phase B
		if arIdx >= 0 {
			phBStart := arIdx + 1
			phBEnd := lastBar
			if springIdx >= 0 {
				phBEnd = springIdx - 1
			}
			if phBEnd < 0 {
				phBEnd = lastBar
			}
			phB := WyckoffPhase{Phase: "B", Start: phBStart, End: phBEnd}
			phB.Events = eventsInRange(events, phBStart, phBEnd)
			if len(phB.Events) > 0 {
				phases = append(phases, phB)
			}
		}

		// Phase C
		if springIdx >= 0 {
			phC := WyckoffPhase{Phase: "C", Start: springIdx, End: springIdx + 5}
			if phC.End >= n {
				phC.End = n - 1
			}
			phC.Events = eventsInRange(events, phC.Start, phC.End)
			phases = append(phases, phC)
		}

		// Phase D
		if sosIdx >= 0 {
			phDStart := sosIdx
			phDEnd := lastBar
			if lpsIdx >= 0 {
				phDEnd = lpsIdx + 5
			}
			if phDEnd >= n {
				phDEnd = lastBar
			}
			phD := WyckoffPhase{Phase: "D", Start: phDStart, End: phDEnd}
			phD.Events = eventsInRange(events, phDStart, phDEnd)
			phases = append(phases, phD)
		}

		// Phase E — markup
		if lpsIdx >= 0 {
			phE := WyckoffPhase{Phase: "E", Start: lpsIdx + 1, End: lastBar}
			phE.Events = eventsInRange(events, phE.Start, phE.End)
			if len(phE.Events) > 0 {
				phases = append(phases, phE)
			}
		}

	} else if isDist {
		// Phase A (Distribution)
		phAEnd := arDistIdx
		if phAEnd < 0 {
			phAEnd = lastBar
		}
		phA := WyckoffPhase{Phase: "A", Start: 0, End: phAEnd}
		phA.Events = eventsInRange(events, phA.Start, phA.End)
		if len(phA.Events) > 0 {
			phases = append(phases, phA)
		}

		// Phase B
		if arDistIdx >= 0 {
			phBEnd := lastBar
			if utadIdx >= 0 {
				phBEnd = utadIdx - 1
			}
			if phBEnd < 0 {
				phBEnd = lastBar
			}
			phB := WyckoffPhase{Phase: "B", Start: arDistIdx + 1, End: phBEnd}
			phB.Events = eventsInRange(events, phB.Start, phB.End)
			if len(phB.Events) > 0 {
				phases = append(phases, phB)
			}
		}

		// Phase C
		if utadIdx >= 0 {
			phCEnd := utadIdx + 5
			if phCEnd >= n {
				phCEnd = lastBar
			}
			phC := WyckoffPhase{Phase: "C", Start: utadIdx, End: phCEnd}
			phC.Events = eventsInRange(events, phC.Start, phC.End)
			phases = append(phases, phC)
		}

		// Phase D
		if sowIdx >= 0 {
			phD := WyckoffPhase{Phase: "D", Start: sowIdx, End: lastBar}
			phD.Events = eventsInRange(events, phD.Start, phD.End)
			phases = append(phases, phD)
		}
	}

	return phases
}

// eventsInRange returns events whose BarIndex falls in [start, end].
// If start < 0 or end < 0 or end < start, returns an empty slice.
func eventsInRange(events []WyckoffEvent, start, end int) []WyckoffEvent {
	if start < 0 || end < 0 || end < start {
		return nil
	}
	var out []WyckoffEvent
	for _, e := range events {
		if e.BarIndex < start {
			continue
		}
		if e.BarIndex > end {
			continue
		}
		out = append(out, e)
	}
	return out
}

// currentPhase returns the label of the last phase that has events.
func currentPhase(phases []WyckoffPhase) string {
	if len(phases) == 0 {
		return "UNDEFINED"
	}
	return phases[len(phases)-1].Phase
}

// tradingRange returns [support, resistance] from SC/AR anchors (accumulation)
// or BC/AR_D anchors (distribution).
func tradingRangeFor(bars []ta.OHLCV, events []WyckoffEvent) [2]float64 {
	var lo, hi float64
	for _, e := range events {
		switch e.Name {
		case EventSC, EventSpring:
			if lo == 0 || bars[e.BarIndex].Low < lo {
				lo = bars[e.BarIndex].Low
			}
		case EventAR, EventSOS:
			if bars[e.BarIndex].High > hi {
				hi = bars[e.BarIndex].High
			}
		case EventBC, EventUTAD:
			if bars[e.BarIndex].High > hi {
				hi = bars[e.BarIndex].High
			}
		case EventARDist:
			if lo == 0 || bars[e.BarIndex].Low < lo {
				lo = bars[e.BarIndex].Low
			}
		}
	}
	return [2]float64{lo, hi}
}
