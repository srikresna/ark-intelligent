# TASK-309: /sentiment Source Availability Diagnostic Footer

**Priority:** low
**Type:** ux-improvement
**Estimated:** S
**Area:** internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-04 01:00 WIB
**Siklus:** Data-2 (Siklus 2 Putaran 23)
**Ref:** research/2026-04-04-01-data-sources-audit-naaim-coingecko-global-sentiment-ux-putaran23.md

## Deskripsi

Ketika `/sentiment` gagal mengambil salah satu sumber (misal CBOE P/C karena Firecrawl timeout, atau AAII karena scraping blokir), **section tersebut hilang tanpa penjelasan**. User tidak tahu apakah data memang tidak ada atau sedang error.

**Contoh problem:**
- CBOE Firecrawl timeout → seluruh section CBOE tidak muncul
- AAII scraping blocked → section AAII kosong
- User bertanya: "kenapa tidak ada data CBOE hari ini?"

**Target output dengan fix:**
```
[... section sentiment normal ...]

📡 Sumber: CNN ✅ AAII ✅ CBOE ❌ CryptoFG ✅ VIX ✅
⚠️ CBOE Put/Call tidak tersedia (coba lagi nanti)
Data: 3 menit lalu
```

## Implementasi

Semua logic ada di `FormatSentiment()` di `formatter.go`. Pure display change, tidak perlu ubah domain atau service.

### Fungsi helper untuk footer

```go
func buildSentimentDiagnosticFooter(data *sentiment.SentimentData) string {
    type sourceStatus struct {
        name      string
        available bool
    }

    sources := []sourceStatus{
        {"CNN", data.CNNAvailable},
        {"AAII", data.AAIIAvailable},
        {"CBOE", data.PutCallAvailable},
        {"CryptoFG", data.CryptoFearGreedAvailable},
        {"VIX", data.VIXAvailable},
    }

    // NAAIM — hanya tampilkan kalau TASK-305 sudah implement
    // sources = append(sources, sourceStatus{"NAAIM", data.NAAIMAvailable})

    available := 0
    statusParts := make([]string, 0, len(sources))
    missing := make([]string, 0)

    for _, s := range sources {
        if s.available {
            available++
            statusParts = append(statusParts, s.name+"✅")
        } else {
            statusParts = append(statusParts, s.name+"❌")
            missing = append(missing, s.name)
        }
    }

    var sb strings.Builder

    // Show footer only if at least one source is missing
    if available < len(sources) {
        sb.WriteString(fmt.Sprintf("\n<i>📡 Sumber: %s</i>\n", strings.Join(statusParts, " ")))
        if len(missing) > 0 {
            sb.WriteString(fmt.Sprintf("<i>⚠️ %s tidak tersedia (coba beberapa saat lagi)</i>\n",
                strings.Join(missing, ", ")))
        }
    }

    // Always show data age
    if !data.FetchedAt.IsZero() {
        age := fmtDataAge(data.FetchedAt)  // dari TASK-307
        sb.WriteString(fmt.Sprintf("<i>Data: %s</i>\n", age))
    }

    return sb.String()
}
```

### Tambah ke FormatSentiment() di akhir function

```go
// Di akhir FormatSentiment():
b.WriteString(buildSentimentDiagnosticFooter(data))

return b.String()
```

## Catatan: Dependency dengan TASK-307

TASK-307 dan TASK-309 keduanya menyentuh footer `/sentiment`. Koordinasi:
- TASK-307 (Data Age): Tambah `fmtDataAge()` helper dan tampilkan di footer
- TASK-309 (Diagnostics): Tampilkan source status + gunakan `fmtDataAge()` dari TASK-307

Jika keduanya diimplementasi bersama (atau TASK-307 setelah TASK-309), pastikan tidak duplikasi footer.

**Jika TASK-307 belum done saat implement ini:** Inlinekan `fmtDataAge()` langsung di TASK-309 (duplikasi kecil yang bisa di-cleanup nanti).

## Acceptance Criteria

- [ ] Ketika semua 5 sumber tersedia → **tidak ada** footer diagnostics (clean output)
- [ ] Ketika ≥1 sumber unavailable → footer diagnostics muncul dengan status per-source
- [ ] Footer mencantumkan nama sumber yang tidak tersedia
- [ ] Data age selalu ditampilkan (menggunakan FetchedAt)
- [ ] Format rapi, menggunakan `<i>...</i>` untuk italic (Telegram HTML)
- [ ] `go build ./...` clean

## Referensi

- `internal/adapter/telegram/formatter.go` — FormatSentiment() function
- `internal/service/sentiment/sentiment.go` — Available flags per source
- TASK-307 — `fmtDataAge()` helper (bisa share atau inlinekan)
- research/2026-04-04-01-data-sources-audit-naaim-coingecko-global-sentiment-ux-putaran23.md
