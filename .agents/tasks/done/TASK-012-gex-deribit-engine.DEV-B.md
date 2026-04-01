# TASK-012: Gamma Exposure (GEX) Engine via Deribit API

**Priority:** HIGH  
**Cycle:** Siklus 3 — Fitur Baru  
**Estimated Complexity:** MEDIUM  
**Research Ref:** `.agents/research/2026-04-01-09-ict-smc-wyckoff-elliott-features.md`

---

## Deskripsi

Implementasi Gamma Exposure (GEX) engine untuk crypto (BTC, ETH) menggunakan Deribit public API (gratis, no auth required). GEX menunjukkan di mana market maker akan menjadi penyerap volatilitas atau penguat volatilitas berdasarkan posisi options mereka.

## Konteks Teknis

### Data Source: Deribit Public API (GRATIS, no API key)
```
Base URL: https://www.deribit.com/api/v2/public/

Endpoints yang diperlukan:
GET /get_instruments?currency=BTC&kind=option&expired=false
  → daftar semua aktif options (strike, expiry, type)

GET /get_book_summary_by_currency?currency=BTC&kind=option
  → OI, volume, mark price per instrument

GET /ticker?instrument_name=BTC-28MAR25-80000-C
  → delta, gamma, vega per instrument
```

### File yang Perlu Dibuat
```
internal/service/marketdata/deribit/
├── client.go     ← HTTP client untuk Deribit API
└── types.go      ← DeribitOption, Ticker, Instrument structs

internal/service/gex/
├── types.go      ← GEXLevel, GEXResult, GEXProfile structs
├── engine.go     ← Engine.Analyze(ctx, symbol) *GEXResult
└── calculator.go ← GEX calculation logic
```

### File yang Perlu Dimodifikasi
- `internal/adapter/telegram/handler_alpha.go` — tambah `/gex` command
- `internal/adapter/telegram/formatter.go` — FormatGEXResult()
- `internal/adapter/telegram/bot.go` — wire GEX engine
- `internal/config/config.go` — tidak perlu API key (public endpoint)

## Spesifikasi

### Deribit Client
```go
type Client struct {
    baseURL    string
    httpClient *http.Client
}

func (c *Client) GetInstruments(ctx context.Context, currency string) ([]Instrument, error)
func (c *Client) GetBookSummary(ctx context.Context, currency string) ([]BookSummary, error)
func (c *Client) GetTicker(ctx context.Context, instrument string) (*Ticker, error)
```

### GEX Calculation
```go
// GEX per strike = Gamma × OI × ContractSize × SpotPrice²
// Positive GEX (calls > puts at level) = dealer sells as price rises → damping
// Negative GEX (puts > calls at level) = dealer buys as price rises → amplifying

type GEXLevel struct {
    Strike   float64
    CallGEX  float64  // gamma exposure from calls
    PutGEX   float64  // gamma exposure from puts
    NetGEX   float64  // CallGEX - PutGEX
}

type GEXResult struct {
    Symbol        string
    SpotPrice     float64
    TotalGEX      float64   // aggregate net GEX
    GEXFlipLevel  float64   // price level where GEX changes sign (Gamma Neutral)
    PositiveZones []GEXLevel // zones where GEX dampens volatility
    NegativeZones []GEXLevel // zones where GEX amplifies volatility
    KeyLevels     []float64  // strikes dengan GEX terbesar
    Regime        string     // "POSITIVE_GEX" | "NEGATIVE_GEX"
    Implication   string     // human-readable market implication
    AnalyzedAt    time.Time
}
```

### GEX Interpretation
```
Total GEX Positif (> 0):
  - Market maker punya net long gamma
  - Mereka akan sell saat harga naik, buy saat turun
  - Efek: range-bound, volatility dampening, mean reversion
  - Sinyal: sulit untuk breakout

Total GEX Negatif (< 0):
  - Market maker punya net short gamma
  - Mereka akan buy saat harga naik, sell saat turun (momentum)  
  - Efek: volatility amplifying, trending
  - Sinyal: breakout lebih mudah terjadi

GEX Flip Level:
  - Level di mana GEX berubah dari positif ke negatif
  - Breakout di atas flip level biasa lebih explosive
```

## Telegram Command `/gex`

Target: BTC dan ETH (crypto options market cukup liquid)

```
📊 GAMMA EXPOSURE — BTC
💰 Spot: $82,450

🌡️ GEX REGIME: NEGATIVE (-$2.4B)
⚠️  Market dalam negative GEX — volatility amplifying

🎯 KEY LEVELS:
  🔴 GEX Flip: $80,000 (below this = volatile)
  📌 Max Pain: $82,000
  📌 Gamma Wall: $85,000 (call resistance)
  📌 Put Wall: $78,000 (put support)

📊 GEX PROFILE:
  ▓▓▓▓▓░░░░░  $90,000 Calls
  ▓▓▓░░░░░░░  $85,000 Calls [WALL]
  ░░░▓▓▓▓▓░░  $80,000 (Flip Level)
  ░░░░░░▓▓▓▓  $78,000 Puts [WALL]
  ░░░░░░░▓▓▓  $75,000 Puts

💡 IMPLICATION: Dengan GEX negatif, pergerakan price akan
   diperkuat. Break di atas $85K atau bawah $78K berpotensi
   move besar. Watch for gamma squeeze di kedua arah.
```

## Rate Limiting & Caching

- Deribit: rate limit 10 req/s per IP (cukup besar)
- Cache GEX result selama 15 menit (options Greeks berubah lambat)
- Hanya fetch saat command dipanggil (bukan background scheduler)

## Acceptance Criteria

- [ ] Compile tanpa error
- [ ] HTTP client menggunakan context dengan timeout (10s)
- [ ] GEX calculation akurat: test dengan synthetic option chain
- [ ] Cache result 15 menit untuk menghindari hammering Deribit API
- [ ] Graceful error jika Deribit tidak bisa diakses (timeout/rate limit)
- [ ] Output readable di mobile Telegram
- [ ] Min 3 unit tests untuk calculator.go

