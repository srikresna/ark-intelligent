package factors

// scoreCrowding computes a crowding risk score from COT positioning data.
//
// Crowding = asset is heavily owned by one directional side → tail risk.
// This is a RISK PENALTY: high crowding → negative score (avoid).
//
// Inputs from AssetProfile:
//   - COTIndex (0-100): commercial percentile. High = commercially undervalued.
//   - CrowdingIndex: pre-computed from domain.COTAnalysis (0-100 scale).
//   - SpecMomentum4W: recent speculative momentum direction.
//   - SmartMoneyNet: net speculative position.
//
// A highly crowded asset (extremes in spec positioning + fast momentum) is penalized.
func scoreCrowding(profile AssetProfile) float64 {
	// CrowdingIndex from COT analysis: 0 = no crowding, 100 = extreme crowding.
	// We convert to [-1, +1] where -1 = dangerously crowded, +1 = very uncrowded.

	if profile.CrowdingIndex == 0 && profile.COTIndex == 0 {
		// No COT data available — neutral
		return 0
	}

	// Base crowding from pre-computed index
	// 0-100 → flip to risk penalty: 100 crowded = -1 score
	crowdPenalty := -(profile.CrowdingIndex / 100.0 * 2) + 1 // maps 0→+1, 100→-1

	// COT Index as contrarian signal:
	// Very high COTIndex (>80) means smart money is very long → NOT crowded from short side
	// Very low COTIndex (<20) means smart money is very short → contrarian caution
	cotContrib := 0.0
	if profile.COTIndex > 80 {
		// Extreme longs — crowd may reverse sharply
		cotContrib = -0.3
	} else if profile.COTIndex < 20 {
		// Extreme shorts — squeeze risk, but also crowd on short side
		cotContrib = -0.2
	}

	// Speculative momentum acceleration: if spec momentum is fast (both direction and magnitude),
	// the crowd is piling in — increases crowding risk.
	specMomContrib := 0.0
	if profile.SpecMomentum4W > 0.3 {
		specMomContrib = -0.2 // building fast long crowd
	} else if profile.SpecMomentum4W < -0.3 {
		specMomContrib = -0.2 // building fast short crowd
	}

	score := crowdPenalty + cotContrib + specMomContrib
	return clamp1(score)
}
