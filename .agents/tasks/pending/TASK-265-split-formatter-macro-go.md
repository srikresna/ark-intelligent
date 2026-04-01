# TASK-265: Split formatter.go — Ekstrak Macro/FRED/Regime ke formatter_macro.go

**Priority:** high
**Type:** refactor
**Estimated:** M
**Area:** internal/adapter/telegram/formatter.go → formatter_macro.go
**Created by:** Research Agent
**Created at:** 2026-04-02 12:00 WIB

## Deskripsi

`formatter.go` saat ini 4,539 LOC (TECH-001 dari TECH_REFACTOR_PLAN.md). Sudah ada partial split
(formatter_ict, formatter_gex, formatter_wyckoff, formatter_quant, formatter_compact), tapi 12 fungsi
Macro/FRED/Regime masih ada di formatter.go dan berkontribusi ~1,003 LOC.

Task ini mengekstrak semua fungsi Macro/FRED/Regime ke file baru `formatter_macro.go`.
**Zero behavior change** — pure file split, tidak ada logic yang diubah.

## Fungsi yang Dipindah (dari formatter.go → formatter_macro.go)

| Fungsi | Baris (approx) |
|--------|---------------|
| FormatMacroRegime | L1682 |
| FormatRegimeAssetInsight | L1763 |
| FormatFREDContext | L1800 |
| FormatMacroComposites | L1866 |
| FormatMacroGlobal | L1997 |
| FormatMacroLabor | L2097 |
| FormatMacroInflation | L2197 |
| FormatMacroSummary | L2304 |
| FormatMacroExplain | L2491 |
| FormatRegimeLabel | L2592 |
| FormatRegimePerformance | L3284 |

**Total estimasi dipindah:** ~1,003 LOC
**formatter.go setelah split:** ~3,536 LOC (turun ~22%)

## Implementasi

### 1. Buat formatter_macro.go

```go
package telegram

// formatter_macro.go — Macro/FRED/Regime formatting for Telegram HTML messages.

import (
    "fmt"
    "math"
    "sort"
    "strings"
    "time"

    "github.com/arkcode369/ark-intelligent/internal/domain"
    "github.com/arkcode369/ark-intelligent/internal/service/fred"
    "github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)
```

Pindahkan semua 11 fungsi di atas ke file ini. Import hanya yang benar-benar digunakan oleh fungsi yang dipindah.

### 2. Hapus fungsi dari formatter.go

Hapus semua 11 fungsi dari formatter.go. Hapus import yang tidak lagi digunakan setelah penghapusan
(jika ada import yang hanya dipakai oleh fungsi-fungsi ini).

### 3. Verifikasi

```bash
go build ./...
go vet ./...
```

## Acceptance Criteria

- [ ] `formatter_macro.go` baru berisi semua 11 fungsi FormatMacro*/FormatFRED*/FormatRegime*
- [ ] Fungsi-fungsi tersebut dihapus dari `formatter.go`
- [ ] `formatter.go` setelah split ≤ 3,600 LOC
- [ ] `go build ./...` clean — zero compile errors
- [ ] `go vet ./...` clean — zero warnings
- [ ] Semua test yang ada pass (bila ada yang terkait)
- [ ] Zero behavior change — tidak ada logic yang diubah, hanya pindah file

## Referensi

- `.agents/research/2026-04-02-12-tech-refactor-formatter-handler-splits-putaran15.md`
- `.agents/TECH_REFACTOR_PLAN.md` — TECH-001
- `internal/adapter/telegram/formatter_ict.go` — contoh pola file split yang benar
- `internal/adapter/telegram/formatter.go:1682` — titik mulai fungsi Macro
