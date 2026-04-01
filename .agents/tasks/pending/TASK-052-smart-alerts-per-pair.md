# TASK-052: Smart Alerts per Pair — Granular Alert Config di UserPrefs

**Priority:** medium
**Type:** feature
**Estimated:** L (4-6 jam)
**Area:** internal/domain/prefs.go, internal/adapter/telegram/handler.go, internal/scheduler/
**Created by:** Research Agent
**Created at:** 2026-04-01 18:00 WIB
**Siklus:** UX-1 (Siklus 1 Sesi 2)

## Deskripsi

`UserPrefs.COTAlertsEnabled` hanya master switch (bool) — semua atau tidak sama sekali.
`CurrencyFilter []string` ada tapi hanya untuk economic calendar, tidak untuk COT alerts.
UX_AUDIT TASK-UX-008 merekomendasikan alert per pair dengan threshold custom.

**Saat ini:** Alert COT = global, tidak bisa per pair, tidak ada threshold conviction.
**Target:** User bisa set "alert jika EUR conviction score berubah > 1.5 poin" atau "alert jika GBP bias flip".

## Solution

### 1. Extend `UserPrefs` di domain/prefs.go

```go
// PairAlert defines alert criteria for a specific currency pair.
type PairAlert struct {
    Currency        string  `json:"currency"`          // "EUR", "GBP", etc.
    ConvictionDelta float64 `json:"conviction_delta"`  // Alert if conviction changes by this amount (0 = any change)
    BiasFlip        bool    `json:"bias_flip"`         // Alert on bullish↔bearish flip
    Enabled         bool    `json:"enabled"`
}

// UserPrefs additions:
PairAlerts []PairAlert `json:"pair_alerts,omitempty"` // Per-pair alert config
```

### 2. Tambah `/setalert` command

```
/setalert EUR          → alert jika EUR bias berubah (default: any change)
/setalert EUR 2.0      → alert jika EUR conviction delta > 2.0
/setalert EUR flip     → alert hanya saat bias flip bullish↔bearish
/setalert list         → tampilkan semua pair alert aktif
/setalert clear EUR    → hapus alert EUR
```

### 3. Keyboard helper `AlertPairSelector()`

Tambah di keyboard.go, tampilkan 8 major currencies sebagai toggle.
State pair yang sudah di-alert tampil dengan ✅.

### 4. Update COT scheduler untuk check PairAlerts

Di scheduler yang push COT alerts:
```go
for _, pa := range prefs.PairAlerts {
    if !pa.Enabled { continue }
    // Compare current conviction vs cached last conviction
    // If delta > pa.ConvictionDelta OR (pa.BiasFlip && bias changed) → send alert
}
```

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `/setalert EUR` → save PairAlert untuk EUR di prefs
- [ ] `/setalert list` → tampilkan active pair alerts
- [ ] COT scheduler membaca `PairAlerts` dan kirim alert jika kriteria terpenuhi
- [ ] `/setalert clear EUR` → hapus alert EUR

## File yang Kemungkinan Diubah
- `internal/domain/prefs.go`
- `internal/adapter/telegram/handler.go`
- `internal/adapter/telegram/keyboard.go`
- `internal/scheduler/` (COT alert scheduler)
