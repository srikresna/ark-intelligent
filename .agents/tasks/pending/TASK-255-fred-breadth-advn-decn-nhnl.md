# TASK-255: FRED Market Breadth — Tambah ADVN, DECN, NHNL ke MacroData

**Priority:** medium
**Type:** data-source
**Estimated:** S
**Area:** internal/service/fred/fetcher.go, internal/domain/macro.go, internal/service/fred/composites.go, internal/service/ai/unified_outlook.go
**Created by:** Research Agent
**Created at:** 2026-04-02 09:00 WIB

## Deskripsi

FRED API sudah dipakai untuk ratusan macro series. Tiga series NYSE market breadth
yang tersedia gratis belum diambil:
- `ADVN` — NYSE Advancing Issues (daily)
- `DECN` — NYSE Declining Issues (daily)
- `NHNL` — NYSE Net New Highs (New Highs minus New Lows, weekly)

Breadth data ini berguna sebagai health check pasar saham: jika breadth negatif
(lebih banyak saham turun dari naik) bersamaan dengan VIX elevated → risk-off signal
yang kuat. Saat ini unified_outlook tidak punya data ini sama sekali.

Implementasi sangat mudah: cukup tambah ke jobs list yang sudah ada di `fetcher.go`.
Tidak perlu client baru, tidak perlu service baru.

## File yang Harus Diubah

1. `internal/service/fred/fetcher.go` — tambah series ke jobs list + assign ke MacroData
2. `internal/domain/macro.go` — tambah field ke `MacroData` struct (atau `domain`)
3. `internal/service/fred/composites.go` — hitung `AdvDecRatio`
4. `internal/service/ai/unified_outlook.go` — tambah breadth section ke prompt

## Implementasi

### 1. fetcher.go — jobs list (sekitar baris 272)

Tambah ke slice jobs:
```go
{"ADVN", 5},  // NYSE Advancing Issues
{"DECN", 5},  // NYSE Declining Issues
{"NHNL", 5},  // NYSE Net New Highs (weekly)
```

### 2. fetcher.go — assign ke MacroData struct (sekitar baris 425+)

```go
data.AdvancingIssues = single("ADVN")
data.DecliningIssues = single("DECN")
data.NetNewHighs     = single("NHNL")
```

### 3. MacroData struct (fred/fetcher.go atau domain)

```go
// NYSE Market Breadth
AdvancingIssues float64 // NYSE advancing issues (daily)
DecliningIssues float64 // NYSE declining issues (daily)
NetNewHighs     float64 // NYSE net new highs (NHs - NLs, weekly)
```

### 4. composites.go — compute ratio

```go
if data.AdvancingIssues > 0 && data.DecliningIssues > 0 {
    c.AdvDecRatio = data.AdvancingIssues / data.DecliningIssues
}
c.NetNewHighs = data.NetNewHighs
```

Tambah field ke `MacroComposites` struct (domain):
```go
AdvDecRatio float64 // NYSE Adv/Dec ratio (>1 = net bullish breadth)
NetNewHighs float64 // NYSE net new highs
```

### 5. unified_outlook.go — breadth section

Dalam `BuildUnifiedOutlookPrompt`, setelah Risk Sentiment section:
```go
if data.MacroComposites != nil {
    c := data.MacroComposites
    if c.AdvDecRatio > 0 || c.NetNewHighs != 0 {
        b.WriteString(fmt.Sprintf("=== %d. NYSE MARKET BREADTH ===\n", section))
        section++
        b.WriteString(fmt.Sprintf("Adv/Dec Ratio: %.2f (%s) | Net New Highs: %.0f\n",
            c.AdvDecRatio,
            breadthSignal(c.AdvDecRatio),
            c.NetNewHighs))
        b.WriteString("\n")
    }
}
```

Helper:
```go
func breadthSignal(ratio float64) string {
    switch {
    case ratio >= 1.5: return "STRONG_BREADTH"
    case ratio >= 1.0: return "POSITIVE"
    case ratio >= 0.7: return "MIXED"
    default:           return "NEGATIVE_BREADTH"
    }
}
```

## Acceptance Criteria

- [ ] `MacroData` struct punya field `AdvancingIssues`, `DecliningIssues`, `NetNewHighs`
- [ ] `fetcher.go` mengambil ADVN, DECN, NHNL dari FRED
- [ ] `MacroComposites` punya `AdvDecRatio` dan `NetNewHighs`
- [ ] `/outlook` prompt includes "NYSE MARKET BREADTH" section saat data tersedia
- [ ] Jika ADVN/DECN 0 (data tidak tersedia) — section dilewati, tidak crash
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-09-data-sources-audit-gaps-putaran13.md` — Temuan #3
- `internal/service/fred/fetcher.go:272` — jobs list (tambah 3 series di sini)
- `internal/service/fred/fetcher.go:425` — assign section (tambah di sini)
- `internal/service/fred/composites.go:52` — compute composites
- `internal/service/ai/unified_outlook.go:258` — Risk Sentiment section (tambah setelah ini)
- FRED series: https://fred.stlouisfed.org/series/ADVN (free, same API key)
