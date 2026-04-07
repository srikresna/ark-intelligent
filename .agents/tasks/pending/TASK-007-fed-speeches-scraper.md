# TASK-007: Fed Speeches Scraper via Firecrawl

**Priority:** high
**Type:** data
**Estimated:** M
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-01 00:00 WIB
**Siklus:** Data

## Deskripsi
Buat scraper untuk Fed speeches dari `federalreserve.gov/newsevents/speech/` menggunakan Firecrawl (sudah ada key di .env). Ambil 5 speech terbaru: tanggal, speaker, judul, dan excerpt/summary. Integrasikan ke `UnifiedOutlookData` sebagai context tambahan untuk AI analysis.

DATA_SOURCES_AUDIT.md secara eksplisit mencantumkan ini sebagai peluang: "Fed speeches scraper via Firecrawl → input ke AI analysis".

## Konteks
Fed communication adalah salah satu driver terbesar untuk USD dan seluruh forex market. Saat ini AI tidak punya info Fed communication terbaru — hanya ada FRED data (yield curve, inflation angka), tapi bukan narrative dari Fed officials.

Dengan Fed speeches:
- AI bisa bedakan antara hawkish vs dovish tone terbaru
- Konteks tambahan: siapa yang bicara (Powell, Waller, Bostic = weight berbeda)
- Input narrative ke unified outlook prompt

Firecrawl key sudah tersedia. federalreserve.gov adalah public source tanpa auth.

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Buat package baru `internal/service/fed/` dengan file `speeches.go`
- [ ] Struct `FedSpeech`: Date string, Speaker string, Title string, URL string
- [ ] Struct `FedSpeechData`: Speeches []FedSpeech, FetchedAt time.Time, Available bool
- [ ] Fungsi `FetchFedSpeeches(ctx context.Context) (*FedSpeechData, error)` menggunakan Firecrawl JSON extraction dari `https://www.federalreserve.gov/newsevents/speech/`
- [ ] Jika `FIRECRAWL_API_KEY` tidak di-set → return Available=false, no error
- [ ] `UnifiedOutlookData` di `unified_outlook.go` ditambah field `FedSpeeches *fed.FedSpeechData`
- [ ] `BuildUnifiedOutlookPrompt()` menampilkan FedSpeeches jika tersedia (section baru setelah MARKET SENTIMENT)
- [ ] Firecrawl schema mengextract: title, speaker, date, dan jika ada: excerpt/intro paragraph pertama

## File yang Kemungkinan Diubah
- `internal/service/fed/speeches.go` (baru)
- `internal/service/ai/unified_outlook.go` (UnifiedOutlookData + BuildUnifiedOutlookPrompt)
- `internal/adapter/telegram/handler.go` (inject FedSpeeches ke handler yang build outlook — cek cmdCTA/cmdOutlook)

## Referensi
- `.agents/research/2026-04-01-00-data-integrasi-baru.md`
- `.agents/DATA_SOURCES_AUDIT.md` (section "PELUANG MANFAATKAN FIRECRAWL LEBIH LANJUT")
- `internal/service/sentiment/cboe.go` (contoh pola Firecrawl integration)
- `internal/service/sentiment/sentiment.go` (contoh fetchAAIISentiment — pattern yang sama)
