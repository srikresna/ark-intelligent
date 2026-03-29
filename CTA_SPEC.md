# Classical Technical Analysis (CTA) — Implementation Spec

## Command: `/cta` (aliased as `/cta [SYMBOL] [TIMEFRAME]`)

## File Structure
```
internal/service/ta/
├── types.go          # All domain types
├── indicators.go     # RSI, MACD, Stoch, BB, EMA, Williams%R, CCI, MFI, OBV, ADX
├── ichimoku.go       # Ichimoku Cloud (Tenkan, Kijun, Senkou A/B, Chikou)
├── supertrend.go     # SuperTrend(10,3)
├── fibonacci.go      # Auto swing detection + Fib retracement
├── patterns.go       # Candlestick pattern detection
├── divergence.go     # RSI/MACD divergence (regular + hidden)
├── confluence.go     # Multi-indicator scoring + grade
├── mtf.go            # Multi-timeframe alignment matrix
├── zones.go          # Entry/exit zone + R:R calculator
├── engine.go         # Main engine orchestrator
├── ta_test.go        # Tests for indicator math

scripts/
├── cta_chart.py      # Python mplfinance chart renderer

internal/adapter/telegram/
├── handler_cta.go    # /cta command + inline callback handlers
```

## Indicator Specifications (MUST be mathematically correct)

### RSI(14)
- Wilder's smoothing (NOT simple average after first window)
- First RSI: avg_gain = sum(gains)/14, avg_loss = sum(losses)/14
- Subsequent: avg_gain = (prev_avg_gain * 13 + current_gain) / 14
- RSI = 100 - (100 / (1 + RS)), RS = avg_gain / avg_loss
- Signal: >70 = overbought, <30 = oversold

### MACD(12,26,9)
- MACD Line = EMA(12) - EMA(26)
- Signal Line = EMA(9) of MACD Line
- Histogram = MACD Line - Signal Line
- EMA uses standard multiplier: 2/(period+1)
- Signal: bullish cross = MACD crosses above Signal, histogram positive

### Stochastic(14,3,3)
- %K_raw = (Close - Lowest_Low_14) / (Highest_High_14 - Lowest_Low_14) * 100
- %K = SMA(3) of %K_raw (slow stochastic)
- %D = SMA(3) of %K
- Signal: >80 = overbought, <20 = oversold, %K cross %D

### Bollinger Bands(20,2)
- Middle = SMA(20)
- Upper = Middle + 2 * StdDev(20)
- Lower = Middle - 2 * StdDev(20)
- Bandwidth = (Upper - Lower) / Middle * 100
- %B = (Close - Lower) / (Upper - Lower)
- Squeeze: Bandwidth < 20-period average of Bandwidth * 0.75

### EMA(period)
- Multiplier = 2 / (period + 1)
- EMA[0] = SMA(period) for first value (seed)
- EMA[i] = (Close[i] - EMA[i-1]) * multiplier + EMA[i-1]
- Ribbon: 9, 21, 55, 100, 200

### ADX(14) — Exact (not approximation)
- +DM = High[i] - High[i-1] if positive and > -(Low[i] - Low[i-1]), else 0
- -DM = Low[i-1] - Low[i] if positive and > +(High[i] - High[i-1]), else 0
- TR = max(High-Low, |High-PrevClose|, |Low-PrevClose|)
- Smoothed +DM14 = Wilder smoothing (first = sum of 14, then = prev - prev/14 + current)
- Smoothed -DM14 = same
- Smoothed TR14 = same
- +DI = Smoothed +DM14 / Smoothed TR14 * 100
- -DI = Smoothed -DM14 / Smoothed TR14 * 100
- DX = |+DI - -DI| / (+DI + -DI) * 100
- ADX = Wilder smoothing of DX (14-period)
- Signal: ADX > 25 = trending, < 20 = ranging

