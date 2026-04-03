# TASK-269: Fix TECH-011 — Ganti Hardcoded Contract Code Literals dengan domain.ContractXXX

**Priority:** low
**Type:** refactor
**Estimated:** XS
**Area:** internal/adapter/telegram/handler.go, internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-02 12:00 WIB

## Deskripsi

`internal/domain/contracts.go` sudah mendefinisikan konstanta CFTC contract codes:
```go
ContractEUR   ContractCode = "099741"
ContractGBP   ContractCode = "096742"
// ... dst
```

Namun 3 lokasi di adapter/telegram masih menggunakan string literals hardcoded:

1. `handler.go:1368` — fungsi `currencyToContractCode()` map string literals
2. `formatter.go:267` — array `Codes` dengan 8 hardcoded strings
3. `formatter.go:1042` — reverse map code→currency dengan 8 hardcoded strings

**Zero behavior change** — hanya ganti string literal dengan konstanta dari domain package.

## File yang Harus Diubah

### handler.go — currencyToContractCode()

Lokasi: `internal/adapter/telegram/handler.go:1368`

**Sebelum:**
```go
func currencyToContractCode(currency string) string {
    mapping := map[string]string{
        "EUR":  "099741", // Euro FX
        "GBP":  "096742", // British Pound
        "JPY":  "097741", // Japanese Yen
        "AUD":  "232741", // Australian Dollar
        "NZD":  "112741", // New Zealand Dollar
        "CAD":  "090741", // Canadian Dollar
        "CHF":  "092741", // Swiss Franc
        "USD":  "098662", // US Dollar Index
        "GOLD": "088691", // Gold
        "XAU":  "088691", // Gold alias
        "OIL":  "067651", // Crude Oil
    }
    ...
}
```

**Sesudah:**
```go
func currencyToContractCode(currency string) string {
    mapping := map[string]string{
        "EUR":  string(domain.ContractEUR),
        "GBP":  string(domain.ContractGBP),
        "JPY":  string(domain.ContractJPY),
        "AUD":  string(domain.ContractAUD),
        "NZD":  string(domain.ContractNZD),
        "CAD":  string(domain.ContractCAD),
        "CHF":  string(domain.ContractCHF),
        "USD":  string(domain.ContractDXY),
        "GOLD": string(domain.ContractGold),
        "XAU":  string(domain.ContractGold),
        "OIL":  string(domain.ContractOil),
    }
    ...
}
```

Tambahkan import `domain` jika belum ada (sudah ada di handler.go).

### formatter.go — Codes array (L267)

**Sebelum:**
```go
Codes: []string{"098662", "099741", "096742", "097741", "232741", "112741", "090741", "092741"},
```

**Sesudah:**
```go
Codes: []string{
    string(domain.ContractDXY),
    string(domain.ContractEUR),
    string(domain.ContractGBP),
    string(domain.ContractJPY),
    string(domain.ContractAUD),
    string(domain.ContractNZD),
    string(domain.ContractCAD),
    string(domain.ContractCHF),
},
```

### formatter.go — reverse map (L1042)

**Sebelum:**
```go
"099741": "EUR",
"096742": "GBP",
// ... dst
"098662": "USD",
```

**Sesudah:**
```go
string(domain.ContractEUR): "EUR",
string(domain.ContractGBP): "GBP",
// ... dst
string(domain.ContractDXY): "USD",
```

## Verifikasi

```bash
go build ./...
go vet ./...
```

## Acceptance Criteria

- [ ] `currencyToContractCode()` di handler.go menggunakan `domain.ContractXXX` untuk semua entries
- [ ] `Codes` array di formatter.go menggunakan `domain.ContractXXX` string() casts
- [ ] Reverse map di formatter.go menggunakan `domain.ContractXXX` keys
- [ ] Tidak ada string literal contract code tersisa di kedua file (99741, 098662, dll.)
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] Zero behavior change — output mapping identik dengan sebelumnya

## Referensi

- `.agents/research/2026-04-02-12-tech-refactor-formatter-handler-splits-putaran15.md`
- `.agents/TECH_REFACTOR_PLAN.md` — TECH-011
- `internal/domain/contracts.go:14` — definisi semua konstanta ContractXXX
- `internal/adapter/telegram/handler.go:1368` — currencyToContractCode() function
- `internal/adapter/telegram/formatter.go:267` — Codes array
- `internal/adapter/telegram/formatter.go:1042` — reverse code→currency map
