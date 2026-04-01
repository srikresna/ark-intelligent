# TASK-267: Split handler.go — Ekstrak Macro Handlers ke handler_macro.go

**Priority:** medium
**Type:** refactor
**Estimated:** S
**Area:** internal/adapter/telegram/handler.go → handler_macro.go
**Created by:** Research Agent
**Created at:** 2026-04-02 12:00 WIB

## Deskripsi

`handler.go` saat ini 2,909 LOC (TECH-002). Sudah banyak split (13 handler_*.go files),
tapi semua macro-related handlers masih dalam handler.go (~253 LOC, 7 fungsi).

Pindahkan ke `handler_macro.go`. **Zero behavior change** — pure file split.

## Fungsi yang Dipindah (dari handler.go → handler_macro.go)

| Fungsi | Baris (approx) |
|--------|---------------|
| cmdMacro | L2039 |
| macroSendSummary | L2113 |
| macroSendDetail | L2130 |
| cbMacro | L2146 |
| buildRegimeAssetInsight | L2212 |
| macroRegimePerformance | L2242 |
| currentMacroRegimeName | L2275 |

**Total estimasi dipindah:** ~253 LOC
**handler.go setelah split:** ~2,656 LOC

## Implementasi

### 1. Buat handler_macro.go

```go
package telegram

// handler_macro.go — /macro command and FRED regime handlers.

import (
    "context"
    "fmt"
    "strings"

    fred "github.com/arkcode369/ark-intelligent/internal/service/fred"
    // ... import lain sesuai kebutuhan fungsi yang dipindah
)
```

Pindahkan 7 fungsi ke file baru ini.

### 2. Hapus dari handler.go

Hapus 7 fungsi tersebut dari handler.go. Bersihkan import yang tidak lagi digunakan di handler.go.

### 3. Pastikan callback registration tetap di handler.go

Di `handler.go` RegisterCallbacks section, pastikan `cbMacro` masih terdaftar dengan benar
(sudah berada di file yang sama package — tidak ada perubahan yang diperlukan karena masih satu package).

### 4. Verifikasi

```bash
go build ./...
go vet ./...
```

## Acceptance Criteria

- [ ] `handler_macro.go` baru berisi 7 fungsi macro handlers
- [ ] Fungsi-fungsi tersebut dihapus dari `handler.go`
- [ ] `handler.go` setelah split ≤ 2,700 LOC
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `/macro` command tetap berfungsi (zero behavior change)
- [ ] Callback `macro:` tetap terdaftar dan berfungsi

## Referensi

- `.agents/research/2026-04-02-12-tech-refactor-formatter-handler-splits-putaran15.md`
- `.agents/TECH_REFACTOR_PLAN.md` — TECH-002
- `internal/adapter/telegram/handler_alpha.go` — contoh pola handler split yang baik
- `internal/adapter/telegram/handler.go:2039` — titik mulai fungsi macro
- `internal/adapter/telegram/handler.go:192` — RegisterCommands (tetap di handler.go)
