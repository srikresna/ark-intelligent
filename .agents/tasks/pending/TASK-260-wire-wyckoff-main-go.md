# TASK-260: Wire /wyckoff Command di cmd/bot/main.go

**Priority:** high
**Type:** feature-wiring
**Estimated:** XS
**Area:** cmd/bot/main.go
**Created by:** Research Agent
**Created at:** 2026-04-02 10:00 WIB

## Deskripsi

Service Wyckoff (`internal/service/wyckoff/`) dan handler `/wyckoff` (`internal/adapter/telegram/handler_wyckoff.go`) sudah **sepenuhnya diimplementasi** tapi **tidak pernah disambungkan** ke aplikasi di `cmd/bot/main.go`. Akibatnya user tidak bisa mengakses `/wyckoff` sama sekali — command tidak terdaftar.

Perbandingan: ICT, GEX, VP, CTA, Quant sudah terdaftar. Wyckoff terlewat.

## Root Cause

`cmd/bot/main.go` di blok "Wire services" (sekitar baris 385-410) tidak memanggil `handler.WithWyckoff()`.

## File yang Harus Diubah

1. `cmd/bot/main.go` — tambah blok wiring Wyckoff

## Implementasi

Tambahkan setelah blok WithGEX (baris ~405):

```go
// Wire Wyckoff services (Wyckoff Method structure detection)
wyckoffServices := tgbot.WyckoffServices{
    DailyPriceRepo: dailyPriceRepo,
    IntradayRepo:   intradayRepo,
    WyckoffEngine:  wyckoffsvc.NewEngine(),
}
handler.WithWyckoff(wyckoffServices)
log.Info().Msg("Wyckoff commands registered (/wyckoff)")
```

Tambah import di blok imports (sesuai pola ICT):
```go
wyckoffsvc "github.com/arkcode369/ark-intelligent/internal/service/wyckoff"
```

## Acceptance Criteria

- [ ] `/wyckoff EURUSD` merespons (bukan "command not found")
- [ ] `/wyckoff XAUUSD D1` menghasilkan Wyckoff analysis output
- [ ] `go build ./...` clean
- [ ] `log.Info().Msg("Wyckoff commands registered (/wyckoff)")` muncul saat bot start

## Referensi

- `.agents/research/2026-04-02-10-feature-index-wiring-gaps-unified-outlook-putaran14.md` — Temuan #1
- `cmd/bot/main.go:391` — pola WithICT (template untuk WithWyckoff)
- `internal/adapter/telegram/handler_wyckoff.go:33` — WithWyckoff() signature
- `internal/service/wyckoff/engine.go:19` — wyckoff.NewEngine()
