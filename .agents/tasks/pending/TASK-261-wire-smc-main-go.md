# TASK-261: Wire /smc Command di cmd/bot/main.go

**Priority:** high
**Type:** feature-wiring
**Estimated:** XS
**Area:** cmd/bot/main.go
**Created by:** Research Agent
**Created at:** 2026-04-02 10:00 WIB

## Deskripsi

Handler `/smc` (`internal/adapter/telegram/handler_smc.go`) dan TA SMC engine (`internal/service/ta/smc.go`) sudah **sepenuhnya diimplementasi** tapi **tidak disambungkan** di `cmd/bot/main.go`. User tidak bisa mengakses `/smc` — command tidak terdaftar.

`SMCServices` struct dan `WithSMC()` sudah ada, sudah ditest, tapi main.go tidak pernah memanggil `handler.WithSMC()`.

## Root Cause

`cmd/bot/main.go` di blok "Wire services" tidak memanggil `handler.WithSMC()`.

## File yang Harus Diubah

1. `cmd/bot/main.go` — tambah blok wiring SMC

## Implementasi

`taEngine` sudah dibuat di baris 355 (`taEngine := ta.NewEngine()`). Gunakan kembali.

Tambahkan setelah blok WithICT (baris ~398):

```go
// Wire SMC services (Smart Money Concepts: BOS/CHOCH + ICT overlay)
smcServices := &tgbot.SMCServices{
    ICTEngine:      ictsvc.NewEngine(),
    TAEngine:       taEngine,
    DailyPriceRepo: dailyPriceRepo,
    IntradayRepo:   intradayRepo,
}
handler.WithSMC(smcServices)
log.Info().Msg("SMC commands registered (/smc)")
```

`ictsvc` sudah diimport (dipakai di WithICT). Tidak perlu import tambahan.

## Acceptance Criteria

- [ ] `/smc EURUSD` merespons (bukan "command not found")
- [ ] `/smc BTCUSD H4` menghasilkan SMC dashboard (BOS/CHOCH + FVG + Order Blocks)
- [ ] `go build ./...` clean
- [ ] `log.Info().Msg("SMC commands registered (/smc)")` muncul saat bot start

## Referensi

- `.agents/research/2026-04-02-10-feature-index-wiring-gaps-unified-outlook-putaran14.md` — Temuan #2
- `cmd/bot/main.go:391` — pola WithICT (template untuk WithSMC)
- `internal/adapter/telegram/handler_smc.go:87` — WithSMC() signature
- `internal/adapter/telegram/handler_smc.go:30` — SMCServices struct fields
- `cmd/bot/main.go:355` — taEngine := ta.NewEngine() — sudah ada, reuse
