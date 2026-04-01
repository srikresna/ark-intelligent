# TASK-017: Centralize CFTC Contract Codes ke domain/contracts.go

**Status:** pending
**Priority:** MEDIUM
**Effort:** S (Small — estimasi 30-45 menit)
**Cycle:** Siklus 4 — Technical Refactor
**Ref:** TECH-011 in TECH_REFACTOR_PLAN.md

---

## Problem

CFTC contract codes (contoh: "099741" untuk EUR) tersebar di **33 lokasi** di non-test files:

```
internal/adapter/telegram/handler.go:1103   → EUR: "099741"
internal/adapter/telegram/formatter.go:266  → array 8 codes
internal/adapter/telegram/formatter.go:1008 → reverse map
internal/adapter/telegram/keyboard.go:228   → duplicate map
```

Jika CFTC ganti kode kontrak, atau kita tambah instrumen baru, harus edit 4+ tempat berbeda.

---

## Solution

Buat file `internal/domain/contracts.go`:

```go
package domain

// CFTC contract codes for COT reporting
const (
    ContractEUR = "099741" // Euro FX (6E)
    ContractGBP = "096742" // British Pound (6B)
    ContractJPY = "097741" // Japanese Yen (6J)
    ContractAUD = "232741" // Australian Dollar (6A)
    ContractCAD = "090741" // Canadian Dollar (6C)
    ContractCHF = "092741" // Swiss Franc (6S)
    ContractNZD = "112741" // New Zealand Dollar (6N)
    ContractDXY = "098662" // US Dollar Index
)

// AllCOTContracts adalah list semua contract yang di-track
var AllCOTContracts = []string{
    ContractDXY, ContractEUR, ContractGBP, ContractJPY,
    ContractAUD, ContractCAD, ContractCHF, ContractNZD,
}

// ContractToCurrency maps contract code ke nama currency
var ContractToCurrency = map[string]string{
    ContractEUR: "EUR",
    ContractGBP: "GBP",
    ContractJPY: "JPY",
    ContractAUD: "AUD",
    ContractCAD: "CAD",
    ContractCHF: "CHF",
    ContractNZD: "NZD",
    ContractDXY: "USD",
}

// CurrencyToContract maps currency name ke contract code
var CurrencyToContract = map[string]string{
    "EUR": ContractEUR,
    "GBP": ContractGBP,
    // ... dst
}
```

Kemudian replace semua hardcoded strings di handler.go, formatter.go, keyboard.go dengan constants ini.

---

## Acceptance Criteria

- [ ] `internal/domain/contracts.go` dibuat dengan semua 8 contracts
- [ ] Semua hardcoded contract codes di non-test files diganti dengan constants
- [ ] `go build ./...` clean
- [ ] `go test ./...` pass (test files yang pakai literal string boleh dibiarkan)
- [ ] TIDAK ada behavior change

---

## Implementation Notes

1. Cek dulu full list contract codes yang ada di formatter.go:266 (array 8 elements)
2. Cross-reference dengan `internal/service/cot/` untuk pastikan list lengkap
3. Ganti di handler.go dan keyboard.go — formatter.go bisa tunggu TASK-015

---

## Assigned To

(unassigned — task ini AMAN dikerjakan parallel dengan task lain, tidak ada conflict risk)
