package fred

import (
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

func TestComputeSpreadRange(t *testing.T) {
	tests := []struct {
		name  string
		pairs []domain.CarryPairSnapshot
		want  float64
	}{
		{
			name:  "empty",
			pairs: nil,
			want:  0,
		},
		{
			name: "single pair",
			pairs: []domain.CarryPairSnapshot{
				{Currency: "AUD", Spread: 100},
			},
			want: 0,
		},
		{
			name: "multiple pairs",
			pairs: []domain.CarryPairSnapshot{
				{Currency: "AUD", Spread: 150},
				{Currency: "EUR", Spread: -50},
				{Currency: "JPY", Spread: -400},
			},
			want: 550, // 150 - (-400)
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeSpreadRange(tt.pairs)
			if got != tt.want {
				t.Errorf("computeSpreadRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyUnwindRisk(t *testing.T) {
	tests := []struct {
		name      string
		current   float64
		prev      float64
		changePct float64
		want      domain.UnwindRisk
	}{
		{"normal range", 300, 310, -3.2, domain.UnwindNormal},
		{"narrowing", 250, 310, -19.4, domain.UnwindNarrow},
		{"unwind collapse", 200, 310, -35.5, domain.UnwindAlert},
		{"very tight spread", 40, 300, -86.7, domain.UnwindAlert},
		{"no prev data", 300, 0, 0, domain.UnwindNormal},
		{"expanding range", 350, 300, 16.7, domain.UnwindNormal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyUnwindRisk(tt.current, tt.prev, tt.changePct)
			if got != tt.want {
				t.Errorf("classifyUnwindRisk() = %v, want %v", got, tt.want)
			}
		})
	}
}
