# TASK-082: World Bank API — Global Macro Fundamentals

**Priority:** MEDIUM
**Siklus:** 2 (Data & Integrasi Gratis)
**Estimasi:** 4-6 jam
**Area:** internal/service/fred/ (extend) atau internal/service/macro/ (new)

---

## Latar Belakang

World Bank API menyediakan data makro global secara gratis tanpa API key:
- GDP growth rates per country (annual)
- Current Account Balance (% of GDP)
- Inflation (CPI YoY)
- Foreign Exchange Reserves
- Trade balance

Data ini sangat relevan untuk:
- Fundamental analysis pairs commodity currencies (AUD, NZD, CAD)
- EM currencies context (TRY, BRL, ZAR, MXN)
- /macro command enrichment dengan data global

Base URL: https://api.worldbank.org/v2/country/{code}/indicator/{indicator}?format=json

---

## Tujuan

Integrasikan World Bank data untuk 8 currency blocs (USD, EUR, GBP, JPY, AUD,
NZD, CAD, CHF) ke macro analysis layer.

---

## Implementasi

### 1. Buat `internal/service/fred/worldbank.go`

```go
package fred

// WorldBankData holds macro fundamentals from World Bank API.
type WorldBankData struct {
    // Per currency bloc
    Countries map[string]*CountryMacro // key: ISO country code
    Available bool
    FetchedAt time.Time
}

// CountryMacro holds annual macro fundamentals for one country.
type CountryMacro struct {
    Country      string  // "Australia"
    Currency     string  // "AUD"
    GDPGrowthYoY float64 // Real GDP growth (%)
    CurrentAccount float64 // CA balance (% of GDP)
    InflationCPI float64  // CPI YoY (%)
    FXReserves   float64  // Total reserves (USD billions)
    Year         int      // Data vintage year
}

func FetchWorldBankMacro(ctx context.Context) *WorldBankData
```

### 2. Indicator Mapping

```go
var wbIndicators = map[string]string{
    "NY.GDP.MKTP.KD.ZG": "gdp_growth",    // Real GDP growth
    "BN.CAB.XOKA.GD.ZS": "current_acct",  // Current account (% GDP)
    "FP.CPI.TOTL.ZG":    "inflation",      // CPI inflation
    "FI.RES.TOTL.CD":    "fx_reserves",    // Total reserves
}
```

### 3. Country Mapping ke Currency

```go
var currencyCountry = map[string]string{
    "EUR": "EMU",  // Euro area
    "GBP": "GBR",  // UK
    "JPY": "JPN",  // Japan
    "AUD": "AUS",  // Australia
    "NZD": "NZL",  // New Zealand
    "CAD": "CAN",  // Canada
    "CHF": "CHE",  // Switzerland
    "USD": "USA",  // United States
}
```

### 4. Caching

Data annual → cache 7 hari. World Bank updates annually so daily refresh unnecessary.
Store di BadgerDB dengan key "wb:macro:{year}:{country}".

### 5. Integrasi ke /macro command

Tambah section di macro formatter:
```
🌍 GLOBAL FUNDAMENTALS (World Bank, latest annual)
AUD: GDP +2.1% | CA -1.8% GDP | CPI +3.5%
NZD: GDP +1.4% | CA -6.2% GDP | CPI +3.3%
CAD: GDP +1.2% | CA -0.5% GDP | CPI +2.6%
```

### 6. Integrasi ke AI prompt

Sertakan ke unified_outlook.go sebagai additional macro context.

---

## Edge Cases

- Data vintage: World Bank sering tertinggal 1-2 tahun, gunakan data terbaru available
- Missing data: beberapa small countries mungkin tidak punya semua indikator
- Rate limiting: tidak ada (public API), tapi tambah polite 500ms delay antar request

---

## Testing

- Unit test: TestWorldBankParsing (mock response JSON)
- Integration test: FetchWorldBankMacro returns valid data for AUD/NZD/CAD
- Edge case: graceful handling of missing/null data points

---

## File yang Dimodifikasi

- `internal/service/fred/worldbank.go` (NEW)
- `internal/adapter/telegram/handler_macro.go` (tambah WB section)
- `internal/service/ai/unified_outlook.go` (context enrichment)

---

## Referensi

- World Bank API docs: https://datahelpdesk.worldbank.org/knowledgebase/articles/898581
- DATA_SOURCES_AUDIT.md — "World Bank API" section
- Research: `.agents/research/2026-04-01-10-data-integrasi-gratis.md`
