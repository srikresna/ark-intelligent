# TASK-306: CoinGecko Global Data → /sentiment (BTC Dominance, Altcoin MCap)

**Priority:** medium
**Type:** data
**Estimated:** S
**Area:** internal/service/marketdata/coingecko/client.go, internal/service/sentiment/sentiment.go, internal/adapter/telegram/formatter.go, internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-04 01:00 WIB
**Siklus:** Data-2 (Siklus 2 Putaran 23)
**Ref:** research/2026-04-04-01-data-sources-audit-naaim-coingecko-global-sentiment-ux-putaran23.md

## Deskripsi

`internal/service/marketdata/coingecko/client.go` berisi 4 methods yang **belum pernah dipanggil** di luar package sendiri:

```bash
$ grep -rn "GetGlobalData\|GetBTCDominance\|GetAltcoinMarketCap\|GetMarketSentiment" internal/ | grep -v "client.go"
# → 0 results
```

Methods ini menggunakan CoinGecko API key yang **sudah dikonfigurasi** di `.env` (COINGECKO_API_KEY):

- `GetGlobalData(ctx)` → total market cap (USD), 24h change%, active cryptos, BTC dominance%
- `GetBTCDominance(ctx)` → % BTC dari total crypto market cap
- `GetAltcoinMarketCap(ctx)` → approx altcoin market cap (total - BTC - ETH)
- `GetMarketSentiment(ctx)` → score 0-100 derived dari 24h market cap change

**Use case:**
1. Tampilkan di `/sentiment` section "Crypto Market" → BTC dominance%, altcoin mcap, 24h market change
2. Gunakan `GetMarketSentiment()` sebagai **fallback** ketika `alternative.me` Crypto F&G gagal
3. BTC dominance rising + altcoin mcap falling = BTC season (favorable untuk BTCUSD longs, unfavorable alts)
4. BTC dominance falling + altcoin mcap rising = altseason signal

## File yang Harus Diubah

1. `internal/service/sentiment/sentiment.go` — Tambah fields ke SentimentData + fetch call
2. `internal/adapter/telegram/formatter.go` — Tambah section "Crypto Market" ke FormatSentiment
3. `internal/adapter/telegram/handler.go` — Pastikan CoinGeckoClient dipass ke sentiment fetch

## Implementasi

### Step 1: Tambah fields ke SentimentData struct (`sentiment.go`)

```go
// CoinGecko Global Market Data
CryptoTotalMCap      float64 // Total crypto market cap USD
CryptoMCap24hChange  float64 // 24h market cap change %
BTCDominance         float64 // BTC % of total market cap
AltcoinMCap          float64 // Altcoin market cap (approx, USD)
CGMarketSentLabel    string  // "EXTREME_FEAR", "FEAR", "NEUTRAL", "GREED", "EXTREME_GREED"
CGMarketSentScore    float64 // 0-100
CGMarketAvailable    bool
```

### Step 2: Tambah ke SentimentFetcher struct

```go
type SentimentFetcher struct {
    httpClient *http.Client
    cbCNN      *circuitbreaker.Breaker
    cbAAII     *circuitbreaker.Breaker
    cbCBOE     *circuitbreaker.Breaker
    cbCrypto   *circuitbreaker.Breaker
    cbNAAIM    *circuitbreaker.Breaker  // dari TASK-305
    cbCG       *circuitbreaker.Breaker  // CoinGecko global data
    cgClient   *coingecko.Client        // BARU
}
```

### Step 3: Update NewSentimentFetcher() dan tambah WithCoinGecko setter

```go
func (f *SentimentFetcher) WithCoinGeckoClient(client *coingecko.Client) *SentimentFetcher {
    f.cgClient = client
    return f
}
```

Atau inject via handler.go yang sudah punya cgClient.

### Step 4: Fetch di Fetch()

```go
// CoinGecko Global Data
if f.cgClient != nil && f.cgClient.IsConfigured() {
    if err := f.cbCG.Execute(func() error {
        return fetchCoinGeckoGlobal(ctx, f.cgClient, data)
    }); err != nil {
        log.Debug().Err(err).Msg("sentiment: CoinGecko global circuit breaker")
    }
}
```

### Step 5: Fungsi fetchCoinGeckoGlobal

