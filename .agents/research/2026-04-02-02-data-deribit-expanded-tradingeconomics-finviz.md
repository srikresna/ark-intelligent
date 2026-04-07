# Research Report: Data Siklus 2 Putaran 3 — Deribit Expanded, TradingEconomics, Finviz

**Tanggal:** 2026-04-02 02:00 WIB
**Fokus:** Data & Integrasi Baru Gratis (Siklus 2, Putaran 3)
**Siklus:** 2/5

---

## Ringkasan

Riset data sources yang enhance fitur baru (Wyckoff, GEX, Sentiment). Fokus pada: (1) Expand Deribit API yang sudah terintegrasi, (2) Scrape macro data via Firecrawl, (3) Cross-asset context data. 7 sumber dievaluasi, 5 dipilih untuk task.

---

## Temuan Utama

### 1. Deribit Public API — FAR MORE DATA AVAILABLE (VERY HIGH VALUE)

Bot sudah pakai Deribit untuk GEX (TASK-012). Tapi Deribit punya banyak endpoint publik yang BELUM dimanfaatkan:

| Endpoint | Data | Status |
|---|---|---|
| `get_book_summary_by_currency` | Mark IV, OI, volume untuk SEMUA options (880+ BTC, 746 ETH) | ✅ Verified |
| `get_volatility_index_data` | DVOL candles (crypto VIX equivalent) - 1s to 1D | ✅ Verified |
| `get_historical_volatility` | Realized vol historical (384 data points) | ✅ Verified |
| `get_funding_rate_history` | Perpetual funding rates | ✅ Verified |
| `get_instruments` | Full option chain enumeration | ✅ Verified |

**Expanded asset coverage:** USDC-settled options cover **SOL, AVAX, XRP, TRX** (2,296 options) — tidak hanya BTC/ETH!

**IV Surface Construction:** Call `get_book_summary_by_currency` → group by expiry for term structure, group by strike for skew. Satu API call = semua data.

### 2. TradingEconomics.com — Firecrawl Scraping Works (HIGH VALUE)

- Calendar page: 100+ events dengan country, impact level, actual/forecast/previous
- Individual indicators: GDP, CPI, employment, PMI per country
- URL pattern: `tradingeconomics.com/{country}/{indicator}`
- Free API: `api.tradingeconomics.com/calendar?c=guest:guest` — limited/old data
- **Firecrawl approach lebih baik** — current data, structured

### 3. Finviz.com — Clean Scrapeable Data (HIGH VALUE)

| Page | Data |
|---|---|
| `/futures.ashx` | Futures: indices, energy, metals, grains, currencies, bonds — price + change |
| `/forex.ashx` | Major FX pairs — %, change, high, low |
| `/groups.ashx?g=sector&v=140` | 11 sectors: week/month/quarter/year performance |
| `/forex_performance.ashx` | FX pair performance multi-timeframe |

Clean markdown tables, easily parseable. Delayed ~15min.

### 4. ForexFactory Calendar JSON (MEDIUM VALUE)

Endpoint: `https://nfs.faireconomy.media/ff_calendar_thisweek.json`
- 104 events/week, no auth needed
- Fields: date, country, title, impact, forecast, previous, actual
- Also: `ff_calendar_nextweek.json`

### 5. Yahoo Finance Unofficial API (MEDIUM VALUE, BACKUP)

`https://query1.finance.yahoo.com/v8/finance/chart/{SYMBOL}?interval=1d&range=1mo`
- Works for FX (`EURUSD=X`), commodities (`GC=F`), indices (`^VIX`)
- No key needed, requires User-Agent header
- Unofficial — may break

### Sumber TIDAK Viable

| Source | Alasan |
|---|---|
| CME QuikStrike | JS + reCAPTCHA, tidak scrapeable |
| Barchart.com | Data tables JS-rendered, Premier needed |
| OpenBB | Framework bukan data source |

---

## Task Recommendations

1. **TASK-130**: Deribit IV surface + skew + term structure [HIGH]
2. **TASK-131**: Deribit DVOL index (crypto VIX) [HIGH]
3. **TASK-132**: Deribit expanded assets (SOL, AVAX, XRP options) [MEDIUM]
4. **TASK-133**: TradingEconomics macro scraper via Firecrawl [MEDIUM]
5. **TASK-134**: Finviz cross-asset sentiment scraper [MEDIUM]
