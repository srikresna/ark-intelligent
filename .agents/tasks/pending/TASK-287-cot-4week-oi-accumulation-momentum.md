# TASK-287: COT 4-Week OI Accumulation Momentum Score

**Priority:** medium
**Type:** enhancement
**Estimated:** M
**Area:** internal/service/cot/analyzer.go, internal/domain/cot.go
**Created by:** Research Agent
**Created at:** 2026-04-02 27:00 WIB

## Deskripsi

`OITrend` yang ada di `analyzer.go:302-306` hanya mengukur 1-week OI change (current vs prev). Ini terlalu noise. **Multi-week OI accumulation** adalah sinyal lebih kuat: jika OI naik 4 minggu berturut = smart money sedang aktif commit ke posisi (accumulation). Jika OI turun 4 minggu berturut = smart money sedang exit (distribution).

**Signal Logic:**
```
4 consecutive OI rising  → OI4WTrend = "ACCUMULATING"  → conviction boost +5
4 consecutive OI falling → OI4WTrend = "DISTRIBUTING"  → conviction drag -5  
Mixed                    → OI4WTrend = "MIXED"          → neutral
Only 2 data points       → OI4WTrend = "INSUFFICIENT"  → skip
```

**Kombinasi paling kuat:**
- OI4WTrend=ACCUMULATING + Spec net BULLISH = confirmation: "smart money building longs"
- OI4WTrend=DISTRIBUTING + Spec net BULLISH = divergence warning: "longs exiting meski bullish"
- OI4WTrend=ACCUMULATING + Spec net BEARISH = confirmation: "smart money building shorts"

## Perubahan yang Diperlukan

### 1. Tambah fields ke `domain/cot.go` (struct `COTAnalysis`)

```go
// 4-Week Open Interest Momentum
OI4WTrend      string  `json:"oi_4w_trend"`      // "ACCUMULATING", "DISTRIBUTING", "MIXED", "INSUFFICIENT"
OI4WMomentum   float64 `json:"oi_4w_momentum"`   // avg weekly OI change over 4 weeks (%)
OI4WAccumCount int     `json:"oi_4w_accum_count"` // consecutive weeks of OI rising (positive = accum)
```

### 2. Kalkulasi di `internal/service/cot/analyzer.go` (seksi 5, setelah OITrend)

```go
// 5c. 4-Week OI Accumulation Momentum
if len(history) >= 4 {
    consecutiveUp := 0
    consecutiveDown := 0
    weeklyChanges := make([]float64, 0, 4)
    
    for i := 0; i < minInt(4, len(history)-1); i++ {
        currOI := history[i].OpenInterest
        prevOI := history[i+1].OpenInterest
        if prevOI <= 0 {
            continue
        }
        chgPct := (currOI - prevOI) / prevOI * 100
        weeklyChanges = append(weeklyChanges, chgPct)
        if chgPct > 0.5 {
            consecutiveUp++
        } else if chgPct < -0.5 {
            consecutiveDown++
        }
    }
    
    // Average OI momentum
    if len(weeklyChanges) > 0 {
        sum := 0.0
        for _, c := range weeklyChanges {
            sum += c
        }
        analysis.OI4WMomentum = sum / float64(len(weeklyChanges))
    }
    
    switch {
    case consecutiveUp >= 3:
        analysis.OI4WTrend = "ACCUMULATING"
        analysis.OI4WAccumCount = consecutiveUp
    case consecutiveDown >= 3:
        analysis.OI4WTrend = "DISTRIBUTING"
        analysis.OI4WAccumCount = -consecutiveDown
    case len(weeklyChanges) >= 2:
        analysis.OI4WTrend = "MIXED"
    default:
        analysis.OI4WTrend = "INSUFFICIENT"
    }
} else {
    analysis.OI4WTrend = "INSUFFICIENT"
}
```

### 3. Integrasikan ke `ConvictionScoreV3` di `internal/service/cot/confluence_score.go`

Cari section yang menyesuaikan OITrend. Tambah adjustment untuk OI4WTrend:

```go
// OI 4-week momentum boosts/drags conviction
switch a.OI4WTrend {
case "ACCUMULATING":
    if isLong {
        score += 5  // smart money building — confirm long
    } else {
        score -= 5  // OI building contradicts short signal
    }
case "DISTRIBUTING":
    if isLong {
        score -= 5  // smart money exiting — warn against long
    } else {
        score += 5  // distribution confirms short
    }
}
```

### 4. Surface di formatter COT (`internal/adapter/telegram/formatter.go`)

Di section OI Change di formatter, tambah OI4W line:
```go
if a.OI4WTrend != "" && a.OI4WTrend != "INSUFFICIENT" {
    icon := "➡️"
    if a.OI4WTrend == "ACCUMULATING" { icon = "📈" }
    if a.OI4WTrend == "DISTRIBUTING" { icon = "📉" }
    b.WriteString(fmt.Sprintf("<code>  OI 4W Trend:    %s %s (avg %+.1f%%/wk)</code>\n",
        icon, a.OI4WTrend, a.OI4WMomentum))
}
```

## File yang Harus Diubah

1. `internal/domain/cot.go` — tambah 3 fields ke `COTAnalysis`
2. `internal/service/cot/analyzer.go` — kalkulasi OI4WTrend di section 5c
3. `internal/service/cot/confluence_score.go` — tambah OI4W adjustment ke ConvictionScoreV3
4. `internal/adapter/telegram/formatter.go` — tampilkan OI4WTrend di COT detail

## Verifikasi

```bash
go build ./...
go test ./internal/service/cot/...
# Manual: /cot EUR → lihat "OI 4W Trend" muncul di detail
# Periksa /cot detail saat OI sedang ACCUMULATING → conviction score lebih tinggi
```

## Acceptance Criteria

- [ ] `OI4WTrend` field populated di `COTAnalysis`
- [ ] ACCUMULATING/DISTRIBUTING terdeteksi dengan benar dari history
- [ ] ConvictionScoreV3 dipengaruhi OI4WTrend (+/-5)
- [ ] Ditampilkan di formatter COT detail view
- [ ] Gracefully handle jika history < 4 minggu (INSUFFICIENT)
- [ ] `go build ./...` + `go test ./internal/service/cot/...` clean

## Referensi

- `.agents/research/2026-04-02-27-feature-index-gaps-carry-gjrgarch-oi4w-hmm4-vix-putaran19.md` — GAP 3
- `internal/service/cot/analyzer.go:280-306` — OITrend 1-week (template)
- `internal/service/cot/confluence_score.go` — ConvictionScoreV3 untuk integrasi
- `internal/domain/cot.go:295-310` — COTAnalysis struct fields area
