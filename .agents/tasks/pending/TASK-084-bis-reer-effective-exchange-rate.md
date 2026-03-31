# TASK-084: BIS Real Effective Exchange Rate (REER) Integration

**Priority:** LOW-MEDIUM
**Siklus:** 2 (Data & Integrasi Gratis)
**Estimasi:** 4-5 jam
**Area:** internal/service/fred/ (extend)

---

## Latar Belakang

Bank for International Settlements (BIS) menyediakan Real Effective Exchange Rate
(REER) dan Nominal Effective Exchange Rate (NEER) untuk semua major currencies
secara gratis melalui BIS Statistics API v1:

URL: https://stats.bis.org/api/v1/data/WS_EER/{frequency}/{currency}/{series}

REER mengukur nilai mata uang relatif terhadap basket mata uang mitra dagang,
disesuaikan dengan inflasi relatif. Ini lebih akurat dari nominal pair untuk
menilai apakah suatu mata uang "overvalued" atau "undervalued" secara fundamental.

Contoh: USD REER 120 = USD 20% above long-run average vs trading partners.

---

## Tujuan

Integrasi BIS REER monthly data untuk major currencies ke /macro command
sebagai fundamental valuation layer.

---

## Implementasi

### 1. Buat `internal/service/fred/bis.go`

```go
package fred

// BISData holds BIS effective exchange rate data.
type BISData struct {
    Currencies map[string]*BISCurrencyData
    Available  bool
    FetchedAt  time.Time
}

type BISCurrencyData struct {
    Currency    string  // "USD", "EUR", etc.
    REER        float64 // Real EER index (100 = base period 2020)
    NEER        float64 // Nominal EER index
    REERChange1M float64 // 1-month change (%)
    REERChange1Y float64 // 12-month change (%)
    Valuation   string  // "OVERVALUED", "UNDERVALUED", "FAIR"
    Date        time.Time
}

func FetchBISEffectiveRates(ctx context.Context) *BISData
```

### 2. BIS API Endpoints

```go
// BIS Statistics API v1 — Real Effective Exchange Rate (Broad)
// Monthly frequency, broad basket (61 trading partners)
const bisREERTemplate = "https://stats.bis.org/api/v1/data/WS_EER/M/{currency}/R/B"

// Currencies to fetch: USD, EUR, GBP, JPY, AUD, CAD, CHF
var bisCurrencies = []string{"USD", "EUR", "GBP", "JPY", "AUD", "CAD", "CHF"}
```

### 3. Valuation Classification

```go
// Classify based on deviation from 100 (base period)
func classifyREERValuation(reer float64) string {
    switch {
    case reer > 115:
        return "SIGNIFICANTLY_OVERVALUED"
    case reer > 105:
        return "OVERVALUED"
    case reer < 85:
        return "SIGNIFICANTLY_UNDERVALUED"
    case reer < 95:
        return "UNDERVALUED"
    default:
        return "FAIR_VALUE"
    }
}
```

### 4. Response Parsing

BIS API returns SDMX JSON format — parse "observations" array:
```json
{
  "data": {
    "dataSets": [{
      "series": {
        "0:0:0:0:0": {
          "observations": {
            "0": [120.45],
            "1": [119.82]
          }
        }
      }
    }]
  }
}
```

### 5. Caching

Monthly data — cache 24 jam. Store di BadgerDB.

### 6. Display di /macro

```
📊 REAL EFFECTIVE EXCHANGE RATES (BIS, monthly)
USD: REER 109.2 → 🔴 OVERVALUED (+9.2% vs long-run avg)
EUR: REER  97.4 → 🟡 SLIGHTLY UNDERVALUED (-2.6%)
GBP: REER  94.1 → 🟢 UNDERVALUED (-5.9%)
JPY: REER  77.3 → 🟢 HISTORICALLY CHEAP (-22.7%)
```

---

## Edge Cases

- BIS API rate limits: tambah polite delay, cache aggressively
- SDMX format complex: gunakan dedicated parser function
- Historical baselines vary — BIS uses 2020=100

---

## Testing

- Unit test: TestBISSDMXParsing (mock SDMX response)
- Unit test: TestREERValuationClassification
- Integration test: FetchBISEffectiveRates returns valid REER for USD/EUR/JPY

---

## File yang Dimodifikasi

- `internal/service/fred/bis.go` (NEW)
- `internal/adapter/telegram/handler_macro.go` (tambah REER section)
- `internal/service/ai/unified_outlook.go` (REER context ke AI prompt)

---

## Referensi

- BIS Statistics API: https://stats.bis.org/api/v1/
- BIS EER methodology: https://www.bis.org/statistics/eer.htm
- Research: `.agents/research/2026-04-01-10-data-integrasi-gratis.md`