### OBV (On-Balance Volume)
- If Close > PrevClose: OBV += Volume
- If Close < PrevClose: OBV -= Volume
- If Close == PrevClose: OBV unchanged
- Compare OBV trend vs price trend for divergence

### Williams %R(14)
- %R = (Highest_High_14 - Close) / (Highest_High_14 - Lowest_Low_14) * -100
- Signal: < -80 = oversold, > -20 = overbought

### CCI(20)
- TP = (High + Low + Close) / 3
- CCI = (TP - SMA(TP, 20)) / (0.015 * Mean_Deviation(TP, 20))
- Mean Deviation = average of |TP - SMA(TP)|
- Signal: > +100 = overbought, < -100 = oversold

### MFI(14)
- TP = (High + Low + Close) / 3
- Raw Money Flow = TP * Volume
- Positive MF = sum of RMF when TP > prev_TP (14 periods)
- Negative MF = sum of RMF when TP < prev_TP (14 periods)
- MFI = 100 - (100 / (1 + Positive_MF / Negative_MF))
- Signal: > 80 = overbought, < 20 = oversold
- NOTE: For FX where volume may be 0, skip MFI or use tick volume

### Ichimoku Cloud (9,26,52)
- Tenkan-sen = (Highest_High_9 + Lowest_Low_9) / 2
- Kijun-sen = (Highest_High_26 + Lowest_Low_26) / 2
- Senkou Span A = (Tenkan + Kijun) / 2, plotted 26 periods ahead
- Senkou Span B = (Highest_High_52 + Lowest_Low_52) / 2, plotted 26 periods ahead
- Chikou Span = Close, plotted 26 periods back
- Signals:
  - TK Cross: Tenkan crosses Kijun (bullish if above cloud, bearish if below)
  - Kumo Breakout: Price breaks above/below cloud
  - Chikou: Chikou above price of 26 periods ago = bullish
  - Cloud color: Senkou A > Senkou B = bullish cloud

### SuperTrend(10,3)
- ATR = ATR(10)
- Basic Upper Band = (High + Low) / 2 + 3 * ATR
- Basic Lower Band = (High + Low) / 2 - 3 * ATR
- Final Upper = min(Basic Upper, prev Final Upper) if prev Close <= prev Final Upper
- Final Lower = max(Basic Lower, prev Final Lower) if prev Close >= prev Final Lower
- SuperTrend = Final Lower if Close > prev Final Upper, else Final Upper
- Signal: Price above SuperTrend = bullish, below = bearish

### Fibonacci Retracement
- Auto-detect swing high and swing low from last N bars (use 50 bars)
- Swing High: bar where High > High of 5 bars before and after
- Swing Low: bar where Low < Low of 5 bars before and after
- Use most recent significant swing pair
- Levels: 0% (swing low), 23.6%, 38.2%, 50%, 61.8%, 78.6%, 100% (swing high)
- If current trend is up: measure from swing low to swing high
- If current trend is down: measure from swing high to swing low

### Candlestick Patterns
Detect on the last 1-5 bars:
- **Single bar**: Doji, Hammer, Inverted Hammer, Shooting Star, Spinning Top, Marubozu
- **Two bar**: Bullish/Bearish Engulfing, Piercing, Dark Cloud Cover, Tweezer Top/Bottom
- **Three bar**: Morning Star, Evening Star, Three White Soldiers, Three Black Crows
- Each pattern has: name, direction (BULLISH/BEARISH/NEUTRAL), reliability (1-3)

### Divergence Detection
For RSI and MACD:
- **Regular Bullish**: Price makes lower low, indicator makes higher low → reversal up
- **Regular Bearish**: Price makes higher high, indicator makes lower high → reversal down
- **Hidden Bullish**: Price makes higher low, indicator makes lower low → continuation up
- **Hidden Bearish**: Price makes lower high, indicator makes higher high → continuation down
- Use 5-bar swing detection for pivot identification
- Minimum 5 bars between pivots

## Confluence Scoring

