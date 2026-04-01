# TASK-266: Split formatter.go — Ekstrak COT/Ranking/Bias ke formatter_cot.go

**Priority:** high
**Type:** refactor
**Estimated:** M
**Area:** internal/adapter/telegram/formatter.go → formatter_cot.go
**Created by:** Research Agent
**Created at:** 2026-04-02 12:00 WIB

## Deskripsi

Kelanjutan dari TECH-001. Setelah TASK-265 (macro split), formatter.go masih ~3,536 LOC.
Langkah berikutnya: ekstrak 9 fungsi COT/Ranking/Bias yang berkontribusi ~1,343 LOC ke `formatter_cot.go`.

**Zero behavior change** — pure file split.

## Fungsi yang Dipindah (dari formatter.go → formatter_cot.go)

| Fungsi | Baris (approx) |
|--------|---------------|
| FormatCOTOverview | L328 |
| FormatCOTDetail | L442 |
| FormatCOTDetailWithCode | L447 |
| FormatCOTRaw | L619 |
| FormatRanking | L1108 |
| FormatRankingWithConviction | L1181 |
| FormatConvictionBlock | L1329 |
| FormatBiasHTML | L2622 |
| FormatBiasSummary | L2665 |

**Note:** `formatProgressBar` (L998) dan `momentumLabel` (L1021) adalah private helpers
yang digunakan oleh fungsi COT — harus ikut dipindah ke formatter_cot.go.

**Total estimasi dipindah:** ~1,343 LOC
**formatter.go setelah split (dari ~3,536):** ~2,193 LOC (turun total 52% dari awal 4,539)

## Implementasi

### 1. Buat formatter_cot.go

```go
package telegram

// formatter_cot.go — COT positioning, ranking, and bias formatting for Telegram HTML messages.

import (
    "fmt"
    "math"
    "sort"
    "strings"
    "time"

    "github.com/arkcode369/ark-intelligent/internal/domain"
    "github.com/arkcode369/ark-intelligent/internal/service/cot"
    "github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)
```

Pindahkan semua 9 fungsi public + 2 private helpers ke file ini.

### 2. Hapus dari formatter.go

Hapus semua fungsi tersebut dari formatter.go. Bersihkan import yang tidak lagi digunakan.

### 3. Verifikasi

```bash
go build ./...
go vet ./...
```

## Dependency Note

Sebaiknya dikerjakan **setelah TASK-265** selesai agar tidak ada conflict di formatter.go,
tapi secara teknis bisa dikerjakan paralel jika Dev agent mengambil section yang berbeda.
Kalau paralel, koordinasi dengan Dev yang mengerjakan TASK-265.

## Acceptance Criteria

- [ ] `formatter_cot.go` baru berisi 9 fungsi public + 2 private helpers (formatProgressBar, momentumLabel)
- [ ] Fungsi-fungsi tersebut dihapus dari `formatter.go`
- [ ] `formatter.go` setelah split ≤ 2,300 LOC
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] Zero behavior change

## Referensi

- `.agents/research/2026-04-02-12-tech-refactor-formatter-handler-splits-putaran15.md`
- `.agents/TECH_REFACTOR_PLAN.md` — TECH-001
- `internal/adapter/telegram/formatter_compact.go` — contoh pola file split dengan COT imports
- `internal/adapter/telegram/formatter.go:328` — titik mulai fungsi COT
- TASK-265 — split macro (sebaiknya selesai lebih dulu atau dikerjakan paralel di branch berbeda)
