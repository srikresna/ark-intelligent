# TASK-286: GJR-GARCH Asymmetric Volatility — Leverage Effect Detection

**Priority:** medium
**Type:** enhancement
**Estimated:** M
**Area:** internal/service/price/garch.go, internal/adapter/telegram/formatter_quant.go
**Created by:** Research Agent
**Created at:** 2026-04-02 27:00 WIB

## Deskripsi

GARCH(1,1) sudah ada di `price/garch.go` tapi hanya menangkap symmetric volatility clustering. **GJR-GARCH** (Glosten-Jagannathan-Runkle, 1993) menambahkan **leverage effect**: downward moves menghasilkan volatilitas lebih tinggi dari upward moves — pola yang dominan di FX dan equities.

FEATURE_INDEX secara eksplisit menyebut: "EGARCH, GJR-GARCH untuk volatility asymmetry" sebagai area riset.

**Manfaat Praktis:**
- Mendeteksi apakah pair sedang dalam "fear regime" (downside vol > upside vol)
- Position sizing lebih akurat saat market jatuh
- Signal: `AsymmetryCoeff > 0.05` → downside risk elevated → kurangi long conviction

## Model GJR-GARCH(1,1)

```
σ²(t) = ω + α·ε²(t-1) + γ·I(ε(t-1)<0)·ε²(t-1) + β·σ²(t-1)

Di mana:
  I = 1 jika ε(t-1) < 0 (negatif return), 0 jika positif
  γ = asymmetry coefficient (leverage effect)
  γ > 0 → downside shocks more persistent (fear)
  Total persistence = α + γ/2 + β
  Stationarity: α + γ/2 + β < 1
```

## Perubahan yang Diperlukan

### 1. Tambah `GJRGARCHResult` struct di `internal/service/price/garch.go`

```go
// GJRGARCHResult holds the output of a GJR-GARCH(1,1) estimation.
type GJRGARCHResult struct {
	Omega           float64 `json:"omega"`
	Alpha           float64 `json:"alpha"`           // symmetric shock weight
	Gamma           float64 `json:"gamma"`           // asymmetry/leverage coefficient
	Beta            float64 `json:"beta"`            // persistence weight
	Persistence     float64 `json:"persistence"`     // α + γ/2 + β
	AsymmetryRatio  float64 `json:"asymmetry_ratio"` // γ / (2α + γ) * 100 — % of shock from downside
	CurrentVol      float64 `json:"current_vol"`     // √σ²(t)
	ForecastVol1    float64 `json:"forecast_vol_1"`  // 1-step ahead
	AsymmetryLabel  string  `json:"asymmetry_label"` // "HIGH", "MODERATE", "LOW", "NONE"
	LeverageEffect  bool    `json:"leverage_effect"` // γ > 0.05
	Converged       bool    `json:"converged"`
	SampleSize      int     `json:"sample_size"`
}
```

### 2. Tambah `EstimateGJRGARCH()` di `internal/service/price/garch.go`

