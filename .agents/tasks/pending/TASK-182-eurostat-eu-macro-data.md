# TASK-182: Eurostat API — EU Macro Data for EUR Trading

**Priority:** high
**Type:** data
**Estimated:** M
**Area:** internal/service/eurostat/ (new)

## Deskripsi

Integrasi Eurostat API untuk EU-specific macro data. FRED punya beberapa EU proxies tapi Eurostat adalah primary source. Critical untuk EUR pair trading.

## Endpoints

- HICP Inflation: `GET /data/prc_hicp_manr?geo=EA20&coicop=CP00&format=JSON`
- Unemployment: `GET /data/une_rt_m?geo=EA20&s_adj=SA&format=JSON`
- GDP Growth: `GET /data/namq_10_gdp?geo=EA20&unit=CLV_PCH_PRE&format=JSON`

Base: `https://ec.europa.eu/eurostat/api/dissemination/statistics/1.0`
Auth: None.

## File Changes

- `internal/service/eurostat/client.go` — NEW: Eurostat API client
- `internal/service/eurostat/parser.go` — NEW: JSON-stat response parser (Eurostat uses custom JSON-stat format)
- `internal/service/eurostat/models.go` — NEW: EUInflation, EUUnemployment, EUGDP types
- `internal/adapter/telegram/formatter.go` — Add EU macro section to /macro output

## Acceptance Criteria

- [ ] Fetch EU HICP inflation (headline + core) monthly
- [ ] Fetch EU unemployment rate monthly
- [ ] Fetch EU GDP growth quarterly
- [ ] Compare EU vs US metrics for rate differential context
- [ ] Display in /macro output under "EU Economy" section
- [ ] JSON-stat format parser robust (Eurostat-specific format)
- [ ] Cache with 24h TTL (monthly/quarterly data)
- [ ] Graceful handling of Eurostat API downtime
