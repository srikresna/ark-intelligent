# TASK-135: Volatility Cone Analysis

**Priority:** high
**Type:** feature
**Estimated:** M
**Area:** internal/service/price
**Created by:** Research Agent
**Created at:** 2026-04-02 03:00 WIB
**Siklus:** Fitur

## Deskripsi
Build "volatility cone" yang menunjukkan apakah current volatility historically high atau low untuk periode kalender ini. Gunakan GARCH forecast + historical IV snapshots untuk compute percentile bands (5th/25th/50th/75th/95th). Alert saat vol anomali.

## Konteks
- GARCH(1,1) sudah ada di `service/price/garch.go` — volatility forecasting
- ATR volatility regime sudah ada di `service/price/volatility.go`
- Deribit IV data bisa di-fetch via existing client
- `CombineVolatilityWithGARCH()` sudah blend ATR + VIX + GARCH
- Missing: IV percentile bands per calendar period, seasonality heatmap
- Ref: `.agents/research/2026-04-02-03-fitur-volcom-carry-microstructure-regime-alert.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat `internal/service/price/vol_cone.go`
- [ ] Compute rolling realized volatility (20d, 60d, 120d windows)
- [ ] Build percentile distribution per calendar month dari historical data (min 2 tahun)
- [ ] Type `VolCone`: CurrentVol, Percentile (0-100), Bands (P5, P25, P50, P75, P95), ZScore, IsAnomaly
- [ ] Alert condition: vol > P95 ("Volatility unusually high") atau vol < P5 ("Volatility unusually low")
- [ ] Telegram: extend `/quant` output dengan "Volatility Cone" section
- [ ] Formatter: visual cone representation (e.g., bar menunjukkan posisi current vs bands)
- [ ] Cache percentile bands (TTL 24h — hanya berubah saat data baru masuk)

## File yang Kemungkinan Diubah
- `internal/service/price/vol_cone.go` (baru)
- `internal/adapter/telegram/handler_quant.go` (extend output)
- `internal/adapter/telegram/formatter_quant.go` (vol cone formatter)

## Referensi
- `.agents/research/2026-04-02-03-fitur-volcom-carry-microstructure-regime-alert.md`
- `internal/service/price/garch.go`
- `internal/service/price/volatility.go`
