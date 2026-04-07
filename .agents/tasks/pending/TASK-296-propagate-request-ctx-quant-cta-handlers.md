# TASK-296: Propagate Request Context ke fetchMultiAssetCloses & generateCTAChart

**Priority:** medium
**Type:** bug-fix
**Estimated:** S
**Area:** internal/adapter/telegram/handler_quant.go, handler_cta.go
**Created by:** Research Agent
**Created at:** 2026-04-02 10:00 WIB

## Deskripsi

Dua fungsi di adapter Telegram menggunakan `context.Background()` lokal alih-alih menerima ctx dari caller:

**1. `handler_quant.go:484` — `fetchMultiAssetCloses()`**
```go
func (h *Handler) fetchMultiAssetCloses(excludeSymbol string, tf string) (map[string][]quantAssetClose, error) {
    ctx := context.Background()  // ← SALAH: tidak pakai request ctx
    ...
    records, err := h.quant.DailyPriceRepo.GetDailyHistory(ctx, ...)
```

**2. `handler_cta.go:581` — `generateCTAChart()`**
```go
func (h *Handler) generateCTAChart(state *ctaState, timeframe string) ([]byte, error) {
    ctx := context.Background()  // ← SALAH: tidak pakai request ctx
    bars, ok := state.bars[timeframe]
```

Karena menggunakan `context.Background()`:
- Jika user Telegram membatalkan request, atau koneksi timeout, operasi tetap berjalan penuh
- DB reads tidak bisa di-cancel saat bot melakukan restart
- Saat load spike, banyak goroutines concurrent yang tidak bisa di-stop

## Perubahan yang Diperlukan

### 1. `handler_quant.go` — Tambah ctx parameter

```go
// SEBELUM:
func (h *Handler) fetchMultiAssetCloses(excludeSymbol string, tf string) (map[string][]quantAssetClose, error) {
    ctx := context.Background()

// SESUDAH:
func (h *Handler) fetchMultiAssetCloses(ctx context.Context, excludeSymbol string, tf string) (map[string][]quantAssetClose, error) {
    // gunakan ctx yang diterima, tidak perlu deklarasi baru
```

Semua caller di handler_quant.go yang memanggil `fetchMultiAssetCloses(...)` harus diupdate: tambahkan `ctx` sebagai arg pertama.

### 2. `handler_cta.go` — Tambah ctx parameter

```go
// SEBELUM:
func (h *Handler) generateCTAChart(state *ctaState, timeframe string) ([]byte, error) {
    ctx := context.Background()

// SESUDAH:
func (h *Handler) generateCTAChart(ctx context.Context, state *ctaState, timeframe string) ([]byte, error) {
    // ctx diterima dari caller, hapus deklarasi ctx lokal
```

Semua caller `generateCTAChart(...)` harus diupdate.

## File yang Harus Diubah

1. `internal/adapter/telegram/handler_quant.go`
   - Signature `fetchMultiAssetCloses` + semua caller
   - Hapus `ctx := context.Background()` di fungsi tersebut
2. `internal/adapter/telegram/handler_cta.go`
   - Signature `generateCTAChart` + semua caller (`getCTAChart` wrapper)
   - Hapus `ctx := context.Background()` di fungsi tersebut

## Verifikasi

```bash
# Pastikan tidak ada context.Background() selain yang memang intentional
grep -n "context\.Background()" internal/adapter/telegram/handler_quant.go
grep -n "context\.Background()" internal/adapter/telegram/handler_cta.go

# Build clean
go build ./...
```

## Acceptance Criteria

- [ ] `fetchMultiAssetCloses` menerima `ctx context.Context` sebagai parameter pertama
- [ ] `generateCTAChart` menerima `ctx context.Context` sebagai parameter pertama
- [ ] Semua caller diupdate agar pass request ctx
- [ ] Tidak ada lagi `ctx := context.Background()` di kedua fungsi tersebut
- [ ] `go build ./...` clean
- [ ] Fungsi masih berjalan normal (tidak ada perubahan behavior)

## Referensi

- `.agents/research/2026-04-02-10-codebase-bug-analysis-putaran21.md` — BUG-2
- `internal/adapter/telegram/handler_quant.go:484` — fetchMultiAssetCloses
- `internal/adapter/telegram/handler_cta.go:581` — generateCTAChart