```go
func fetchCoinGeckoGlobal(ctx context.Context, client *coingecko.Client, data *SentimentData) error {
    global, err := client.GetGlobalData(ctx)
    if err != nil {
        return err
    }

    usdMCap := global.TotalMarketCap["usd"]
    btcDom  := global.MarketCapPercentage["btc"]
    ethDom  := global.MarketCapPercentage["eth"]
    altcoin := usdMCap * (1 - (btcDom+ethDom)/100)

    data.CryptoTotalMCap     = usdMCap
    data.CryptoMCap24hChange = global.MarketCapChangePercent
    data.BTCDominance        = btcDom
    data.AltcoinMCap         = altcoin
    data.CGMarketAvailable   = true

    // Derive sentiment label
    ms, err := client.GetMarketSentiment(ctx)
    if err == nil {
        data.CGMarketSentLabel = ms.Label
        data.CGMarketSentScore = ms.Score
    }

    // Fallback for CryptoFearGreed if not already available
    if !data.CryptoFearGreedAvailable && ms != nil {
        data.CryptoFearGreed          = ms.Score
        data.CryptoFearGreedLabel     = ms.Label
        data.CryptoFearGreedAvailable = true
        log.Debug().Msg("CryptoF&G: using CoinGecko fallback")
    }

    return nil
}
```

### Step 6: Tampilkan di formatter.go

```go
if data.CGMarketAvailable {
    // Format market cap in trillions/billions
    mcapStr := fmtutil.FormatLargeUSD(data.CryptoTotalMCap)

    b.WriteString("\n<b>Crypto Market (CoinGecko)</b>\n")
    b.WriteString(fmt.Sprintf("<code>Total MCap : %s (24h: %+.1f%%)</code>\n", mcapStr, data.CryptoMCap24hChange))
    b.WriteString(fmt.Sprintf("<code>BTC Dom    : %.1f%%</code>\n", data.BTCDominance))
    altStr := fmtutil.FormatLargeUSD(data.AltcoinMCap)
    b.WriteString(fmt.Sprintf("<code>Altcoin Cap: %s</code>\n", altStr))

    // Altseason signal
    if data.BTCDominance > 55 {
        b.WriteString("<code>Signal     : 📈 BTC Season — dominance tinggi</code>\n")
    } else if data.BTCDominance < 45 {
        b.WriteString("<code>Signal     : 🌊 Altseason — BTC dominance rendah</code>\n")
    } else {
        b.WriteString("<code>Signal     : ⚖️ Netral — dominance transisi</code>\n")
    }
}
```

## Catatan: FormatLargeUSD helper

Cek apakah `fmtutil.FormatLargeUSD()` sudah ada. Jika tidak:

```go
func FormatLargeUSD(v float64) string {
    switch {
    case v >= 1e12:
        return fmt.Sprintf("$%.2fT", v/1e12)
    case v >= 1e9:
        return fmt.Sprintf("$%.1fB", v/1e9)
    default:
        return fmt.Sprintf("$%.0f", v)
    }
}
```

## Acceptance Criteria

- [ ] `CGMarketAvailable`, `BTCDominance`, `CryptoTotalMCap`, `CryptoMCap24hChange`, `AltcoinMCap` ada di SentimentData
- [ ] `fetchCoinGeckoGlobal()` dipanggil di `SentimentFetcher.Fetch()`
- [ ] Circuit breaker `cbCG` terpasang
- [ ] Ketika CoinGecko key tidak set (IsConfigured() = false) → skip gracefully
- [ ] `/sentiment` menampilkan section "Crypto Market" dengan BTC dominance dan altcoin cap
- [ ] Altseason/BTC season signal muncul berdasarkan BTC dominance threshold
- [ ] Ketika alternative.me Crypto F&G gagal, CoinGecko sentiment digunakan sebagai fallback
- [ ] `go build ./...` clean

## Referensi

- `internal/service/marketdata/coingecko/client.go:99` — GetGlobalData()
- `internal/service/marketdata/coingecko/client.go:218` — GetMarketSentiment()
- `internal/service/sentiment/cboe.go` — pattern circuit breaker
- `internal/service/sentiment/sentiment.go:115` — SentimentData struct
- research/2026-04-04-01-data-sources-audit-naaim-coingecko-global-sentiment-ux-putaran23.md