```go
// EstimateGJRGARCH fits a GJR-GARCH(1,1) model to daily returns.
// Prices must be newest-first. Requires at least 30 observations.
//
// GJR-GARCH(1,1): σ²(t) = ω + α·ε²(t-1) + γ·I(ε<0)·ε²(t-1) + β·σ²(t-1)
func EstimateGJRGARCH(prices []domain.PriceRecord) (*GJRGARCHResult, error) {
	if len(prices) < 30 {
		return nil, fmt.Errorf("insufficient data for GJR-GARCH: need 30, got %d", len(prices))
	}

	n := len(prices)
	returns := make([]float64, 0, n-1)
	for i := n - 1; i > 0; i-- {
		if prices[i].Close <= 0 || prices[i-1].Close <= 0 {
			continue
		}
		returns = append(returns, math.Log(prices[i-1].Close/prices[i].Close))
	}
	return estimateGJRGARCHFromReturns(returns)
}

// estimateGJRGARCHFromReturns fits GJR-GARCH(1,1) via variance targeting + grid search.
func estimateGJRGARCHFromReturns(returns []float64) (*GJRGARCHResult, error) {
	// Variance targeting: ω = σ̄²(1 - α - γ/2 - β)
	// Grid search over (α, γ, β) to maximize log-likelihood
	// Constraints: α ≥ 0, γ ≥ 0, β ≥ 0, α + γ/2 + β < 1, ω > 0
	
	n := len(returns)
	mu := meanSlice(returns)
	
	// Variance target
	varTarget := 0.0
	for _, r := range returns {
		varTarget += (r - mu) * (r - mu)
	}
	varTarget /= float64(n)
	
	bestLL := math.Inf(-1)
	var bestAlpha, bestGamma, bestBeta float64
	
	for _, alpha := range []float64{0.02, 0.05, 0.08, 0.10, 0.12, 0.15} {
		for _, gamma := range []float64{0.0, 0.02, 0.05, 0.08, 0.10, 0.12} {
			for _, beta := range []float64{0.75, 0.80, 0.85, 0.88, 0.90, 0.92} {
				if alpha+gamma/2+beta >= 0.9999 {
					continue
				}
				omega := varTarget * (1 - alpha - gamma/2 - beta)
				if omega <= 0 {
					continue
				}
				ll := gjrLogLikelihood(returns, omega, alpha, gamma, beta)
				if ll > bestLL {
					bestLL = ll
					bestAlpha, bestGamma, bestBeta = alpha, gamma, beta
				}
			}
		}
	}
	
	omega := varTarget * (1 - bestAlpha - bestGamma/2 - bestBeta)
	persistence := bestAlpha + bestGamma/2 + bestBeta
	
	// Compute final conditional variance
	variances := make([]float64, len(returns))
	variances[0] = varTarget
	for t := 1; t < len(returns); t++ {
		e := returns[t-1] - mu
		indicator := 0.0
		if e < 0 {
			indicator = 1.0
		}
		variances[t] = omega + bestAlpha*e*e + bestGamma*indicator*e*e + bestBeta*variances[t-1]
	}
	currentVar := variances[len(variances)-1]
	
	// Forecast 1-step ahead
	eT := returns[len(returns)-1] - mu
	indT := 0.0
	if eT < 0 {
		indT = 1.0
	}
	forecastVar1 := omega + bestAlpha*eT*eT + bestGamma*indT*eT*eT + bestBeta*currentVar
	
	asymLabel := "NONE"
	switch {
	case bestGamma > 0.10:
		asymLabel = "HIGH"
	case bestGamma > 0.05:
		asymLabel = "MODERATE"
	case bestGamma > 0.01:
		asymLabel = "LOW"
	}
	
	asymRatio := 0.0
	if 2*bestAlpha+bestGamma > 0 {
		asymRatio = bestGamma / (2*bestAlpha + bestGamma) * 100
	}
	
	return &GJRGARCHResult{
		Omega: omega, Alpha: bestAlpha, Gamma: bestGamma, Beta: bestBeta,
		Persistence: persistence, AsymmetryRatio: asymRatio,
		CurrentVol: math.Sqrt(math.Max(currentVar, 1e-12)),
		ForecastVol1: math.Sqrt(math.Max(forecastVar1, 1e-12)),
		AsymmetryLabel: asymLabel,
		LeverageEffect: bestGamma > 0.05,
		Converged: true,
		SampleSize: len(returns),
	}, nil
}

func gjrLogLikelihood(returns []float64, omega, alpha, gamma, beta float64) float64 {
	mu := meanSlice(returns)
	varTarget := 0.0
	for _, r := range returns {
		varTarget += (r - mu) * (r - mu)
	}
	varTarget /= float64(len(returns))
	
	v := varTarget
	ll := 0.0
	for _, r := range returns {
		if v <= 0 {
			v = 1e-10
		}
		e := r - mu
		ind := 0.0
		if e < 0 {
			ind = 1.0
		}
		ll += -0.5 * (math.Log(2*math.Pi) + math.Log(v) + e*e/v)
		v = omega + alpha*e*e + gamma*ind*e*e + beta*v
	}
	return ll
}
```

### 3. Tambah `GJRGARCHResult` ke `QuantContext` di `domain/`

Cari struct `QuantAnalysis` atau `QuantContext` yang dipakai `/quant`. Tambah:
```go
GJRGARCHResult *price.GJRGARCHResult `json:"gjr_garch,omitempty"`
```

### 4. Surface di formatter `/quant`

Di `formatter_quant.go`, di seksi GARCH, tambah GJR-GARCH section:
```
📊 GJR-GARCH(1,1) — Asymmetric Vol
   α=0.08 γ=0.09 β=0.87 | Persist=0.985
   Asymmetry: MODERATE (55% dari shock berasal dari downside)
   → Downside risk elevated — kurangi long size
```

## File yang Harus Diubah

1. `internal/service/price/garch.go` — tambah `GJRGARCHResult`, `EstimateGJRGARCH()`, helpers
2. `internal/service/price/aggregator.go` — panggil `EstimateGJRGARCH()` dan simpan ke context
3. `internal/adapter/telegram/formatter_quant.go` — tambah GJR-GARCH section di /quant output

## Verifikasi

```bash
go build ./...
go test ./internal/service/price/...
# Manual: /quant EURUSD → cek ada "GJR-GARCH" section
# Pastikan asymmetry label tampil jika γ > 0.05
```

## Acceptance Criteria

- [ ] `EstimateGJRGARCH()` pure Go, no external deps
- [ ] Constraints terpenuhi: α≥0, γ≥0, β≥0, α+γ/2+β<1
- [ ] `AsymmetryLabel` dan `LeverageEffect` populated dengan benar
- [ ] Surface di `/quant` output dengan interpretasi Indonesia
- [ ] `go build ./...` + `go test ./internal/service/price/...` clean

## Referensi

- `.agents/research/2026-04-02-27-feature-index-gaps-carry-gjrgarch-oi4w-hmm4-vix-putaran19.md` — GAP 2
- `internal/service/price/garch.go` — GARCH(1,1) sebagai template
- Glosten, Jagannathan & Runkle (1993) — original GJR-GARCH paper
- FEATURE_INDEX.md → "Sudah ada, bisa didalami: GARCH → EGARCH, GJR-GARCH"
