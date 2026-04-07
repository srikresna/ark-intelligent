# Research Report: Bug Hunting & Edge Cases
**Siklus:** 5 — Bug Hunting & Edge Cases
**Tanggal:** 2026-04-01
**Agent:** Research

---

## Metodologi

Analisis statis codebase (~213 file Go) fokus pada:
- Goroutine lifecycle & concurrency safety
- Nil pointer dereferences & index out-of-bounds
- Error swallowing & missing propagation
- Edge cases di input validation & data parsing
- HTML rendering & message chunking bugs

---

## Temuan Kritis

### BUG-1: Unbounded Goroutine Spawning di Polling Loop (MEDIUM)
**File:** `internal/adapter/telegram/bot.go` — `StartPolling()`

```go
for _, update := range updates {
    b.offset = update.UpdateID + 1
    go b.handleUpdate(ctx, update) // No semaphore, no worker pool
}
```

Telegram `getUpdates` returns up to 100 updates per call. Under spam/flood conditions:
- 100 updates/call × rapid repeat = ratusan goroutine concurrent
- Setiap goroutine bisa trigger AI call (30-120 detik response time)
- Goroutine pile-up → memory pressure → OOM risk pada VPS kecil
- **Fix:** Worker pool dengan semaphore (e.g., buffer channel 20 slots)

---

### BUG-2: Fibonacci Levels Invalid Ketika Market Flat (LOW-MEDIUM)
**File:** `internal/service/ta/fibonacci.go` — `CalcFibonacci()`

```go
diff := swingHigh - swingLow
// diff bisa = 0 jika swingHigh == swingLow (flat/choppy market)
// Semua level akan identik — output misleading tapi tidak crash
for key, ratio := range fibRatios {
    levels[key] = swingHigh - ratio*diff  // 0 * 0 = 0, semua sama
}
```

Saat market sangat choppy atau data OHLC sama (e.g., synthetic/test data):
- swingHigh == swingLow → diff = 0 → semua Fibonacci level identik
- `NearestLevel` memilih level acak (semua dist=0 → minDist tetap MaxFloat64 → nearestLevel tetap "")
- **Fix:** Early return nil jika diff < threshold (0.0001% dari price)

---

### BUG-3: detectUnclosedTags Gagal Handle Tag Dengan Attributes (LOW)
**File:** `internal/adapter/telegram/bot.go` — `detectUnclosedTags()`

```go
name := tag
if spIdx := strings.IndexByte(name, ' '); spIdx > 0 {
    name = name[:spIdx]
}
```

Attribute stripping hanya strip space pertama. Edge cases:
- `<pre\n>` (newline sebelum `>`) — gagal strip → tag masuk sebagai "pre\n"
- `<code\t>` — tab delimiter → gagal
- Nested closing tags yang tidak match stack: `</b>` saat stack kosong → stack tidak ter-pop dengan benar (sudah ada guard `len(stack) > 0` tapi hanya check top, bukan seluruh stack — mismatched close tag bisa corrupt state)
- **Impact:** Pesan terpotong dengan unclosed HTML → Telegram parse error → pesan gagal terkirim

---

### BUG-4: News Alert Goroutine Menggunakan Outer Context Yang Bisa Cancelled (LOW)
**File:** `internal/service/news/scheduler.go` line 676

```go
go func() {
    defer recover()...
    s.impactRecorder.RecordImpact(ctx, ev, ...) // ctx dari outer loop
}()
```

Jika bot shutdown saat loop sedang berjalan:
- `ctx` ter-cancel → `RecordImpact` internal HTTP calls gagal
- Price impact tidak ter-record → gap di impact history database
- **Fix:** Gunakan `context.WithoutCancel(ctx)` atau detached background context dengan timeout terpisah (e.g., 30s)

---

### BUG-5: Callback Handler — Empty chatID Tidak Diproteksi Sepenuhnya (LOW)
**File:** `internal/adapter/telegram/bot.go` — `handleCallback()`

```go
chatID := ""
if cb.Message != nil {
    chatID = strconv.FormatInt(cb.Message.Chat.ID, 10)
    msgID = cb.Message.MessageID
}
// chatID bisa "" jika cb.Message == nil (inline query dari channel)
// Handler dipanggil dengan chatID="" → fallback ke defaultID
```

Callback dari inline keyboard yang di-forward ke channel bisa punya `Message == nil`. Handlers yang menggunakan `chatID` kosong akan menulis ke `defaultID` (owner chat). Ini bisa **leak conversation data ke owner** atau spam owner chat.
- **Fix:** Return early dengan `AnswerCallback` error jika chatID kosong setelah fallback check

---

### BUG-6: COT IndexRateOfChange — Accesses weeklyIndices[2] Tanpa Explicit Guard (LOW)
**File:** `internal/service/cot/index.go` — `ComputeIndexROC()`

```go
if len(weeklyIndices) < 5 {
    return nil
}
// len >= 5 guaranteed, tapi prevROC1W mengakses [1] dan [2]
prevROC1W := weeklyIndices[1] - weeklyIndices[2]  // len >= 5 → [2] safe
```

Saat ini aman karena guard `< 5` memastikan `len >= 5`, tetapi jika guard diubah jadi `< 3` di masa depan (intentionally) untuk ROC1W saja, akses `[4]` di bawahnya akan panic. **Latent risk**, bukan bug aktif.

---

## Gap Coverage Test

### Area Tanpa Test
1. `fibonacci.go` — zero `diff` edge case tidak ada test
2. `bot.go` `splitMessage` / `detectUnclosedTags` — tidak ada unit test untuk malformed HTML
3. `bot.go` `StartPolling` — tidak ada test untuk goroutine cleanup / graceful shutdown
4. `news/scheduler.go` goroutine impact recorder — tidak ada integration test
5. `microstructure/engine.go` — tidak ada unit test sama sekali

---

## Rekomendasi Prioritas

| Priority | Item |
|---|---|
| HIGH | Worker pool untuk polling goroutines |
| MEDIUM | Fibonacci flat-market early return |
| MEDIUM | splitMessage + detectUnclosedTags unit tests |
| LOW | Impact recorder detached context |
| LOW | Callback empty chatID guard |