### Per-Indicator Signal (-1 to +1)
Each indicator produces a normalized signal:
- RSI: map 0-100 to signal. <30 = +1 (bullish reversal), >70 = -1 (bearish reversal), 50 = 0
  - In trending market (ADX>25): RSI 40-60 = 0, <40 = -0.5, >60 = +0.5
- MACD: histogram direction + crossover. +1 = bullish cross, -1 = bearish cross, histogram sign = ±0.5
- Stochastic: similar to RSI zones
- Bollinger: %B < 0 = -1 (below lower), > 1 = +1 (above upper), squeeze = 0 (neutral)
- EMA Ribbon: count aligned EMAs. All bullish aligned = +1, all bearish = -1
- ADX: only modifies weight, not direction. ADX > 25 = full weight, < 20 = half weight
- Ichimoku: composite of TK cross + cloud position + chikou. Range -1 to +1
- SuperTrend: +1 if price above, -1 if below
- Volume (OBV): +1 if OBV trend confirms price trend, -1 if diverges, 0 if flat

### Weights
```
Trend (40%):
  EMA Ribbon:    15%
  SuperTrend:    10%
  Ichimoku:      10%
  ADX direction: 5%

Momentum (35%):
  RSI:           10%
  MACD:          12%
  Stochastic:    8%
  CCI:           5%

Volume (15%):
  OBV:           8%
  MFI:           7%

Volatility (10%):
  Bollinger:     6%
  Williams %R:   4%
```

### Multi-Timeframe Weighting
```
Daily:  0.35
4H:     0.25
1H:     0.20
15m:    0.10
Weekly: 0.10
```

### Final Score & Grade
Score range: -100 to +100
- ±75-100: Grade A (Strong)
- ±50-74:  Grade B (Good)
- ±25-49:  Grade C (Moderate)
- ±1-24:   Grade D (Weak)
- 0:       Grade F (Flat/No edge)

Positive = bullish, negative = bearish.

## Entry/Exit Zone Calculation
1. Entry zone: Fibonacci level nearest to current price + Bollinger Band middle ± ATR*0.5
2. Stop Loss: Below/above nearest Fibonacci level beyond entry + ATR(14) * 1.5
3. Take Profit: Next Fibonacci level in trade direction + Bollinger Band opposite side
4. Risk:Reward = |TP - Entry| / |Entry - SL|
5. Only suggest zones if Grade >= C and R:R >= 1.5

## Chart (Python mplfinance)
- Input: JSON file with OHLCV + indicator values
- Output: PNG file (1200x900, dark theme)
- Layout:
  - Main panel (60%): Candlestick + EMA overlay + Bollinger Bands + Ichimoku Cloud (optional) + SuperTrend line
  - RSI panel (13%): RSI(14) with 30/70 lines
  - MACD panel (13%): MACD + Signal + Histogram
  - Volume panel (14%): Volume bars colored by direction
- Fibonacci levels: horizontal dashed lines on main panel
- Candlestick patterns: annotated arrows on main panel
- Dark professional theme (black background, green/red candles)

## Telegram Handler
- `/cta EUR` → compute all timeframes, show daily summary + chart + inline menu
- `/cta EUR 15m` → show specific timeframe
- Callback routing: `cta:` prefix
  - `cta:summary` / `cta:refresh` / `cta:back`
  - `cta:tf:15m` / `cta:tf:30m` / `cta:tf:1h` / `cta:tf:4h` / `cta:tf:daily` / `cta:tf:weekly`
  - `cta:ichi` / `cta:fib` / `cta:patterns` / `cta:confluence` / `cta:mtf` / `cta:zones`
- Summary text: Indonesian with financial data

## SendPhoto for bot.go
- Add `SendPhoto(ctx, chatID, photoBytes []byte, caption string) (int, error)` to Bot
- Use multipart/form-data POST to Telegram `sendPhoto` API
- Add `SendPhotoWithKeyboard` variant that includes inline keyboard
