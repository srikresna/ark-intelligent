# TASK-299: Fix Python Subprocess Temp File Cleanup on SIGKILL (Quant & VP)

**Priority:** medium
**Type:** bug-fix
**Estimated:** S
**Area:** internal/adapter/telegram/handler_quant.go, handler_vp.go
**Created by:** Research Agent
**Created at:** 2026-04-02 10:00 WIB

## Deskripsi

Dua handler yang menjalankan Python subprocess (`quant_engine.py` dan `vp_engine.py`) memiliki masalah:

### 1. Temp Files Tidak Dibersihkan Saat Timeout

```go
// handler_quant.go
cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath, chartPath)
if err := cmd.Run(); err != nil {
    os.Remove(chartPath)  // ← cleanup chart
    return nil, fmt.Errorf("quant engine failed: %w", err)
}
```

Ketika context timeout terjadi, `cmd.Run()` return error (`context.DeadlineExceeded`). Kode memang memanggil `os.Remove(chartPath)`, tapi **tidak menghapus `inputPath` dan `outputPath`**. Padahal `defer os.Remove(inputPath)` dan `defer os.Remove(outputPath)` sudah ada di awal fungsi. Masalahnya hanya pada chartPath cleanup saat error.

Lebih serius: ketika context expired, Go kirim **SIGKILL** ke Python process. Python yang di-SIGKILL tidak sempat menulis `outputPath` dengan sempurna. Jika ada partial write, `os.Remove` tidak dipanggil sebelum `os.ReadFile` — tapi ini tidak relevan karena error dari `cmd.Run()` menyebabkan early return.

### 2. Parent Context Tidak Di-Pass (Dari TASK-296, Dampak ke Sini)

Kedua fungsi menggunakan `context.Background()` sebagai parent untuk subprocess timeout:
```go
cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
```

Akibatnya jika Telegram request sudah expired/cancelled, Python process tetap berjalan penuh 60/90 detik.

### 3. Chartpath Cleanup Tidak Konsisten

Di `handler_quant.go`, chartPath di-cleanup hanya pada beberapa error path tapi tidak di semua path. Jika `os.ReadFile(outputPath)` gagal setelah Python sukses tapi chart sudah terbuat, chartPath tidak dihapus.

## Perubahan yang Diperlukan

### Fix 1: Konsistenkan Temp File Cleanup dengan `defer`

```go
func (h *Handler) runQuantEngine(inputPath, outputPath, chartPath string) (*quantEngineResult, error) {
    // Cleanup guaranteed via defer (regardless of error path)
    defer os.Remove(inputPath)
    defer os.Remove(outputPath)
    defer os.Remove(chartPath)  // ← Pindahkan ke defer, bukan di error paths
    
    // CATATAN: jika chartPath adalah output yang ingin disimpan, 
    // hapus defer chartPath dan gunakan rename/copy approach
    
    scriptPath := findQuantScript()
    cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    ...
}
```

**Tapi perhatian:** di kode existing, `chartPath` adalah file yang INGIN DISIMPAN dan path-nya dikembalikan di `result.ChartPath`. Jadi `defer os.Remove(chartPath)` hanya boleh dilakukan jika error terjadi.

**Solusi yang lebih tepat:**

```go
func (h *Handler) runQuantEngine(inputPath, outputPath, chartPath string) (result *quantEngineResult, err error) {
    defer os.Remove(inputPath)
    defer os.Remove(outputPath)
    
    // Cleanup chartPath hanya jika error
    defer func() {
        if err != nil {
            os.Remove(chartPath)
        }
    }()
    
    scriptPath := findQuantScript()
    cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath, chartPath)
    
    if runErr := cmd.Run(); runErr != nil {
        return nil, fmt.Errorf("quant engine failed: %w", runErr)
    }
    
    // ... read outputPath, return result
}
```

### Fix 2: Hapus Inline `os.Remove(chartPath)` yang Redundant

Setelah named return + defer di atas, hapus semua `os.Remove(chartPath)` yang tersebar di error paths.

## File yang Harus Diubah

1. `internal/adapter/telegram/handler_quant.go` — fungsi yang jalankan Python (sekitar line 430-480)
2. `internal/adapter/telegram/handler_vp.go` — fungsi yang jalankan VP engine (sekitar line 405-450)

## Verifikasi

```bash
go build ./...

# Manual test: buat test request ke /quant atau /vp command
# Pastikan tidak ada leftover file di /tmp atau working dir setelah:
# 1. Sukses
# 2. Python script error
# 3. Timeout (simulasi dengan set timeout sangat kecil)
```

## Acceptance Criteria

- [ ] Temp files (`inputPath`, `outputPath`) selalu dibersihkan via defer
- [ ] `chartPath` dibersihkan hanya jika error (tidak jika sukses — file dikembalikan ke caller)
- [ ] Tidak ada inline `os.Remove(chartPath)` yang tersebar di error paths
- [ ] Named return digunakan agar defer bisa cek error state
- [ ] `go build ./...` clean
- [ ] Behavior sukses tidak berubah (chart path tetap dikembalikan di result)

## Referensi

- `.agents/research/2026-04-02-10-codebase-bug-analysis-putaran21.md` — BUG-7
- `internal/adapter/telegram/handler_quant.go:440-480` — Python subprocess quant
- `internal/adapter/telegram/handler_vp.go:422-450` — Python subprocess VP
- TASK-296 — fix ctx propagation (related: setelah TASK-296 selesai, ganti context.Background() dengan ctx dari parameter)
