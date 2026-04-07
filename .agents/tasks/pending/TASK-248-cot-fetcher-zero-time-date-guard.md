# TASK-248: BUG-004 — COT fetcher: `socrataToRecord` menyimpan record dengan zero-time date

**Priority:** medium
**Type:** bugfix
**Estimated:** XS
**Area:** internal/service/cot/fetcher.go
**Created by:** Research Agent
**Created at:** 2026-04-02 23:00 WIB

## Deskripsi

Di `socrataToRecord()` (baris 344), kedua error dari `time.Parse` dibuang dengan `_`. Jika format tanggal dari API CFTC Socrata tidak cocok dengan KEDUA format yang dicoba, `reportDate` tetap bernilai `time.Time{}` (zero time = 0001-01-01 00:00:00 UTC):

```go
func socrataToRecord(sr domain.SocrataRecord, contract domain.COTContract) domain.COTRecord {
    reportDate, _ := time.Parse("2006-01-02T15:04:05.000", sr.ReportDate)
    if reportDate.IsZero() && len(sr.ReportDate) >= 10 {
        reportDate, _ = time.Parse("2006-01-02", sr.ReportDate[:10])
    }
    // Jika kedua gagal: reportDate = 0001-01-01 00:00:00
    // Record tetap di-return tanpa warning!
```

**Dampak:**
- Record dengan tanggal `0001-01-01` tersimpan di BadgerDB
- Saat query range (`GetHistory`, `GetByDateRange`), record ini muncul sebagai "paling lama"
- COT Index, Z-score, dan decay analysis bergantung pada urutan tanggal — record zero-date bisa mengacaukan semua kalkulasi ini
- Tidak ada log warning → silent data corruption

## File yang Harus Diubah

- `internal/service/cot/fetcher.go`
  - Di `socrataToRecord()`: tambah guard setelah kedua parse attempt
  - Log warning + return empty record jika tanggal masih zero setelah semua attempt

## Implementasi

### Sesudah (fetcher.go):
```go
func socrataToRecord(sr domain.SocrataRecord, contract domain.COTContract) domain.COTRecord {
    reportDate, _ := time.Parse("2006-01-02T15:04:05.000", sr.ReportDate)
    if reportDate.IsZero() && len(sr.ReportDate) >= 10 {
        reportDate, _ = time.Parse("2006-01-02", sr.ReportDate[:10])
    }
    // Guard: jika masih zero, log warning dan return record kosong
    if reportDate.IsZero() {
        log.Warn().
            Str("contract", contract.Code).
            Str("raw_date", sr.ReportDate).
            Msg("cot/fetcher: socrataToRecord: unrecognized date format, skipping record")
        return domain.COTRecord{} // caller harus cek ContractCode == "" atau ReportDate.IsZero()
    }
    // ... rest of function
```

Kemudian di caller yang memproses hasil `socrataToRecord`, tambah filter:
```go
rec := socrataToRecord(sr, contract)
if rec.ReportDate.IsZero() {
    continue // skip record dengan date tidak valid
}
records = append(records, rec)
```

Cari caller di fungsi yang sama (kemungkinan di `FetchSOCRATA` atau fungsi parse loop).

## Analisis Call Sites

```bash
grep -n "socrataToRecord" internal/service/cot/fetcher.go
```

## Acceptance Criteria

- [ ] Jika kedua `time.Parse` gagal, log.Warn ditulis dengan raw date string
- [ ] Record dengan zero-time TIDAK disimpan ke DB
- [ ] Caller loop yang memanggil `socrataToRecord` menambahkan `continue` untuk skip zero-date records
- [ ] `go build ./...` sukses

## Referensi

- `.agents/research/2026-04-02-23-bug-hunt-putaran11.md` — BUG-004
- `internal/service/cot/fetcher.go:344` — lokasi bug
- `internal/service/cot/fetcher.go` — cari caller `socrataToRecord` untuk menambah filter
