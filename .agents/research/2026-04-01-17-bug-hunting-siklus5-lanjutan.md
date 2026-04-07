# Research: Bug Hunting & Edge Cases — Siklus 5 (Lanjutan)
**Date:** 2026-04-01 17:00 WIB
**Focus:** Bug Hunting & Edge Cases (Siklus 5 — sesi kedua)

---

## Metodologi

Analisis lanjutan setelah sesi pertama (BUG-A1 s/d BUG-A7). Sesi ini fokus pada:
- `internal/service/ai/gemini.go` — konfigurasi model
- `internal/adapter/telegram/handler_cta.go` — python subprocess tanpa timeout
- `internal/adapter/telegram/handler_ctabt.go` — python subprocess tanpa timeout
- `internal/scheduler/scheduler.go` — BUG-5 wiring newsScheduler
- `internal/service/ai/gemini.go` — ctx-unaware retry sleep
- `cmd/bot/main.go` — wiring gaps

---

## Bug Baru yang Ditemukan

### BUG-B1: GEMINI_MODEL env var dikonfigurasi tapi tidak dipakai
**File:** `internal/service/ai/gemini.go:37,108` dan `internal/config/config.go:104`
**Severity:** Medium
**Deskripsi:**
```go
// config.go:104 — env var diparsed
GeminiModel: getEnv("GEMINI_MODEL", "gemini-3.1-flash-lite-preview"),

// gemini.go:37 — tapi model HARDCODED dalam constructor, tidak menggunakan cfg.GeminiModel
model := client.GenerativeModel("gemini-3.1-flash-lite-preview")

// gemini.go:108 — juga hardcoded di GenerateWithSystem
model := gc.client.GenerativeModel("gemini-3.1-flash-lite-preview")
```

`NewGeminiClient` tidak menerima parameter model name. `cfg.GeminiModel` tidak pernah di-pass ke constructor. Operator tidak bisa mengubah model Gemini via env var `GEMINI_MODEL` meskipun config sudah menyediakan field tersebut.

**Impact:**
- Dead config field — bisa menyesatkan operator yang mencoba mengubah model via env var
- Tidak bisa switch ke model Gemini baru tanpa code change
- `GenerateWithSystem` hardcodes model terpisah dari `model` field di struct, sehingga ada dua tempat yang perlu di-update jika model berubah

**Fix:**
```go
// Tambahkan parameter model ke constructor
func NewGeminiClient(ctx context.Context, apiKey, modelName string, maxRPM, maxDaily int) (*GeminiClient, error) {
    ...
    model := client.GenerativeModel(modelName)
    ...
    return &GeminiClient{..., modelName: modelName}, nil
}

// GenerateWithSystem gunakan gc.modelName
model := gc.client.GenerativeModel(gc.modelName)

// cmd/bot/main.go pass cfg.GeminiModel
gemini, err := aisvc.NewGeminiClient(ctx, cfg.GeminiAPIKey, cfg.GeminiModel, cfg.AIMaxRPM, cfg.AIMaxDaily)
```

---

### BUG-B2: runChartScript (handler_cta.go) menggunakan context.Background() tanpa timeout
**File:** `internal/adapter/telegram/handler_cta.go:745`
**Severity:** Medium-High
**Deskripsi:**
```go
func runChartScript(input interface{}) ([]byte, error) {
    ...
    cmd := exec.CommandContext(context.Background(), "python3", scriptPath, inputPath, outputPath)
    cmd.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
        return nil, fmt.Errorf("chart renderer failed: %w", err)
    }
    ...
}
```

`runChartScript` adalah fungsi standalone (bukan method) yang tidak menerima `ctx` parameter. Dipanggil dari `generateCTADetailChart` untuk mode ichimoku, fibonacci, dan zones. Tidak ada timeout sama sekali — jika Python process hang, goroutine handler ini akan hang selamanya.

Bandingkan dengan `generateCTAChart` (line 710) yang menggunakan `ctx` dari request (benar), dan `handler_quant.go` yang punya timeout 60s. `runChartScript` tidak memiliki keduanya.

**Impact:** Handler goroutine bisa hang permanen jika Python script hang, menumpuk goroutine leak.

**Fix:**
```go
func runChartScript(ctx context.Context, input interface{}) ([]byte, error) {
    ...
    cmdCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
    defer cancel()
    cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath)
    ...
}
// Update semua caller untuk pass ctx
```

---

### BUG-B3: generateCTAChart di handler_ctabt.go tanpa timeout
**File:** `internal/adapter/telegram/handler_ctabt.go:472`
**Severity:** Medium
**Deskripsi:**
```go
cmd := exec.CommandContext(context.Background(), "python3", scriptPath, inputPath, outputPath)
cmd.Stderr = os.Stderr
if err := cmd.Run(); err != nil {
    return nil, fmt.Errorf("backtest chart renderer failed: %w", err)
}
```
Sama dengan BUG-B2 — Python subprocess dijalankan dengan `context.Background()` tanpa timeout. Jika script hang, handler goroutine hang selamanya.

**Impact:** Goroutine leak, unresponsive bot untuk user yang trigger backtest chart.

**Fix:** Gunakan `context.WithTimeout(ctx, 60*time.Second)` seperti `handler_quant.go`.

