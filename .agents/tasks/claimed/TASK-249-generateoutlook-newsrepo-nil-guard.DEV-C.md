# TASK-249: BUG-005 — handler.go: `generateOutlook` memanggil `h.newsRepo` tanpa nil guard

**Priority:** low
**Type:** bugfix
**Estimated:** XS
**Area:** internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 23:00 WIB

## Deskripsi

Fungsi `generateOutlook` (baris 890) memanggil `h.newsRepo.GetByWeek()` di baris 912 tanpa mengecek apakah `h.newsRepo != nil`:

```go
func (h *Handler) generateOutlook(...) error {
    // ...
    now := timeutil.NowWIB()

    cotAnalyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)  // baris 909

    weekEvts, _ := h.newsRepo.GetByWeek(ctx, now.Format("20060102"))  // baris 912 — BUG: no nil check!
```

Di tempat lain di handler yang sama, pola yang benar sudah digunakan:
```go
// baris 754–755 (cmdCOT detail):
if editMsgID == 0 && h.newsRepo != nil {
    ...
    todayEvts, _ := h.newsRepo.GetByDate(ctx, today)
```

Saat ini `newsRepo` selalu di-set di `main.go`, namun:
1. Inkonsisten dengan pattern nil guard di handler yang sama
2. Latent panic jika setup berubah (e.g., test dengan partial Handler)
3. Sama dengan pattern `h.cotRepo` di baris 909 yang juga unguarded

## File yang Harus Diubah

- `internal/adapter/telegram/handler.go`
  - `generateOutlook()` sekitar baris 909–912
  - Wrap akses `h.newsRepo` dan `h.cotRepo` dengan nil guard

## Implementasi

### Sebelum (handler.go:~909):
```go
// COT
cotAnalyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)

// News
weekEvts, _ := h.newsRepo.GetByWeek(ctx, now.Format("20060102"))
```

### Sesudah:
```go
// COT
var cotAnalyses []domain.COTAnalysis
if h.cotRepo != nil {
    cotAnalyses, _ = h.cotRepo.GetAllLatestAnalyses(ctx)
}

// News
var weekEvts []domain.NewsEvent
if h.newsRepo != nil {
    weekEvts, _ = h.newsRepo.GetByWeek(ctx, now.Format("20060102"))
}
```

**Catatan:** Cek seluruh body `generateOutlook` untuk akses `h.cotRepo` dan `h.newsRepo` lainnya yang mungkin juga unguarded, dan tambahkan nil guard yang konsisten.

## Acceptance Criteria

- [ ] Semua akses `h.newsRepo` di `generateOutlook` dilindungi nil guard
- [ ] Semua akses `h.cotRepo` di `generateOutlook` dilindungi nil guard
- [ ] Tidak ada perubahan behavior (ketika keduanya non-nil)
- [ ] `go build ./...` sukses

## Referensi

- `.agents/research/2026-04-02-23-bug-hunt-putaran11.md` — BUG-005
- `internal/adapter/telegram/handler.go:912` — lokasi bug
- `internal/adapter/telegram/handler.go:754` — contoh pattern nil guard yang benar
