# TASK-240: TECH-011 — handler.go: Ganti currencyToContractCode dengan domain.CodeForCurrency

**Priority:** medium
**Type:** refactor
**Estimated:** XS
**Area:** internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 22:00 WIB

## Deskripsi

`internal/domain/contracts.go` sudah menyediakan `CodeForCurrency(currency string) string` yang melakukan persis apa yang dilakukan `currencyToContractCode()` di handler.go (baris 1368–1398) — map currency string ke CFTC contract code.

Fungsi `currencyToContractCode` adalah **duplicate** dari `domain.CodeForCurrency`. Harus dihapus dan semua caller diganti ke domain helper.

Ini adalah bagian dari TECH-011: menyelesaikan centralisasi contract codes ke `internal/domain/contracts.go`.

## File yang Harus Diubah

- `internal/adapter/telegram/handler.go`
  - Hapus seluruh fungsi `currencyToContractCode()` (baris 1368–1398)
  - Ganti semua call site `currencyToContractCode(x)` → `domain.CodeForCurrency(x)`
  - Pastikan import `internal/domain` sudah ada (kemungkinan sudah ada)

## Analisis Call Sites

```bash
# Cari semua penggunaan:
grep -n "currencyToContractCode" internal/adapter/telegram/handler.go
```

## Implementasi

### Sebelum (handler.go:1368):
```go
func currencyToContractCode(currency string) string {
    m := map[string]string{
        "EUR": "099741",
        "GBP": "096742",
        "JPY": "097741",
        "AUD": "232741",
        "NZD": "112741",
        "CAD": "090741",
        "CHF": "092741",
        "USD": "098662",
    }
    if code, ok := m[strings.ToUpper(currency)]; ok {
        return code
    }
    return ""
}
```

### Sesudah (hapus fungsi, ganti call sites):
```go
// Ganti: currencyToContractCode(currency)
// Dengan: domain.CodeForCurrency(strings.ToUpper(currency))
// atau domain.CodeForCurrency(currency) jika domain sudah ToUpper internally
```

Cek domain.CodeForCurrency di contracts.go untuk verifikasi apakah sudah case-insensitive atau perlu ToUpper.

## Aturan Refactor

- **NO behavior change** — `domain.CodeForCurrency` harus return hasil identik
- `go build ./...` harus bersih setelah perubahan
- `go vet ./...` harus bersih

## Acceptance Criteria

- [ ] Fungsi `currencyToContractCode` dihapus dari handler.go
- [ ] Semua call site menggunakan `domain.CodeForCurrency()`
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Tidak ada perubahan behavior pada output `/cot` atau command yang menggunakan currency → code mapping

## Referensi

- `.agents/research/2026-04-02-22-tech-refactor-plan-putaran10.md` — Temuan 1
- `internal/domain/contracts.go:CodeForCurrency()` — fungsi target
- `internal/adapter/telegram/handler.go:1368` — fungsi yang akan dihapus
