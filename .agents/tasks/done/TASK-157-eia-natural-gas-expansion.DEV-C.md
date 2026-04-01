# TASK-157: EIA Natural Gas Storage & Henry Hub Price Integration

**Priority:** medium
**Type:** data
**Estimated:** S
**Area:** internal/service/price/

## Deskripsi

Expand existing EIA integration (`internal/service/price/eia.go`) untuk fetch natural gas data. Key sudah ada di .env, infrastructure sudah ada, tinggal tambah series.

## Detail Teknis

EIA v2 API routes:
- Storage: `GET /v2/natural-gas/stor/wkly` + facets
- Henry Hub: `GET /v2/natural-gas/pri/fut` + `facets[series][]=RNGWHHD`, frequency=daily
- Auth: Same EIA_API_KEY

## File Changes

- `internal/service/price/eia.go` — Add `FetchNaturalGas()` method with storage + price series
- `internal/service/price/models.go` — Add `NaturalGasData` struct (storage_bcf, henry_hub_price, week_change)
- `internal/adapter/telegram/formatter.go` — Add natural gas section to energy/commodity output
- `internal/domain/price.go` — Add NATGAS instrument mapping

## Acceptance Criteria

- [ ] Fetch weekly natural gas storage (BCF) + week-over-week change
- [ ] Fetch Henry Hub spot price (daily)
- [ ] Compute: storage vs 5-year average, injection/withdrawal season context
- [ ] Display in energy section alongside existing petroleum data
- [ ] Cache with 6h TTL (weekly release Thursdays 10:30 ET)
- [ ] Reuse existing EIA client infrastructure