---

### BUG-B4: BUG-5 — newsScheduler tidak di-wire ke scheduler.Deps untuk ConvictionScoreV3
**File:** `internal/scheduler/scheduler.go:1042-1050` dan `cmd/bot/main.go:266-285`
**Severity:** Medium
**Deskripsi:**
```go
// scheduler.go:1042
// BUG-5: newsScheduler (which holds GetSurpriseSigma) is not wired into
// the main scheduler's Deps, so we cannot retrieve the live surprise
// accumulator here. Pass 0 until Deps is extended with a NewsScheduler reference.
cs := cotsvc.ComputeConvictionScoreV3(*analysis, macroRegime, 0, "", macroData, priceCtx) // ← 0 hardcoded
```

Scheduler broadcast COT menggunakan surprise sigma = 0 karena `newsSched` tidak di-pass ke `scheduler.Deps`. Padahal `newsSched` sudah implement interface `SurpriseProvider` dan sudah di-pass ke `Handler`. Tapi di `main.go`, `sched` dibuat sebelum `newsSched`:

```go
// main.go:266 — sched dibuat dulu
sched := scheduler.New(&scheduler.Deps{...}) // newsSched belum ada di sini

// main.go:296 — newsSched dibuat belakangan
newsSched := newssvc.NewScheduler(...)
```

**Impact:** ConvictionScore pada broadcast COT release selalu menggunakan surpriseSigma=0, menyebabkan skor tidak merefeksikan surprise ekonomi terbaru. Pengguna mendapat sinyal yang kurang akurat saat broadcast.

**Fix:**
1. Tambah `SurpriseProvider` interface ke `scheduler.Deps`
2. Di `main.go`, inject `newsSched` ke `sched` setelah keduanya dibuat (gunakan setter method)
3. Atau reorder: buat `newsSched` sebelum `sched`

---

### BUG-B5: Gemini retry sleep tidak menghormati context (GenerateWithSystem)
**File:** `internal/service/ai/gemini.go:118`
**Severity:** Low-Medium
**Deskripsi:**
```go
// GenerateWithSystem — retry loop
for attempt := 0; attempt < 3; attempt++ {
    if attempt > 0 {
        time.Sleep(time.Duration(attempt*attempt) * time.Second)
    }
    ...
}
```
`Generate` (line 74) juga punya masalah yang sama. Retry backoff tidak menghormati `ctx.Done()`. Jika context timeout atau cancel saat sleeping (1s atau 4s), sleep akan berjalan penuh sebelum iterasi berikutnya yang akan langsung gagal karena ctx sudah done.

Ini berbeda dari `claude.go` yang sudah benar menggunakan `select { case <-time.After(backoff): case <-ctx.Done(): }`.

**Impact:** Delay tidak perlu saat context cancelled, user menunggu lebih lama dari seharusnya.

**Fix:**
```go
if attempt > 0 {
    select {
    case <-ctx.Done():
        return "", ctx.Err()
    case <-time.After(time.Duration(attempt*attempt) * time.Second):
    }
}
```

---

## Edge Cases Tambahan

### EDGE-5: chartPath file tidak di-defer remove di runVPEngine dan runQuantEngine
**File:** `internal/adapter/telegram/handler_vp.go` dan `handler_quant.go`
**Detail:**
- `runVPEngine` defers remove untuk `inputPath` dan `outputPath` tapi tidak `chartPath`
- `runQuantEngine` defers remove `inputPath` dan `outputPath` tapi tidak `chartPath`
- chartPath di-remove oleh caller setelah dipakai, tapi jika caller panic/exit early, chart file tidak dibersihkan

**Status:** Low impact karena `/tmp` biasanya dibersihkan OS, tapi bisa accumulate untuk bot yang long-running. **Low priority.**

### EDGE-6: aiCooldown dan chatCooldown cleanup loop bisa lambat di high-traffic
**File:** `internal/adapter/telegram/handler.go:1063-1068`
```go
if len(h.aiCooldown) > 100 {
    for uid, ts := range h.aiCooldown {
        if now.Sub(ts) > aiCooldownDuration*2 {
            delete(h.aiCooldown, uid)
        }
    }
}
```
Cleanup dijalankan saat `len > 100` tapi hanya menghapus entries yang sudah 2x cooldown duration (60s) atau lebih tua. Untuk bot dengan banyak user, map ini bisa tumbuh besar sebelum mencapai 100 threshold. **Low priority, by design.**

---

## Ringkasan Bug Baru

| Bug ID | File | Severity | Action |
|--------|------|----------|--------|
| BUG-B1 | ai/gemini.go | Medium | Fix: pass model name ke constructor, inject cfg.GeminiModel |
| BUG-B2 | handler_cta.go:745 | Medium-High | Fix: tambah ctx+timeout ke runChartScript |
| BUG-B3 | handler_ctabt.go:472 | Medium | Fix: gunakan context.WithTimeout |
| BUG-B4 | scheduler.go:1042 | Medium | Fix: wire SurpriseProvider ke scheduler.Deps |
| BUG-B5 | ai/gemini.go:74,118 | Low-Medium | Fix: ctx-aware sleep di Gemini retry |
