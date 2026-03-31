# Feature Index — ark-intelligent
# Digunakan Research Agent sebagai referensi riset

---

## Fitur Aktif (Telegram Commands)

| Command | Deskripsi | Service |
|---|---|---|
| `/cot` | COT positioning summary + signal detection | service/cot |
| `/bias` | Directional bias dari COT + macro | service/cot |
| `/rank` | Currency strength ranking (COT + macro) | service/cot |
| `/rankx` | Extended rank dengan quant overlay | service/cot + ta |
| `/calendar` | Economic calendar high-impact events | service/news |
| `/outlook` | Weekly macro outlook via AI (Claude/Gemini) | service/ai |
| `/macro` | FRED dashboard (yields, labor, inflation) | service/fred |
| `/cta` | Classical Technical Analysis (multi-indicator) | service/ta |
| `/ctabt` | CTA Backtest | service/backtest + ta |
| `/quant` | Quant analysis (GARCH, HMM, Hurst, seasonal) | service/price |
| `/vp` | Volume Profile / Market Profile | scripts/vp_engine.py |
| `/levels` | Key price levels (S/R, Fib, pivots) | service/price/levels |
| `/price` | Current price + context | service/price |
| `/seasonal` | Seasonal patterns per currency | service/price/seasonal |
| `/alpha` | Alpha signals | service/factors |
| `/cryptoalpha` | Crypto alpha signals | service/marketdata |
| `/sentiment` | Market sentiment (CBOE VIX, AAII) | service/sentiment |
| `/backtest` | Signal backtest results | service/backtest |
| `/report` | Comprehensive market report | multiple |
| `/playbook` | Strategy playbook | service/strategy |
| `/heat` | Portfolio heat map | service/strategy |
| `/xfactors` | X-factors analysis | service/factors |
| `/impact` | Event impact scoring | service/news |
| `/transition` | Regime transition detection | service/fred |
| `/accuracy` | Signal accuracy stats | service/backtest |
| `/prefs` / `/settings` | User preferences | storage/prefs_repo |
| `/membership` | Membership management | storage/user_repo |
| `/users` | User management (admin) | storage/user_repo |
| `/ban` / `/unban` | Ban management (admin) | storage/user_repo |
| `/setrole` | Set user role (admin) | storage/user_repo |
| `/status` | Bot status | health |
| `/start` / `/help` | Onboarding | handler |

---

## Layanan Internal (Services)

### service/cot
- Fetch CFTC Commitment of Traders data
- Net position analysis, percentile ranking
- Signal detection (5 strength levels)
- RecalibratedDetector dengan win rate stats
- ConvictionScoreV3 (5 komponen)
- Carry trade adjustment
- VIX regime filter
- ATR volatility multiplier
- Thin market + concentration alerts
- Broadcast ke semua active users saat data baru

### service/fred
- FRED API integration (St. Louis Fed)
- Macro regime classification
- Yield curve (inversion, steepening)
- Labor market indicators
- Inflation regime
- Rate differential engine (carry ranking)
- Regime asset performance
- Regime history tracking
- Alert system untuk regime change

### service/news
- Economic calendar via MQL5
- Event impact scoring + surprise index
- Revision detection
- Impact bootstrap (historical backfill)
- Surge detection

### service/price
- Multi-source price fetcher (TwelveData, Massive/Polygon)
- Weekly, daily, intraday (15m→12h) data
- GARCH volatility forecasting
- HMM regime detection
- Hurst exponent (trend persistence)
- Seasonal analysis
- Correlation matrix
- Divergence detection
- EIA energy data integration
- Synthetic cross pairs (XAUEUR, dll)
- VIX/SPX risk context
- Position sizing
- Key levels (S/R, Fib, pivots)
- Volume Profile (via Python vp_engine.py)

### service/ta (Classical Technical Analysis)
- RSI(14), MACD(12,26,9), Stochastic(14,3,3)
- Bollinger Bands(20,2), EMA Ribbon (9,21,55,100,200)
- ADX(14), OBV, Williams %R(14), CCI(20), MFI(14)
- Ichimoku Cloud (Tenkan, Kijun, Senkou A/B, Chikou)
- SuperTrend(10,3)
- Fibonacci auto swing detection + retracement
- Candlestick patterns
- RSI/MACD divergence (regular + hidden)
- Multi-indicator confluence scoring
- Multi-timeframe alignment matrix
- Entry/exit zones + R:R calculator
- Chart rendering via mplfinance (Python)

