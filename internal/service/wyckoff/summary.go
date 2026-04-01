// Package wyckoff — summary.go provides conversion from WyckoffResult to
// ta.WyckoffSummary for embedding in ta.FullResult without circular imports.
package wyckoff

import "github.com/arkcode369/ark-intelligent/internal/service/ta"

// ToSummary converts a full WyckoffResult into a lightweight ta.WyckoffSummary
// suitable for embedding in ta.FullResult. Returns nil if r is nil.
func (r *WyckoffResult) ToSummary() *ta.WyckoffSummary {
	if r == nil {
		return nil
	}
	return &ta.WyckoffSummary{
		Schematic:     r.Schematic,
		CurrentPhase:  r.CurrentPhase,
		Confidence:    r.Confidence,
		TradingRange:  r.TradingRange,
		CauseBuilt:    r.CauseBuilt,
		ProjectedMove: r.ProjectedMove,
		Summary:       r.Summary,
	}
}
