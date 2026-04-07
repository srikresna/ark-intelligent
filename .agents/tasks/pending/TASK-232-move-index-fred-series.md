# TASK-232: MOVE Index — Bond Volatility via FRED (ICE BofA MOVE)

**Priority:** medium
**Type:** data
**Estimated:** S
**Area:** internal/service/fred/fetcher.go, internal/service/fred/regime.go
**Created by:** Research Agent
**Created at:** 2026-04-02 19:00 WIB

## Deskripsi

MOVE Index (Merrill Lynch Option Volatility Estimate) adalah "VIX untuk bond market". Mengukur implied volatility 1-bulan dari US Treasury options. Available via FRED (series `MOVE`) — sudah ada FRED client dan fetcher, tinggal tambah satu series.

**Gap saat ini:** `sentiment/vix` fetcher sudah ada VIX (equity vol), VVIX, dan term structure. Tapi tidak ada bond market volatility. VIX + MOVE divergence adalah signal penting:
- `VIX rendah + MOVE tinggi` = equity tenang, bond panik → rate shock / debt ceiling risk
- `VIX tinggi + MOVE rendah` = equity jual off, bond stabil → growth scare, bukan rate shock
- Keduanya tinggi = systemic stress

**FRED series:** `MOVE` (ICE BofA MOVE Index, daily, basis points). Tersedia sejak 1988.

## File yang Harus Diubah

- `internal/service/fred/fetcher.go` — Tambah `MOVE` ke series fetch list + parsing
- `internal/domain/composites.go` — Tambah `MOVEIndex float64` ke FREDData
- `internal/service/fred/regime.go` — Tambah MOVE regime signal ke macro regime analysis
- `internal/adapter/telegram/formatter.go` — Tampilkan MOVE di output /macro atau /sentiment

## Implementasi

### fetcher.go — Tambah MOVE ke fetch list

```go
// Existing fetch list di FetchFREDData():
{"MOVE", 5},  // ICE BofA MOVE Index — bond market volatility
```

### domain/composites.go — Tambah field ke FREDData

```go
// Di struct FREDExtendedData atau FREDData:
MOVEIndex float64 // MOVE — ICE BofA Bond Market Volatility Index (basis points)
```

### regime.go — Tambah MOVE/VIX cross-signal

```go
// ComputeMOVEVIXDivergence: signal berdasarkan relatif level kedua index
// MOVE > 120 = elevated bond vol; MOVE > 150 = extreme
// MOVE/VIX ratio > 6 = bond market lebih takut dari equity (unusual)
func ComputeMOVEVIXDivergence(move, vix float64) string {
    // Returns: "BOND_STRESS", "EQUITY_STRESS", "DUAL_STRESS", "CALM"
}
```

### formatter.go — Tampilkan di /macro output

```
📊 Bond Vol (MOVE): 95.2 bps → Elevated
   MOVE/VIX ratio: 6.1 → Bond market more fearful than equity
```

## Acceptance Criteria

- [ ] FRED fetcher berhasil fetch series `MOVE` daily
- [ ] `FREDData.MOVEIndex` terisi dengan nilai terbaru
- [ ] MOVE level tersedia di /macro output
- [ ] MOVE/VIX divergence signal dihitung dan ditampilkan jika signifikan
- [ ] Jika MOVE tidak tersedia (FRED error) → skip gracefully
- [ ] Unit test: `TestMOVEVIXDivergence` untuk 4 scenario (calm, equity stress, bond stress, dual)

## Referensi

- `.agents/research/2026-04-02-19-data-fed-speeches-cg-trending-edgar-form4-putaran8.md` — Temuan 4
- `internal/service/fred/fetcher.go` — Series fetch list (baris ~271-285)
- `internal/domain/composites.go` — FREDData struct
- `internal/service/vix/` — VIX data sudah tersedia untuk MOVE/VIX comparison
- FRED: `api.stlouisfed.org/fred/series?series_id=MOVE`