### service/ai
- Claude (Anthropic) integration
- Gemini integration
- Cached interpreter (TTL-based)
- Rate limiting
- Context builder untuk AI prompts
- Tool executor (function calling)
- Memory store (conversation history)
- Unified outlook (multi-phase analysis)
- Chat service (conversational AI)

### service/backtest
- Signal persistence untuk future evaluation
- Outcome evaluation setelah N hari
- Win rate, PnL, Sharpe ratio
- Daily trend filter
- Baseline comparison
- Bootstrap statistics
- Cost modeling

### service/microstructure
- Order flow analysis
- Bybit integration (crypto microstructure)
- Market structure detection

### service/sentiment
- CBOE VIX data
- AAII investor sentiment (via Firecrawl)
- Sentiment caching

### service/strategy
- Strategy playbook engine
- Portfolio heat calculation
- Factor engine

### service/marketdata
- Bybit client (crypto)
- CoinGecko client (BTC dominance, TOTAL3)
- Massive/Polygon client (historical data)
- API key rotation (round-robin)

---

## Infrastruktur

- **Storage**: BadgerDB (embedded KV, zero external deps)
- **Scheduler**: Built-in background jobs (9 job types)
- **Messaging**: Telegram Bot API (long-polling)
- **Charts**: Python (mplfinance, matplotlib)
- **Health**: HTTP health check endpoint (:8080)
- **Logging**: Structured JSON logging
- **Rate limiting**: Per-user, per-command

---

## Rencana Pengembangan (dari docs/)

### AMT Upgrade Plan (docs/AMT_UPGRADE_PLAN.md)
Advanced Market Theory — Market Profile / Auction Market Theory:
- Day Type Classification (Dalton's 6 types: Normal, Normal Variation, Trend, Double Distribution, P-shape, b-shape)
- Opening Type Analysis (4 types: Open Drive, Open Test Drive, Open Rejection Reverse, Open Auction)
- Multi-day context analysis
- TPO letters (30m bars)

### Volume Profile Plan (docs/VP_PLAN.md)
- Advanced VP features

---

## Area Riset Potensial untuk Research Agent

### Sudah ada, bisa didalami:
- COT → tambah Open Interest analysis, commercial vs non-commercial divergence
- FRED → tambah leading indicators (PMI, CLI, LEI)
- GARCH → EGARCH, GJR-GARCH untuk volatility asymmetry
- HMM → lebih banyak state, online learning

### Belum ada, bisa diriset:
- **ICT (Inner Circle Trader)**: Fair Value Gap, Order Block, Breaker Block, Liquidity Sweep, Killzone, PD Array
- **Smart Money Concepts (SMC)**: Change of Character (CHOCH), Break of Structure (BOS), premium/discount zones
- **Quant/Institutional**: DTCC/CLS settlement data, dark pool prints, options flow, gamma exposure (GEX)
- **Market Microstructure**: Bid-ask spread analysis, order book imbalance, VWAP deviation
- **Alternative Data**: COT disaggregated (swap dealers, money managers), non-commercial vs commercial spread
- **Macro Quant**: Goldbach conjecture applications in price theory, vortex mathematics untuk cycle analysis
- **ML/AI**: WD-GAN (Wasserstein Divergence GAN) untuk synthetic price generation + scenario analysis
- **Volatility Surface**: VIX term structure, SKEW index, put/call ratio
- **Intermarket Analysis**: Dollar index correlation matrix, commodity currencies, risk-on/off regime
- **Order Flow**: Footprint chart concepts, delta analysis, absorption patterns
- **Wyckoff**: Accumulation/distribution schematics, spring, upthrust, creek
- **Elliott Wave**: Automated wave counting, Fibonacci relationships
- **Seasonality**: COT seasonality (tidak hanya price), macro data seasonality
- **Cross-asset**: Bond-equity correlation, credit spreads sebagai risk indicator
