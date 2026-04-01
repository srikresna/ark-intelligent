# TASK-234: Integrasikan Semua Sentiment Data ke Unified Outlook AI Prompt

**Priority:** high
**Type:** refactor
**Estimated:** S
**Area:** internal/service/ai/unified_outlook.go, internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 19:00 WIB

## Deskripsi

`/outlook` menghasilkan AI analysis komprehensif. Namun saat ini AI prompt tidak mendapat:
1. VIX term structure data (sudah fetch tapi **tidak diinject** ke AI prompt)
2. CBOE Put/Call ratios (sudah di SentimentData tapi tidak diinject ke AI)
3. Crypto Fear & Greed Index (sudah di SentimentData tapi tidak diinject ke AI)
4. BIS REER data (sudah fetch tapi tidak diinject ke AI)
5. World Bank macro data (sudah fetch, masuk ke AI tapi cek apakah lengkap)

Ini adalah "consolidation task" — data sudah ada, tinggal pastikan semua masuk ke prompt.

## Audit: Data yang SUDAH FETCH tapi Cek Apakah Masuk ke AI

```go
// Di cmdOutlook / buildUnifiedOutlookInput:
vixData, _ := vix.FetchTermStructure(ctx)   // FETCH ✅ → AI inject? PERLU CEK
sentimentData, _ := sentiment.GetCachedOrFetch(ctx) // FETCH ✅ → AI inject? PERLU CEK
bisData, _ := bis.GetCachedOrFetch(ctx)      // FETCH ✅ → AI inject? PERLU CEK
wbData, _ := worldbank.GetCachedOrFetch(ctx) // FETCH ✅ → AI inject? PERLU CEK
```

## Langkah Implementasi

### 1. Audit unified_outlook.go

Cek `buildUnifiedOutlookInput()` dan `GenerateUnifiedOutlook()`:
- Apakah `sentimentData.PutCallTotal`, `sentimentData.VIXRegime`, `sentimentData.CryptoFearGreed` dimasukkan ke prompt string?
- Apakah `vixData.Spot`, `vixData.M1`, `vixData.Regime`, `vixData.SlopePct` dimasukkan?
- Apakah `bisData` REER valuations dimasukkan?

### 2. Tambah Section yang Missing ke AI Prompt

```go
// Section: VIX Term Structure
if input.VIXData != nil && input.VIXData.Available {
    b.WriteString(fmt.Sprintf("VIX: %.1f | M1: %.1f | M2: %.1f | Regime: %s | Slope: %.1f%%\n",
        input.VIXData.Spot, input.VIXData.M1, input.VIXData.M2,
        input.VIXData.Regime, input.VIXData.SlopePct))
}

// Section: CBOE Put/Call
if input.SentimentData != nil && input.SentimentData.PutCallAvailable {
    b.WriteString(fmt.Sprintf("CBOE P/C Ratio — Total: %.2f | Equity: %.2f | Signal: %s\n",
        input.SentimentData.PutCallTotal, input.SentimentData.PutCallEquity,
        input.SentimentData.PutCallSignal))
}

// Section: Crypto F&G
if input.SentimentData != nil && input.SentimentData.CryptoFearGreedAvailable {
    b.WriteString(fmt.Sprintf("Crypto Fear & Greed: %.0f (%s)\n",
        input.SentimentData.CryptoFearGreed, input.SentimentData.CryptoFearGreedLabel))
}
```

### 3. Tambah VIXData ke UnifiedOutlookInput struct

```go
type UnifiedOutlookInput struct {
    // ... existing fields
    VIXData       *vix.VIXTermStructure   // add if missing
    SentimentData *sentiment.SentimentData // add if missing
    BISData       *bis.BISData             // add if missing
}
```

## Acceptance Criteria

- [ ] Audit: identifikasi field mana yang belum diinject ke AI prompt
- [ ] VIX term structure (Spot, M1, M2, Regime, SlopePct) masuk ke /outlook AI prompt
- [ ] CBOE Put/Call ratio masuk ke /outlook AI prompt
- [ ] Crypto Fear & Greed masuk ke /outlook AI prompt
- [ ] BIS REER currency valuations masuk ke /outlook AI prompt
- [ ] Tidak ada breaking changes — semua field optional/graceful jika nil
- [ ] /outlook response quality improvement: AI dapat menyebutkan VIX regime dan sentiment dalam analysis

## Referensi

- `.agents/research/2026-04-02-19-data-fed-speeches-cg-trending-edgar-form4-putaran8.md` — Latar belakang
- `internal/service/ai/unified_outlook.go` — Target utama
- `internal/service/vix/` — VIX data yang sudah ada
- `internal/service/sentiment/sentiment.go` — SentimentData yang sudah ada
- `internal/service/bis/` — BIS REER yang sudah ada
