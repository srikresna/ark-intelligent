# TASK-283: Inject EIA Energy Data ke /outlook dan /macro AI Context

**Priority:** medium
**Type:** feature
**Estimated:** M
**Area:** internal/service/ai/unified_outlook.go, internal/adapter/telegram/handler.go, internal/service/price/eia.go
**Created by:** Research Agent
**Created at:** 2026-04-02 26:00 WIB

## Deskripsi

EIA Energy Client (`internal/service/price/eia.go`) sudah implement pengambilan data crude oil inventory, gasoline, distillate, refinery utilization dari EIA API v2 (gratis, perlu API key). Saat ini **hanya dipakai di `/seasonal` command** (`handler_seasonal.go:96-101`).

Data EIA seharusnya tersedia di `/outlook` untuk analisis USDCAD, AUDUSD, dan commodity currencies karena crude inventory adalah leading indicator untuk minyak.

**Kondisi:** EIA_API_KEY sudah ada di `.env` (disebutkan di DATA_SOURCES_AUDIT.md sebagai "freemium, sudah di .env").

## Perubahan yang Diperlukan

### 1. Tambah EIAData ke `UnifiedOutlookData`

File: `internal/service/ai/unified_outlook.go`

```go
import pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"

type UnifiedOutlookData struct {
    // ... existing fields ...
    EIAData *pricesvc.EIASeasonalData // crude oil, gasoline, distillate inventory
}
```

### 2. Buat helper untuk ringkasan EIA di prompt

Di unified_outlook.go, tambah helper `buildEIASummary`:

```go
func buildEIASummary(eiaData *pricesvc.EIASeasonalData) string {
    if eiaData == nil || len(eiaData.CrudeInventory) == 0 {
        return ""
    }
    // Latest crude inventory + 4-week trend
    latest := eiaData.CrudeInventory[0].Value
    var weeklyChanges []float64
    for i := 1; i < min(5, len(eiaData.CrudeInventory)); i++ {
        weeklyChanges = append(weeklyChanges, eiaData.CrudeInventory[i-1].Value - eiaData.CrudeInventory[i].Value)
    }
    avg := avgFloat(weeklyChanges)
    trend := "FLAT"
    if avg > 1.0 { trend = "BUILD" } else if avg < -1.0 { trend = "DRAW" }
    return fmt.Sprintf("Crude: %.1fMbbl (4wk avg Δ %.1fMbbl/wk → %s)", latest, avg, trend)
}
```

### 3. Tambah section EIA ke `BuildUnifiedOutlookPrompt`

```go
if data.EIAData != nil && len(data.EIAData.CrudeInventory) > 0 {
    b.WriteString(fmt.Sprintf("=== %d. EIA ENERGY DATA ===\n", section))
    section++
    b.WriteString(buildEIASummary(data.EIAData))
    b.WriteString("\n(Relevant for USDCAD, AUDUSD, USDNOK analysis)\n\n")
}
```

### 4. Fetch EIA di handler.go

File: `internal/adapter/telegram/handler.go` — di sekitar line 1004

```go
// Fetch EIA energy data (best-effort, requires EIA_API_KEY)
var eiaData *pricesvc.EIASeasonalData
if eiaKey := os.Getenv("EIA_API_KEY"); eiaKey != "" {
    eiaClient := pricesvc.NewEIAClient(eiaKey)
    if d, err := eiaClient.FetchSeasonalData(ctx); err == nil {
        eiaData = d
    }
}

unifiedData := aisvc.UnifiedOutlookData{
    // ... existing ...
    EIAData: eiaData,
}
```

**Note:** Perhatikan bahwa `FetchSeasonalData` mengambil 5 tahun data — ini mahal untuk dipanggil setiap kali `/outlook` dipanggil. Pertimbangkan untuk cache EIA data di handler atau dependency injection level dengan TTL 12 jam (data EIA update mingguan).

### 5. Cache EIA di dependency injection (opsional tapi disarankan)

Tambah `EIACache` ke handler dependencies atau gunakan package-level cache di `price` package untuk avoid 5-year data re-fetch setiap command.

## File yang Harus Diubah

1. `internal/service/ai/unified_outlook.go` — tambah field + prompt section
2. `internal/adapter/telegram/handler.go` — fetch EIA (dengan cache) dan inject
3. `internal/service/price/eia.go` — mungkin tambah TTL cache wrapper

## Verifikasi

```bash
go build ./...
# Set EIA_API_KEY di .env
# Manual: /outlook → cek section EIA Energy muncul
```

## Acceptance Criteria

- [ ] `UnifiedOutlookData` memiliki field `EIAData *pricesvc.EIASeasonalData`
- [ ] Handler fetch EIA jika `EIA_API_KEY` tersedia (gracefully skip jika tidak ada key)
- [ ] EIA fetch di-cache minimal 12 jam untuk hindari re-fetch mahal
- [ ] `BuildUnifiedOutlookPrompt` include section EIA jika data tersedia
- [ ] Section mencantumkan crude inventory level + 4-week trend direction
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-26-data-sources-audit-putaran18.md` — GAP-DS1
- `internal/service/price/eia.go` — EIAClient, FetchSeasonalData(), EIASeasonalData struct
- `internal/adapter/telegram/handler_seasonal.go:96-101` — contoh penggunaan EIAClient
- `internal/service/ai/unified_outlook.go` — UnifiedOutlookData struct
