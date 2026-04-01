# TASK-006: CBOE Put/Call Ratios Masuk ke Unified Outlook AI Prompt

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/service/ai
**Created by:** Research Agent
**Created at:** 2026-04-01 00:00 WIB
**Siklus:** Data

## Deskripsi
CBOE Put/Call ratio data sudah di-fetch di `SentimentData` (field `PutCallTotal`, `PutCallEquity`, `PutCallIndex`, `PutCallSignal`, `PutCallAvailable`), dan sudah ditampilkan di `/sentiment` command. TAPI data ini **tidak dimasukkan** ke prompt AI di `unified_outlook.go` Section 6 (MARKET SENTIMENT). Akibatnya AI tidak punya data options market sentiment saat generate unified outlook.

Task: tambahkan CBOE Put/Call ke section MARKET SENTIMENT di `BuildUnifiedOutlookPrompt()`.

## Konteks
CBOE Put/Call ratio adalah indikator contrarian yang penting:
- `total P/C > 1.0` → fear (contrarian bullish)
- `total P/C < 0.7` → complacency (contrarian bearish)

Data ini diambil via Firecrawl setiap kali `/sentiment` dipanggil, tapi tidak pernah sampai ke AI context saat analisis unified outlook. Gap ini menyebabkan AI analysis incomplete padahal data sudah tersedia.

Lihat riset: `.agents/research/2026-04-01-00-data-integrasi-baru.md` — GAP 1.

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] `BuildUnifiedOutlookPrompt()` menyertakan CBOE Put/Call data di section MARKET SENTIMENT jika `sd.PutCallAvailable == true`
- [ ] Format: `CBOE P/C: Total=X.XX Equity=X.XX Index=X.XX Signal=<SIGNAL>`
- [ ] Tidak ada perubahan perilaku lain — hanya append ke section yang sudah ada

## File yang Kemungkinan Diubah
- `internal/service/ai/unified_outlook.go` (section 6, sekitar baris 268-282)

## Referensi
- `.agents/research/2026-04-01-00-data-integrasi-baru.md`
- `internal/service/sentiment/sentiment.go` (SentimentData struct)
- `internal/service/sentiment/cboe.go` (ClassifyPutCallSignal)
- `internal/adapter/telegram/formatter.go:3682` (contoh format tampilan)
