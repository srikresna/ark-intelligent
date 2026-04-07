# TASK-268: Split handler.go — Ekstrak Admin/Membership Handlers ke handler_admin.go

**Priority:** medium
**Type:** refactor
**Estimated:** S
**Area:** internal/adapter/telegram/handler.go → handler_admin.go
**Created by:** Research Agent
**Created at:** 2026-04-02 12:00 WIB

## Deskripsi

Kelanjutan TECH-002. Setelah TASK-267 (macro split), handler.go masih ~2,656 LOC.
Admin dan membership commands (~254 LOC, 6 fungsi) logis dikelompokkan ke `handler_admin.go`.

**Zero behavior change** — pure file split.

## Fungsi yang Dipindah (dari handler.go → handler_admin.go)

| Fungsi | Baris (approx) |
|--------|---------------|
| cmdMembership | L2329 |
| requireAdmin | L2404 |
| cmdUsers | L2417 |
| cmdSetRole | L2440 |
| cmdBan | L2505 |
| cmdUnban | L2552 |

**Total estimasi dipindah:** ~254 LOC
**handler.go setelah split (dari ~2,656):** ~2,402 LOC

## Implementasi

### 1. Buat handler_admin.go

```go
package telegram

// handler_admin.go — Admin and membership management handlers (/ban, /unban, /users, /setrole, /membership).

import (
    "context"
    "fmt"
    "strings"

    // import sesuai kebutuhan fungsi yang dipindah
)
```

Pindahkan 6 fungsi ke file baru ini.

### 2. Hapus dari handler.go

Hapus 6 fungsi dari handler.go. Bersihkan import yang tidak lagi digunakan.

### 3. Verifikasi

```bash
go build ./...
go vet ./...
```

## Acceptance Criteria

- [ ] `handler_admin.go` baru berisi 6 fungsi admin/membership
- [ ] Fungsi-fungsi tersebut dihapus dari `handler.go`
- [ ] `handler.go` setelah split ≤ 2,450 LOC
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `/ban`, `/unban`, `/users`, `/setrole`, `/membership` commands tetap berfungsi
- [ ] `requireAdmin` tetap bisa dipanggil dari handler.go (masih satu package — tidak ada perubahan)

## Catatan Urutan

Sebaiknya dikerjakan **setelah atau bersamaan dengan TASK-267** di branch berbeda. Keduanya
menyentuh handler.go di section yang berbeda (TASK-267: L2039-2291, TASK-268: L2329-2583),
sehingga merge conflict minimal.

## Referensi

- `.agents/research/2026-04-02-12-tech-refactor-formatter-handler-splits-putaran15.md`
- `.agents/TECH_REFACTOR_PLAN.md` — TECH-002
- `internal/adapter/telegram/handler_alpha.go` — contoh pola handler split
- `internal/adapter/telegram/handler.go:2329` — titik mulai cmdMembership
- TASK-267 — split macro (bisa dikerjakan paralel di branch berbeda)
