# TASK-073: Log dan Filter Silent Parse Errors di Bybit Client

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/marketdata/bybit
**Created by:** Research Agent
**Created at:** 2026-04-01 03:00 WIB
**Siklus:** BugHunt

## Deskripsi
Banyak `strconv.ParseFloat` dan `strconv.ParseInt` di bybit/client.go mengabaikan error dengan `_`. Jika Bybit mengirim format tidak terduga, nilai jadi `0.0` dan masuk ke kalkulasi microstructure tanpa peringatan — bisa merusak sinyal buy/sell pressure.

## Konteks
Lokasi masalah:
- Line 164-165: orderbook bid price/qty
- Line 172-173: orderbook ask price/qty  
- Line 223-225: recent trades price/qty/timestamp
- Line 303-304: helper `toF`/`toI` yang digunakan oleh OI data
- Line 370-371: helper `toF`/`toI` untuk data lain
- Line 427-429: long/short ratio buy/sell
- Line 480-481: open interest value

Contoh masalah:
```go
p, _ := strconv.ParseFloat(b[0], 64)  // jika gagal, p = 0
q, _ := strconv.ParseFloat(b[1], 64)  // jika gagal, q = 0
ob.Bids = append(ob.Bids, OrderbookLevel{Price: p, Quantity: q})
// OrderbookLevel dengan price=0 masuk ke perhitungan
```

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Orderbook entries dengan `price == 0` atau `qty == 0` di-skip (tidak diappend)
- [ ] Trade entries dengan `price == 0` di-skip
- [ ] Jika ada parse error, log `Warn` dengan raw string yang gagal diparsed
- [ ] Helper `toF` / `toI` tetap boleh return 0 tapi caller harus filter sebelum append

## File yang Kemungkinan Diubah
- `internal/service/marketdata/bybit/client.go`

## Referensi
- `.agents/research/2026-04-01-03-bug-hunting-subprocess-tempfile-race.md` — Bug #6
