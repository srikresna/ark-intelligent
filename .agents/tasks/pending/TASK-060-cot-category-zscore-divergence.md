# TASK-060: COT Category ZScore + Divergence Signal

**Priority:** HIGH
**Cycle:** Siklus 3 — Fitur Baru
**Estimated Complexity:** MEDIUM
**Research Ref:** `.agents/research/2026-04-01-02-fitur-baru-siklus3-lanjutan.md`

---

## Deskripsi

Tambahkan per-category ZScore analysis ke COT engine. Saat ini hanya `AssetMgrZScore` yang ada — tambahkan ZScore untuk semua kategori positioning (Dealer, LevFund, ManagedMoney, SwapDealer) dan buat cross-category divergence signal.

## Konteks Teknis

### Data Yang Sudah Ada
- `internal/domain/cot.go`: `DealerLong/Short`, `AssetMgrLong/Short`, `LevFundLong/Short`, `ManagedMoneyLong/Short`, `SwapDealerLong/Short` sudah ada di `COTRecord`
- `internal/service/cot/analyzer.go`: `computeAssetMgrZScore()` sudah ada di fungsi `computeMetrics()` sekitar line 392-413
- `internal/domain/cot.go`: `COTAnalysis` sudah punya `AssetMgrZScore`, `AssetMgrAlert`

### Files Yang Perlu Dimodifikasi

**1. `internal/domain/cot.go` — tambah fields ke `COTAnalysis`:**
```go
// Category Z-Scores (per category WoW change vs 52W mean/stddev)
DealerZScore       float64 `json:"dealer_z_score"`
DealerAlert        bool    `json:"dealer_alert"`
LevFundZScore      float64 `json:"lev_fund_z_score"`
LevFundAlert       bool    `json:"lev_fund_alert"`
ManagedMoneyZScore float64 `json:"managed_money_z_score"`
ManagedMoneyAlert  bool    `json:"managed_money_alert"`
SwapDealerZScore   float64 `json:"swap_dealer_z_score"`
SwapDealerAlert    bool    `json:"swap_dealer_alert"`

// Cross-category divergence
CategoryDivergence     bool   `json:"category_divergence"`      // true if significant divergence detected
CategoryDivergenceDesc string `json:"category_divergence_desc"` // human-readable description
```

**2. `internal/service/cot/analyzer.go` — buat file baru `category_zscore.go`:**
```go
// computeCategoryZScore computes a Z-score for a given category's
// WoW net position change vs 52-week historical distribution.
// history is newest-first, index 0 = current week.
func computeCategoryZScore(getLong, getShort func(r domain.COTRecord) float64, history []domain.COTRecord) (zscore float64, alert bool) {
    if len(history) < 4 {
        return 0, false
    }
    
    // Compute WoW changes for each week
    changes := make([]float64, 0, len(history)-1)
    for i := 1; i < len(history); i++ {
        currNet := getLong(history[i-1]) - getShort(history[i-1])
        prevNet := getLong(history[i]) - getShort(history[i])
        changes = append(changes, currNet-prevNet)
    }
    
    if len(changes) < 3 {
        return 0, false
    }
    
    // Mean and StdDev of historical changes
    currentChange := changes[0]
    historicalChanges := changes[1:]
    avg, stdDev := meanStdDev(historicalChanges)
    
    if stdDev < 1e-9 {
        return 0, false
    }
    
    zscore = (currentChange - avg) / stdDev
    alert = math.Abs(zscore) >= 2.0
    return zscore, alert
}
```

**3. Divergence Signal Logic:**
```go
// detectCategoryDivergence identifies significant divergences between categories.
// Example: LevFunds extreme long (>2σ) while Dealers extreme short (<-2σ) = crowded long setup
func detectCategoryDivergence(a *domain.COTAnalysis) {
    // Pattern 1: LevFund vs Dealer divergence (crowding signal)
    if a.LevFundZScore > 2.0 && a.DealerZScore < -1.5 {
        a.CategoryDivergence = true
        a.CategoryDivergenceDesc = "CROWDED LONG: LevFunds accumulating (+{:.1f}σ), Dealers distributing ({:.1f}σ) — reversal risk"
    } else if a.LevFundZScore < -2.0 && a.DealerZScore > 1.5 {
        a.CategoryDivergence = true
        a.CategoryDivergenceDesc = "CROWDED SHORT: LevFunds heavy short, Dealers covering — squeeze risk"
    }
    
    // Pattern 2: AssetMgr vs LevFund divergence (momentum vs institutional)
    if math.Abs(a.AssetMgrZScore-a.LevFundZScore) > 3.0 {
        // High divergence between momentum and institutional
        if a.AssetMgrZScore > 1.0 && a.LevFundZScore < -1.0 {
            a.CategoryDivergence = true
            a.CategoryDivergenceDesc = "INSTITUTIONAL ACCUMULATING vs FUNDS SELLING — potential trend change"
        }
    }
}
```

**4. Update /cot formatter** (`internal/adapter/telegram/formatter.go` atau handler_cot.go):
- Tampilkan per-category breakdown saat ada divergence atau alert
- Format tambahan di /cot output:
```
📊 Category Breakdown:
  • LevFund: +{zscore:.1f}σ {alert_emoji}
  • AssetMgr: {zscore:.1f}σ
  • Dealer: {zscore:.1f}σ
⚠️ DIVERGENCE: LevFunds accumulating, Dealers distributing
```

## Acceptance Criteria
- [ ] `DealerZScore`, `LevFundZScore`, `ManagedMoneyZScore`, `SwapDealerZScore` computed untuk semua contracts
- [ ] `CategoryDivergence` + `CategoryDivergenceDesc` detected dan populated
- [ ] /cot output menampilkan per-category breakdown jika ada alert/divergence
- [ ] Unit tests untuk `computeCategoryZScore()` dengan mock history data
- [ ] Tidak ada breaking change ke existing COTAnalysis fields

## Notes
- Untuk TFF contracts (FX, bonds): use Dealer, AssetMgr, LevFund
- Untuk DISAGGREGATED (metals, energy): use SwapDealer, ManagedMoney, ProdMerc
- Check `COTRecord.ReportType` untuk menentukan kategori yang relevan
