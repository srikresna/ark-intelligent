# TASK-014: COT Disaggregated Report — Swap Dealer vs Leveraged Fund Divergence

**Priority:** MEDIUM
**Type:** New Data Analysis Feature
**Estimated effort:** M (one day)
**Ref:** research/2026-04-06-11-feature-deep-dive-siklus3.md

---

## Context

CFTC menerbitkan tiga jenis COT report:
1. **Legacy** — Commercial vs Non-Commercial (lama, kurang informatif)
2. **TFF (Traders in Financial Futures)** — Dealer, Asset Manager, Leveraged, Other (sudah ada)
3. **Disaggregated** — Producer/Merchant + Swap Dealers + Managed Money + Other Reportables

Saat ini `/cot` dan `/rank` menggunakan TFF dan Legacy. Analisis yang hilang:

**Swap Dealer vs Leveraged Fund Divergence** — sinyal paling valuable:
- **Swap Dealers** = prime brokers / institutional banks. Posisi mereka mencerminkan
  real hedging demand dari end-users korporat dan sovereign. Biasanya counter-trend
  pada extremes (mereka hedge kebalikan dari posisi klien besar).
- **Leveraged Funds** = CTAs, hedge funds. Biasanya trend-following.
- Ketika Swap Dealer dan Leveraged Fund bergerak BERLAWANAN pada extreme = potential reversal.

CFTC Disaggregated data tersedia GRATIS via CFTC API yang sudah dipakai.

---

## Implementation

### Step 1: Verifikasi data availability

Cek apakah `ports.COTRepository` sudah menyimpan disaggregated data, atau hanya TFF.

```go
// Check domain.COTRecord untuk field disaggregated
// Jika belum ada, tambahkan ke domain/cot.go:
type COTRecord struct {
    // ... existing fields ...
    
    // Disaggregated fields (CFTC Disaggregated report)
    SwapDealerNet     *int64 // nil if not available for this contract
    LeveragedFundNet  *int64
    ManagedMoneyNet   *int64
}
```

### Step 2: Fetch disaggregated data

Di `internal/service/cot/fetcher.go` (atau file baru `fetcher_disaggregated.go`):
- CFTC Disaggregated endpoint sudah tersedia di CFTC public API
- Tambahkan fetch + parse untuk swap_dealer_positions_long/short
- Cache bersama dengan TFF data (update mingguan, sama jadwalnya)

### Step 3: Add divergence analysis

New file: `internal/service/cot/disaggregated.go`

```go
// DisaggregatedDivergence analyzes Swap Dealer vs Leveraged Fund positioning gap.
type DisaggregatedDivergence struct {
    SwapDealerNet     int64
    LeveragedFundNet  int64
    Spread            int64   // SwapDealer - LeveragedFund
    SpreadPercentile  float64 // where current spread ranks in 1yr history
    Signal            string  // "BULLISH_REVERSAL", "BEARISH_REVERSAL", "NEUTRAL"
    Description       string
}
```

### Step 4: Surface in /cot output

Tambahkan section ke formatter ketika disaggregated data tersedia:

```
📊 <b>Disaggregated Positioning</b>
Swap Dealer (bank): <code>-45,200</code> 🔴
Leveraged Fund (CTA): <code>+38,100</code> 🟢
Divergence: <b>83,300 — Extreme (91st pctile)</b>
Signal: ⚠️ Potential reversal — bank vs CTA di extreme

<i>Swap Dealer biasanya benar pada extreme divergence</i>
```

---

## Acceptance Criteria

- [ ] Cek apakah disaggregated data sudah tersedia di existing domain/storage
- [ ] Jika belum: tambahkan fetch disaggregated fields ke COT fetcher
- [ ] `DisaggregatedDivergence` type dan `CalcDisaggregatedDivergence()` diimplementasikan
- [ ] `/cot [currency]` menampilkan disaggregated section ketika data tersedia
- [ ] Graceful degradation: section di-skip jika contract tidak ada di Disaggregated report
- [ ] Percentile calculation menggunakan 52-week rolling window
- [ ] Unit test untuk divergence calculation dan percentile scoring
- [ ] `go build ./...` bersih

---

## Note

Jika domain.COTRecord sudah memiliki disaggregated fields dari existing CFTC fetcher,
scope task ini berkurang menjadi: (1) analisis divergence + (2) formatter — effort XS/S.
Periksa ini PERTAMA sebelum mengimplementasikan fetch baru.
