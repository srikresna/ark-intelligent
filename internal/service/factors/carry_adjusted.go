package factors

// scoreCarryAdjusted computes a carry-adjusted momentum score.
//
// Logic:
//
//	rawMom = cross-sectional momentum score
//	carry = CarryBps (FX rate differential) or FundingRate (crypto perpetual)
//
// The carry adjusts expected return from holding the position:
//
//	carryAdj = rawMom + alpha * normalizedCarry
//
// carryBps is in basis points per year.
// For FX: positive means the asset currency earns more than USD (favorable for long).
// For crypto: negative funding rate = longs paid by shorts = favorable for long.
func scoreCarryAdjusted(profile AssetProfile) float64 {
	mom := scoreMomentum(profile.DailyCloses)

	// Normalize carry to roughly the same scale as momentum returns.
	// Carry is in bps/year, momentum is a fraction. 100bps = 1% = 0.01.
	var normalizedCarry float64
	if profile.IsCrypto && profile.FundingRate != 0 {
		// Perpetual funding: positive = longs pay (unfavorable), negative = longs receive (favorable).
		// Annualized already (passed in as bps).
		normalizedCarry = -profile.FundingRate / 10000.0 // convert bps to fraction
	} else {
		normalizedCarry = profile.CarryBps / 10000.0 // convert bps to fraction
	}

	// Alpha: how much weight to give carry vs momentum.
	// 0.3 = carry contributes up to 30% of the signal.
	const alpha = 0.3

	raw := mom + alpha*normalizedCarry
	return clamp1(raw)
}
