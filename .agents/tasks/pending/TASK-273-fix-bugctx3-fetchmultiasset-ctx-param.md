# TASK-273: Fix BUG-CTX3 — Tambah ctx Parameter ke fetchMultiAssetCloses

**Priority:** medium
**Type:** bugfix
**Estimated:** XS
**Area:** internal/adapter/telegram/handler_quant.go
**Created by:** Research Agent
**Created at:** 2026-04-02 24:00 WIB

## Deskripsi

`fetchMultiAssetCloses` membuat `context.Background()` sendiri alih-alih menerima parent context:

```go
func (h *Handler) fetchMultiAssetCloses(excludeSymbol string, tf string) (map[string][]quantAssetClose, error) {
    ctx := context.Background()  // ← ignores parent ctx!
    result := make(map[string][]quantAssetClose)

    for _, mapping := range domain.DefaultPriceSymbolMappings {
        ...
        records, err := h.quant.DailyPriceRepo.GetDailyHistory(ctx, mapping.ContractCode, 300)
        ...
    }
}
```

Fungsi ini dipanggil dari handler yang sudah punya parent `ctx`:
```go
// handler_quant.go:421
multiAsset, maErr := h.fetchMultiAssetCloses(state.symbol, tf)
```

Loop memfetch data untuk 20+ aset. Jika parent request dibatalkan, loop tetap berjalan sampai selesai.

## File yang Harus Diubah

### internal/adapter/telegram/handler_quant.go

**Signature lama:**
```go
func (h *Handler) fetchMultiAssetCloses(excludeSymbol string, tf string) (map[string][]quantAssetClose, error) {
    ctx := context.Background()
```

**Signature baru:**
```go
func (h *Handler) fetchMultiAssetCloses(ctx context.Context, excludeSymbol string, tf string) (map[string][]quantAssetClose, error) {
    // tidak ada ctx := context.Background() lagi
```

**Update caller di L421:**
```go
// Sebelum:
multiAsset, maErr := h.fetchMultiAssetCloses(state.symbol, tf)

// Sesudah:
multiAsset, maErr := h.fetchMultiAssetCloses(ctx, state.symbol, tf)
```

Pastikan tidak ada caller lain untuk fungsi ini (grep untuk `fetchMultiAssetCloses`).

## Verifikasi

```bash
grep -n "fetchMultiAssetCloses" internal/adapter/telegram/handler_quant.go
go build ./...
go vet ./...
```

## Acceptance Criteria

- [ ] `fetchMultiAssetCloses` menerima `ctx context.Context` sebagai parameter pertama
- [ ] `ctx := context.Background()` dihapus dari body fungsi
- [ ] Semua caller updated untuk meneruskan `ctx`
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean

## Referensi

- `.agents/research/2026-04-02-24-bug-hunt-wyckoff-context-ict-putaran16.md` — BUG-CTX3
- `internal/adapter/telegram/handler_quant.go:483` — fetchMultiAssetCloses
- `internal/adapter/telegram/handler_quant.go:421` — caller
- `.agents/TECH_REFACTOR_PLAN.md` — TECH-008 Context Propagation
