# TASK-231: CoinGecko Trending Search — Tambah ke /sentiment Output

**Priority:** medium
**Type:** data
**Estimated:** S
**Area:** internal/service/marketdata/coingecko/, internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-02 19:00 WIB

## Deskripsi

CoinGecko Demo API sudah dikonfigurasi (API key di .env, client di `coingecko/client.go`). Endpoint `/api/v3/search/trending` **gratis dengan demo key yang sudah ada** dan mengembalikan:
- Top 15 trending coins (sorted by search volume di CoinGecko.com)
- Top 6 trending categories (sector rotation signal — DeFi, Meme, AI, Layer2, dll)

Output `/sentiment` saat ini hanya menampilkan AAII, CNN F&G, CBOE, Crypto F&G, VIX. Tidak ada informasi MANA coin yang sedang trending atau sektor mana yang mendapat perhatian. Ini gap penting untuk crypto trader.

## Verified Endpoint (dari CoinGecko API docs):

```
GET https://pro-api.coingecko.com/api/v3/search/trending
Header: x-cg-demo-api-key: {COINGECKO_API_KEY}
Update: setiap 10 menit
```

Sample response: Top coins dengan price_change_24h, market_cap. Top categories: "Solana Meme Coins" (+14.2% 24h), "Gaming Platform" (+5.9% 24h), dll.

## File yang Harus Diubah

- `internal/service/marketdata/coingecko/client.go` — Tambah `GetTrending()` method
- `internal/service/marketdata/coingecko/models.go` — Tambah `TrendingData`, `TrendingCoin`, `TrendingCategory` structs
- `internal/adapter/telegram/formatter.go` — Tambah trending section ke `FormatSentiment()`
- `internal/adapter/telegram/handler.go` — Fetch trending data di `cmdSentiment`

## Implementasi

### client.go — GetTrending method

```go
// TrendingData holds trending coins and categories from CoinGecko.
type TrendingData struct {
    Coins      []TrendingCoin     // top 15 trending by search volume
    Categories []TrendingCategory // top 6 trending categories
    FetchedAt  time.Time
    Available  bool
}

func (c *Client) GetTrending(ctx context.Context) (*TrendingData, error) {
    // GET /api/v3/search/trending
    // Parse coins[].item: name, symbol, market_cap_rank, data.price_change_percentage_24h.usd
    // Parse categories[]: name, data.market_cap_change_percentage_24h.usd
}
```

### formatter.go — Trending section di FormatSentiment

```
🔥 <b>Trending Crypto (24h)</b>
1. GALA +9.6%  2. SOL -2.1%  3. BTC +0.5%  (by search volume)

📦 <b>Sektor Trending:</b>
• Solana Meme Coins +14.2%
• Gaming Platform   +5.9%
• DeFi              -1.3%
```

### handler.go — Fetch trending sebelum render

```go
var trendingData *coingecko.TrendingData
if h.cgClient != nil {
    trendingData, _ = h.cgClient.GetTrending(ctx)
}
htmlMsg := h.fmt.FormatSentiment(data, macroRegime, trendingData)
```

## Caching

Tambah trending ke dalam `sentiment/cache.go` atau buat cache terpisah 10 menit di coingecko package.

## Acceptance Criteria

- [ ] `GetTrending()` berhasil call `/search/trending` dengan demo API key
- [ ] Mengembalikan min 3 trending coins dengan name, symbol, 24h change
- [ ] Mengembalikan min 3 trending categories dengan name, 24h market cap change
- [ ] `/sentiment` output menampilkan trending section
- [ ] Jika COINGECKO_API_KEY tidak ada atau rate limited → skip gracefully
- [ ] Unit test: `TestGetTrending` memverifikasi parsing response

## Referensi

- `.agents/research/2026-04-02-19-data-fed-speeches-cg-trending-edgar-form4-putaran8.md` — Temuan 2
- `internal/service/marketdata/coingecko/client.go` — Client yang sudah ada
- `internal/adapter/telegram/formatter.go` — `FormatSentiment()` untuk ditambah section
- CoinGecko API docs: `docs.coingecko.com/reference/trending-search`
