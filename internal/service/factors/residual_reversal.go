package factors

// scoreResidualReversal computes a mean-reversion signal based on OLS residuals.
//
// Method:
//  1. Compute rolling 63-day returns for each asset (done externally — we receive the series).
//  2. Regress asset returns against a "market" proxy (average of all assets in universe).
//  3. The residual (idiosyncratic component) is the deviation from expected return.
//  4. Large positive residuals → expect mean reversion → negative score (overperformed).
//  5. Large negative residuals → expect bounce → positive score (underperformed).
//
// For a single asset, we approximate with a simple z-score of recent returns vs its own
// history (without a cross-sectional market regression, which requires full universe data).
// The engine passes the full universe for proper cross-sectional residuals.
func scoreResidualReversal(closes []float64) float64 {
	if len(closes) < 22 {
		return 0
	}

	// 1-month return
	if closes[20] == 0 {
		return 0
	}
	recent1M := (closes[0] - closes[20]) / closes[20]

	// Historical 1-month returns distribution (using ~6 months of non-overlapping months)
	historicalRets := make([]float64, 0, 6)
	step := 21
	for start := 21; start+step < len(closes); start += step {
		if closes[start+step] == 0 {
			continue
		}
		r := (closes[start] - closes[start+step]) / closes[start+step]
		historicalRets = append(historicalRets, r)
	}
	if len(historicalRets) < 3 {
		return 0
	}

	// Z-score of recent1M relative to its own history
	histMean, histStd := meanStd(historicalRets)
	if histStd == 0 {
		return 0
	}
	z := (recent1M - histMean) / histStd

	// Reversal: negate the z-score (high z → likely to mean-revert → short signal)
	// Clamp at ±3σ then scale to [-1, +1]
	if z > 3 {
		z = 3
	}
	if z < -3 {
		z = -3
	}
	return clamp1(-z / 3.0)
}

// crossSectionalResiduals computes OLS residuals for an asset relative to
// universe average return. Call this from the engine when full universe data
// is available for better accuracy.
//
// assetReturns: N-period return series for this asset (newest first)
// universeReturns: N-period return series for the equal-weighted universe
//
// Returns the residual as a score (positive = underperformed → bounce expected).
func crossSectionalResiduals(assetReturns, universeReturns []float64) float64 {
	n := len(assetReturns)
	if n < 10 || len(universeReturns) < n {
		return 0
	}
	universeReturns = universeReturns[:n]

	// OLS: y = alpha + beta * x
	// x = universe returns, y = asset returns
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		sumX += universeReturns[i]
		sumY += assetReturns[i]
		sumXY += assetReturns[i] * universeReturns[i]
		sumX2 += universeReturns[i] * universeReturns[i]
	}
	nf := float64(n)
	denom := nf*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	beta := (nf*sumXY - sumX*sumY) / denom
	alpha := (sumY - beta*sumX) / nf

	// Residual for most recent period = actual - predicted
	predicted := alpha + beta*universeReturns[0]
	residual := assetReturns[0] - predicted

	// Reversal: negate
	return clamp1(-residual * 20) // scale: 5% residual = 1.0
}
