# TASK-009: World Bank API Cross-Country Macro Integration

**Priority:** medium
**Type:** data
**Estimated:** L
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-01 00:00 WIB
**Siklus:** Data

## Deskripsi
Integrasikan World Bank API (`api.worldbank.org/v2/`) untuk mengambil data macro cross-country: GDP growth rate, current account balance, dan inflasi CPI untuk negara-negara major currency (US, Eurozone, UK, Japan, Australia, Canada, New Zealand, Switzerland). Data ini gratis, tanpa API key, dan sangat relevan untuk analisis fundamental forex.

## Konteks
Saat ini sistem punya data macro US yang sangat lengkap via FRED, tapi hampir tidak ada data macro dari negara lain. Untuk analisis EUR/USD, GBP/USD, AUD/USD dll, differential antara dua negara adalah driver fundamental:
- **GDP growth differential**: negara dengan GDP growth lebih tinggi → currency menguat jangka panjang
- **Current Account**: surplus = structural demand untuk currency; deficit = structural supply
- **CPI differential**: negara dengan inflasi lebih tinggi → currency cenderung melemah (PPP)

World Bank API:
- Base URL: `https://api.worldbank.org/v2/country/{country_code}/indicator/{series_id}?format=json&mrv=3`
- No API key required
- Rate limits: sangat longgar (government data API)
- Update frequency: tahunan (annual) — cukup di-cache per bulan

Series yang relevan:
- `NY.GDP.MKTP.KD.ZG` — GDP Growth Rate (%)
- `BN.CAB.XOKA.CD` — Current Account Balance (USD)
- `FP.CPI.TOTL.ZG` — CPI Inflation (%)

Country codes: US, GB, JP, EU (XC di World Bank untuk Eurozone), AU, CA, NZ, CH

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Buat package `internal/service/worldbank/` dengan file `client.go`
- [ ] Struct `CountryMacro`: CountryCode string, Currency string, GDPGrowth float64, CurrentAccount float64, CPIInflation float64, Year int, Available bool
- [ ] Struct `WorldBankData`: Countries []CountryMacro, FetchedAt time.Time
- [ ] Fungsi `FetchWorldBankData(ctx context.Context) (*WorldBankData, error)` fetch parallel untuk semua countries
- [ ] In-memory cache dengan TTL 24 jam (data update annually, daily re-fetch cukup)
- [ ] `UnifiedOutlookData` di `unified_outlook.go` ditambah field `WorldBankData *worldbank.WorldBankData`
- [ ] `BuildUnifiedOutlookPrompt()` menampilkan cross-country macro comparison jika tersedia (section baru: CROSS-COUNTRY MACRO FUNDAMENTALS)
- [ ] Graceful degradation: jika World Bank API gagal, log warn + skip section (tidak error)
- [ ] Unit test untuk data parsing (`worldbank_test.go`) dengan mock response

## File yang Kemungkinan Diubah
- `internal/service/worldbank/client.go` (baru)
- `internal/service/worldbank/worldbank_test.go` (baru)
- `internal/service/ai/unified_outlook.go` (UnifiedOutlookData + BuildUnifiedOutlookPrompt)
- `internal/adapter/telegram/handler.go` (inject WorldBankData ke handler yang build outlook)

## Referensi
- `.agents/research/2026-04-01-00-data-integrasi-baru.md`
- `.agents/DATA_SOURCES_AUDIT.md` (section "KALAU BUTUH SUMBER BARU — GRATIS DULU": World Bank API)
- `internal/service/fred/fetcher.go` (referensi pola fetch + cache + struct MacroData)
- World Bank API docs: https://datahelpdesk.worldbank.org/knowledgebase/articles/898581
