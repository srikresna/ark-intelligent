# TASK-011: ICT IPDA (Interbank Price Delivery Algorithm) Range Detection

**Priority:** HIGH
**Type:** Feature Enhancement
**Estimated effort:** M (one day)
**Ref:** research/2026-04-06-11-feature-deep-dive-siklus3.md

---

## Context

IPDA (Interbank Price Delivery Algorithm) adalah konsep fundamental dalam ICT methodology.
IPDA mendefinisikan "delivery range" — window 20, 40, dan 60 hari trading ke belakang —
yang digunakan algoritma institusional untuk menentukan ke mana harga akan "didelivery"
selanjutnya.

**Logika IPDA:**
- Ambil high dan low dari 20, 40, dan 60 hari terakhir
- Midpoint setiap range = equilibrium
- Jika harga saat ini > midpoint → "Premium Zone" → institusi cenderung SELL
- Jika harga saat ini < midpoint → "Discount Zone" → institusi cenderung BUY
- Range yang "belum diambil" (untraded liquidity di high/low) = target delivery berikutnya

Saat ini `service/ict/engine.go` dan `ta/ict.go` TIDAK memiliki IPDA sama sekali.

---

## Implementation

### Step 1: Add IPDA types to `internal/service/ict/types.go`

```go
// IPDARange represents one IPDA lookback window (20, 40, or 60 bars).
type IPDARange struct {
    Period    int     // 20, 40, or 60
    High      float64 // highest high over the period
    Low       float64 // lowest low over the period
    Midpoint  float64 // (High + Low) / 2
    Zone      string  // "PREMIUM" | "DISCOUNT" | "EQUILIBRIUM"
    ZonePct   float64 // 0-100, where current price sits within the range
}

// IPDAResult holds all three IPDA lookback windows.
type IPDAResult struct {
    CurrentPrice float64
    Range20      IPDARange
    Range40      IPDARange
    Range60      IPDARange
    Confluence   string  // "PREMIUM" if 2+ ranges agree, "DISCOUNT", or "MIXED"
}
```

Add `IPDA *IPDAResult` field to `ICTResult`.

### Step 2: Add IPDA computation to `internal/service/ict/engine.go`

```go
// computeIPDA calculates the three IPDA delivery ranges from oldest-to-newest bars.
// bars is newest-first (standard input format).
func computeIPDA(bars []ta.OHLCV) *IPDAResult {
    if len(bars) < 20 {
        return nil
    }
    spot := bars[0].Close

    calcRange := func(n int) IPDARange {
        subset := bars[:min(n, len(bars))]
        hi, lo := subset[0].High, subset[0].Low
        for _, b := range subset[1:] {
            if b.High > hi { hi = b.High }
            if b.Low < lo { lo = b.Low }
        }
        mid := (hi + lo) / 2
        pct := 0.0
        if hi > lo { pct = (spot - lo) / (hi - lo) * 100 }
        zone := "EQUILIBRIUM"
        if pct > 55 { zone = "PREMIUM" }
        if pct < 45 { zone = "DISCOUNT" }
        return IPDARange{Period: n, High: hi, Low: lo, Midpoint: mid, Zone: zone, ZonePct: pct}
    }

    r20 := calcRange(20)
    r40 := calcRange(40)
    r60 := calcRange(min(60, len(bars)))

    // Confluence: if 2+ agree on premium/discount
    zones := []string{r20.Zone, r40.Zone, r60.Zone}
    confluence := "MIXED"
    pCount, dCount := 0, 0
    for _, z := range zones {
        if z == "PREMIUM" { pCount++ }
        if z == "DISCOUNT" { dCount++ }
    }
    if pCount >= 2 { confluence = "PREMIUM" }
    if dCount >= 2 { confluence = "DISCOUNT" }

    return &IPDAResult{CurrentPrice: spot, Range20: r20, Range40: r40, Range60: r60, Confluence: confluence}
}
```

Call `computeIPDA(bars)` from `Engine.Analyze()` and store result in `ICTResult.IPDA`.

### Step 3: Display IPDA in `internal/adapter/telegram/formatter_ict.go`

Add IPDA section to the ICT output:

```
📐 <b>IPDA Delivery Range</b>
20D: H:1.0850 L:1.0720 Mid:1.0785 → <b>🔴 PREMIUM (68%)</b>
40D: H:1.0920 L:1.0680 Mid:1.0800 → <b>🔴 PREMIUM (62%)</b>
60D: H:1.0980 L:1.0650 Mid:1.0815 → 🟡 EQUILIBRIUM (53%)
Confluence: <b>PREMIUM</b> — Institusi berpotensi distribusi
```

---

## Acceptance Criteria

- [ ] `IPDARange` dan `IPDAResult` types ditambahkan ke `types.go`
- [ ] `ICTResult.IPDA *IPDAResult` field ditambahkan
- [ ] `computeIPDA()` diimplementasikan dan dipanggil dari `Analyze()`
- [ ] IPDA section tampil di `/ict` output ketika data cukup (≥20 bars)
- [ ] Ketika bars < 20, IPDA section di-skip gracefully
- [ ] Unit test: TestIPDA minimal verifikasi range calculation dan zone classification
- [ ] `go build ./...` dan `go test ./...` bersih
