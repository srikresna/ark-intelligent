# TASK-034: IMF WEO Growth & Inflation Forecasts via DataMapper API

**Priority:** medium
**Type:** data
**Estimated:** M
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-01 15:xx WIB
**Siklus:** Data (Siklus 2 Putaran 2)

## Deskripsi
Integrasikan IMF World Economic Outlook (WEO) forecasts via IMF DataMapper API.
Data ini gratis, tanpa API key, dan memberikan FORWARD-LOOKING (forecast) GDP growth,
inflation, dan current account balance dari IMF — berbeda dari World Bank yang historical.

## Konteks
TASK-009 sudah membuat spec untuk World Bank API (historical data).
IMF DataMapper melengkapi ini dengan FORECASTS — lebih relevan untuk trading:

```
Saat ini: World Bank GDP 2024 actual = 2.8% (US) — sudah terjadi, kurang actionable
Lebih useful: IMF WEO 2026 forecast = 2.1% (US) vs 1.2% (Eurozone) → USD structural advantage
```

IMF DataMapper API:
- Base: `https://www.imf.org/external/datamapper/api/v1/{INDICATOR}/{COUNTRY_LIST}`
- No API key, no auth
- Response: JSON dengan forecasts per tahun (current + next 5 years)
- Update: 2x per tahun (April dan Oktober WEO release)

## IMF Indicators untuk Forex

| Code | Metric |
|------|--------|
| `NGDP_RPCH` | GDP Growth Rate (%) |
| `PCPIPCH` | CPI Inflation (%) |
| `BCA_NGDPDP` | Current Account Balance (% of GDP) |

Countries: `USA`, `GBR`, `JPN`, `DEU` (Germany/EUR proxy), `AUS`, `CAN`, `NZD`, `CHE`

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Buat package `internal/service/imf/` dengan file `weo.go`
- [ ] Struct `IMFCountryData`:
  ```go
  type IMFCountryData struct {
      Country        string  // "USA", "GBR", etc.
      Currency       string  // "USD", "GBP", etc.
      GDPGrowth2026  float64 // IMF forecast GDP growth %
      GDPGrowth2027  float64
      Inflation2026  float64 // IMF forecast CPI %
      CurrentAccount float64 // % of GDP
      Available      bool
  }
  ```
- [ ] Struct `IMFWEOData`:
  ```go
  type IMFWEOData struct {
      Countries []IMFCountryData
      FetchedAt time.Time
      Available bool
  }
  ```
- [ ] Fungsi `FetchIMFWEO(ctx context.Context) (*IMFWEOData, error)`
  - Fetch 3 indicators dalam parallel (NGDP_RPCH, PCPIPCH, BCA_NGDPDP)
  - Parse JSON response: `response[indicator][country][year]`
  - In-memory cache TTL 24 jam (data update 2x/tahun, harian sudah cukup)
- [ ] `UnifiedOutlookData` ditambah field `IMFData *imf.IMFWEOData`
- [ ] `BuildUnifiedOutlookPrompt()` menambahkan section "IMF WEO FORECASTS":
  ```
  IMF WEO 2026 Forecasts:
  USD: GDP=2.1% CPI=2.4% CA=-3.2%GDP | EUR: GDP=1.2% CPI=2.1% CA=+2.8%GDP
  GBP: GDP=1.0% CPI=2.8% CA=-3.1%GDP | JPY: GDP=1.1% CPI=2.2% CA=+3.4%GDP
  → Highest growth: USD | Lowest growth: GBP | Best CA surplus: JPY
  ```
- [ ] Graceful degradation: jika IMF API down, log warn + skip

## File yang Kemungkinan Diubah
- `internal/service/imf/weo.go` (baru)
- `internal/service/ai/unified_outlook.go` (UnifiedOutlookData + prompt section)
- `internal/adapter/telegram/handler.go` (inject IMFData)

## Referensi
- `.agents/research/2026-04-01-15-data-integrasi-siklus2-putaran2.md` (GAP 4)
- IMF DataMapper API: https://www.imf.org/external/datamapper/api/v1/
- IMF indicator list: https://www.imf.org/external/datamapper/api/v1/indicators
- `internal/service/worldbank/` (jika sudah ada dari TASK-009 — ikuti pola)
- `internal/service/fred/fetcher.go` (concurrent fetch pattern)
