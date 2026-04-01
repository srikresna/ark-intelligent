# TASK-059: TGA Balance via FRED (WDTGAL) — Liquidity Dashboard

**Priority:** 🟡 MEDIUM  
**Cycle:** Siklus 2 — Data & Integrasi  
**Effort:** ~2 jam  
**Dependency:** TASK-055 (FRED_API_KEY harus dikonfigurasi dulu)  
**Assignee:** Dev-B

---

## Context

Treasury General Account (TGA) balance adalah salah satu leading indicator terpenting untuk USD liquidity. 

- TGA **naik** = Treasury drain likuiditas dari sistem → risk-off, USD kuat
- TGA **turun** (drawdown) = Treasury inject likuiditas → risk-on, USD lemah, equities/crypto naik

FRED menyediakan series `WDTGAL` — TGA balance weekly, gratis dengan FRED API key.

Bot saat ini track FRED balance sheet (`WALCL`) dan reverse repo (`RRPONTSYD`) tapi **tidak track TGA**. TGA + reverse repo + Fed balance sheet = Trinity of liquidity monitoring.

---

## Acceptance Criteria

### 1. Tambah TGA ke MacroData

Di `internal/service/fred/fetcher.go`:
```go
// Treasury General Account
TGABalance      float64     // WDTGAL — TGA balance (billions USD)
TGABalanceTrend SeriesTrend // trend: rising (drain) or falling (inject)
```

### 2. Tambah ke Fetch Batch

Di job list (fetcher.go, batch fetch section):
```go
{"WDTGAL", 8}, // 8 weeks TGA history
```

### 3. Parse di FetchAll()

```go
if obs := obsMap["WDTGAL"]; len(obs) >= 2 {
    data.TGABalance = obs[0]
    data.TGABalanceTrend = computeTrend(obs[0], obs[1], 50) // $50B threshold
}
```

### 4. Tambah ke Persistence

Di `internal/service/fred/persistence.go`:
```go
addObs("WDTGAL", data.TGABalance)
```

### 5. Display di /macro Output

Di formatter (`FormatMacro` atau equivalent):
```
💰 TGA Balance: $623B → Stable
   (TGA drawdown injects liquidity: risk-on signal)
```

### 6. Liquidity Composite

Tambah ke regime assessment: kombinasi TGA + RRP + Fed Balance Sheet untuk classify liquidity regime:
```
Liquidity: TIGHT (RRP high + TGA rising)
Liquidity: EASY (TGA falling + Fed QE)
```

### 7. AI Context

Di `internal/service/ai/unified_outlook.go`:
```go
if data.MacroData.TGABalance > 0 {
    b.WriteString(fmt.Sprintf("TGA Balance: $%.0fB (%s) — %s\n", 
        data.MacroData.TGABalance, data.MacroData.TGABalanceTrend.Direction,
        classifyTGALiquidity(data.MacroData)))
}
```

---

## FRED Series Spec

```
Series ID: WDTGAL
Title: U.S. Treasury, Operating Cash Balance, Total, Close of Business
Frequency: Weekly (Wednesday)
Units: Billions of Dollars
Source: U.S. Department of Treasury, Daily Treasury Statement

Value example: 623.456 = $623B
```

---

## Files to Edit

- `internal/service/fred/fetcher.go` — tambah field + batch fetch + parse
- `internal/service/fred/persistence.go` — tambah addObs
- `internal/service/fred/regime.go` — tambah liquidity composite
- `internal/adapter/telegram/formatter.go` — display TGA di /macro
- `internal/service/ai/unified_outlook.go` — inject TGA context

---

## Notes

- **Dependency:** TASK-055 harus selesai dulu (FRED_API_KEY diperlukan)
- WDTGAL update weekly (Rabu) — tidak perlu polling harian
- Unit sudah dalam Billions — tidak perlu konversi
- Threshold: TGA < 300B = low liquidity risk (debt ceiling concern)
- TGA > 800B = Treasury building war chest = potential liquidity withdrawal risk
