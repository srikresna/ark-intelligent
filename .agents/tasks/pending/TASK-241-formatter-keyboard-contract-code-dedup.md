# TASK-241: TECH-011 — formatter.go + keyboard.go: Ganti Hardcoded Contract Code Maps

**Priority:** medium
**Type:** refactor
**Estimated:** S
**Area:** internal/adapter/telegram/formatter.go, internal/adapter/telegram/keyboard.go
**Created by:** Research Agent
**Created at:** 2026-04-02 22:00 WIB

## Deskripsi

`formatter.go` dan `keyboard.go` masih menggunakan hardcoded CFTC contract code string literals di beberapa tempat, padahal `internal/domain/contracts.go` sudah menyediakan semua konstanta dan maps yang dibutuhkan.

Ditemukan **25 total lokasi** hardcoded codes di 3 file. Task ini menangani formatter.go dan keyboard.go.
(handler.go ditangani terpisah di TASK-240)

Ini adalah bagian dari TECH-011 completion.

## Lokasi Spesifik

### formatter.go:267 — Hardcoded FX code slice
```go
// Sebelum:
Codes: []string{"098662", "099741", "096742", "097741", "232741", "112741", "090741", "092741"},

// Sesudah — derive dari domain.FXContracts:
codes := make([]string, len(domain.FXContracts))
for i, c := range domain.FXContracts {
    codes[i] = string(c.Code)
}
```

### formatter.go:1042 — Hardcoded code→currency reverse map
```go
// Sebelum:
m := map[string]string{
    "099741": "EUR",
    "096742": "GBP",
    "097741": "JPY",
    "092741": "CHF",
    "232741": "AUD",
    "090741": "CAD",
    ...
}

// Sesudah:
// Gunakan domain.CurrencyForCode(code) langsung, atau jika perlu map:
// domain.ContractByCode sudah menyediakan map[ContractCode]ContractInfo
```

### keyboard.go — Contract codes di button data
```bash
# Cari semua hardcoded codes:
grep -n "099741\|096742\|097741\|232741\|112741\|090741\|092741\|098662" \
  internal/adapter/telegram/keyboard.go
```
Ganti dengan konstanta domain: `domain.ContractEUR`, `domain.ContractGBP`, dll.

## File yang Harus Diubah

1. `internal/adapter/telegram/formatter.go`
   - Baris 267: ganti slice literal dengan derive dari `domain.FXContracts`
   - Baris ~1042: ganti map literal dengan `domain.CurrencyForCode(code)` atau `domain.ContractByCode`

2. `internal/adapter/telegram/keyboard.go`
   - Semua string literal contract codes → domain konstanta

## Aturan Refactor

- **NO behavior change** — output tampilan harus identik
- Pastikan `domain.FXContracts` urutan-nya sama dengan yang ada di formatter sebelum diganti
- `go build ./...` harus bersih
- `go vet ./...` harus bersih

## Acceptance Criteria

- [ ] Zero hardcoded CFTC code string literals di formatter.go (kecuali di komentar/docs)
- [ ] Zero hardcoded CFTC code string literals di keyboard.go
- [ ] Semua menggunakan domain.ContractXxx constants atau domain.FXContracts
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Tidak ada perubahan visual pada output command apapun

## Referensi

- `.agents/research/2026-04-02-22-tech-refactor-plan-putaran10.md` — Temuan 1
- `internal/domain/contracts.go` — semua konstanta dan maps tersedia di sini
- `TASK-240` — handler.go cleanup (task terpisah, bisa paralel)
