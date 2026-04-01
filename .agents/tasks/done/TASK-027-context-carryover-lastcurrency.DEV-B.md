# TASK-027: Context Carry-Over — LastCurrency di UserPrefs

**Priority:** MEDIUM  
**Siklus:** 1 (UX/UI)  
**Effort:** Small (< 1 hari)  
**File Utama:** `internal/domain/prefs.go`, `internal/adapter/telegram/handler.go`

---

## Problem

Ketika user melihat `/cot EUR` lalu ketik `/cta`, sistem tidak tahu user mau EUR.  
User harus ketik `/cta EUR` secara eksplisit setiap kali.  
`CalendarFilter` dan `CalendarView` sudah disimpan per-user di prefs — pola ini bisa dipakai untuk currency.

## Solution

### 1. Tambah `LastCurrency` field di `UserPrefs`
```go
// domain/prefs.go
type UserPrefs struct {
    // ... existing fields ...
    LastCurrency string `json:"last_currency,omitempty"` // Last viewed currency (EUR, USD, GBP, etc.)
}
```

### 2. Simpan LastCurrency setiap kali user pilih currency
Di semua handler yang menerima currency arg (COT, CTA, Quant, VP, Price, Levels, Seasonal):
```go
// Setelah parse currency arg:
if currency != "" {
    prefs, _ := h.prefsRepo.Get(ctx, userID)
    prefs.LastCurrency = currency
    _ = h.prefsRepo.Set(ctx, userID, prefs)
}
```

### 3. Fallback ke LastCurrency jika args kosong
Di command yang support currency arg:
```go
if code == "" {
    prefs, _ := h.prefsRepo.Get(ctx, userID)
    if prefs.LastCurrency != "" {
        code = prefs.LastCurrency
        // Tampilkan notifikasi kecil: "Using last currency: EUR"
    }
}
```

### 4. Tambah Button "🔄 Same as last: EUR" di Currency Selector Keyboard
Di `COTCurrencySelector`, `CTACurrencySelector`, dsb — jika `LastCurrency` diketahui, tambah shortcut button di baris pertama.

## Acceptance Criteria
- [ ] `UserPrefs.LastCurrency` tersimpan setelah pilih currency apapun
- [ ] `/cta` tanpa arg → fallback ke LastCurrency jika ada
- [ ] `/quant` tanpa arg → fallback ke LastCurrency
- [ ] `/price` tanpa arg → fallback ke LastCurrency
- [ ] Button "🔄 Same as last: XXX" muncul di currency selector (COT, CTA, Quant)
- [ ] Default `LastCurrency` = "" (tidak ada fallback jika belum pernah pilih)
- [ ] Test: `/cot EUR` → `/cta` → harusnya tampilkan CTA EUR

## Notes
- Tidak perlu backward migration — field optional, default kosong
- Hanya simpan jika currency valid (ada dalam daftar supported currencies)
- Jangan override jika user ketik currency secara eksplisit
