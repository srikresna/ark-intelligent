# TASK-279: Tambah Warning Log + Callback Coverage Test untuk cbNav dan cbQuickCommand

**Priority:** medium
**Type:** reliability
**Estimated:** S
**Area:** internal/adapter/telegram/handler.go, internal/adapter/telegram/handler_test.go
**Created by:** Research Agent
**Created at:** 2026-04-02 25:00 WIB

## Deskripsi

`cbNav` dan `cbQuickCommand` keduanya menggunakan `default: return nil` — silent no-op ketika callback tidak dikenali. Ini menyebabkan bug seperti BUG-UX1 (nav:cot) dan BUG-UX2 (cmd:cta) tidak terdeteksi sampai user melaporkan.

Perbaikan dilakukan dalam dua langkah:

### 1. Tambah Warning Log di default case

Ketika callback tidak dikenali, setidaknya log warning:
```go
// cbNav default:
default:
    log.Warn().Str("action", action).Str("chat_id", chatID).Msg("unhandled nav action — check keyboard.go")
    return nil

// cbQuickCommand default:
default:
    log.Warn().Str("cmd", cmd).Str("chat_id", chatID).Msg("unhandled cmd: callback — check keyboard.go")
    return nil
```

### 2. Unit Test Coverage — enumerate keyboard callbacks vs handler

Buat test yang:
1. Collect semua `CallbackData` values dari `keyboard.go` (via test helper atau regex)
2. Verifikasi setiap `"nav:XYZ"` punya corresponding case di `cbNav`
3. Verifikasi setiap `"cmd:XYZ"` punya corresponding case di `cbQuickCommand`

Alternatif yang lebih pragmatis: buat daftar `knownNavActions` dan `knownCmdActions` sebagai konstanta/slice di file test, lalu assert tidak ada yang missing.

## File yang Harus Diubah

### internal/adapter/telegram/handler.go

Tambah `log.Warn()` di `default` case dari `cbNav` dan `cbQuickCommand`.

### internal/adapter/telegram/handler_callback_coverage_test.go (baru)

```go
package telegram_test

import (
    "testing"
)

// knownNavActions harus sesuai dengan semua nav: actions yang ada di keyboard.go
var knownNavActions = []string{
    "home",
    "cot",
    // tambahkan bila keyboard.go ditambah nav: baru
}

// knownCmdActions harus sesuai dengan semua cmd: prefixes yang ada di keyboard.go
var knownCmdActions = []string{
    "bias", "macro", "rank", "calendar", "accuracy", "sentiment",
    "seasonal", "backtest", "price", "levels", "quant", "vp", "cta",
    "corr", "carry", "intraday", "garch", "hurst", "regime", "factors", "wfopt",
}

func TestNavActionsDocumented(t *testing.T) {
    // This test serves as a living checklist.
    // Add new nav: actions here AND in cbNav when adding keyboard buttons.
    t.Logf("Known nav actions: %v", knownNavActions)
    if len(knownNavActions) == 0 {
        t.Fatal("knownNavActions must not be empty")
    }
}

func TestCmdActionsDocumented(t *testing.T) {
    t.Logf("Known cmd actions: %v", knownCmdActions)
    if len(knownCmdActions) == 0 {
        t.Fatal("knownCmdActions must not be empty")
    }
}
```

Catatan: test ini adalah "living documentation" — developer wajib update saat menambah keyboard button baru.

## Verifikasi

```bash
go build ./...
go test ./internal/adapter/telegram/...
```

## Acceptance Criteria

- [ ] `cbNav` default case log warning dengan action value
- [ ] `cbQuickCommand` default case log warning dengan cmd value
- [ ] File test baru berisi daftar nav actions dan cmd actions yang lengkap
- [ ] Test pass dengan `go test ./internal/adapter/telegram/...`
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-25-ux-audit-dead-callbacks-loading-patterns-putaran17.md` — ISSUE-UX5
- `internal/adapter/telegram/handler.go` — cbNav (~1641), cbQuickCommand (~1593)
- `internal/adapter/telegram/keyboard.go` — semua CallbackData values
