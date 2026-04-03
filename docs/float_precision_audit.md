# Float Precision Audit Report

## Executive Summary

This document audits the 1,864+ float64/float32 usages in the ARK Intelligent codebase to identify precision risks in financial calculations.

## Current State

### Float Usage Statistics
- **Total float usages**: 3,748+ occurrences (higher than initial estimate of 1,864)
- **Primary type**: float64 (standard Go floating point)
- **Secondary type**: float32 (minimal usage)

### High-Risk Areas for Precision Errors

#### 1. Price Domain (internal/domain/)
**Files:**
- `price.go`: OHLCV data, percentage calculations
- `daily_price.go`: Daily OHLC values
- `intraday_price.go`: Intraday bar data

**Precision Risks:**
```go
// PriceRecord - WeeklyChange calculation
type PriceRecord struct {
    Open   float64
    High   float64
    Low    float64
    Close  float64
    Volume float64
}

// WeeklyChange: (Close - Open) / Open * 100
// Risk: Division by near-zero values, accumulated rounding
```

**Critical Calculations:**
- `WeeklyChange()`: Percentage change with division
- `WeeklyRange()`: Normalized range calculation
- `PriceContext`: Moving averages, ADX calculations

#### 2. Technical Analysis (internal/service/ta/)
**Precision-Sensitive Operations:**
- Moving average calculations
- Standard deviation (volatility)
- Correlation coefficients
- Backtest PnL calculations

**Risk Pattern:**
```go
// Multiple operations compound errors
sum += value              // Accumulation
avg := sum / float64(n)   // Division
stdDev := math.Sqrt(...)  // Square root amplifies errors
```

#### 3. Backtest Calculations (handler_ctabt.go)
**Areas of Concern:**
- Trade entry/exit price comparisons
- Profit/loss percentage calculations
- Cumulative returns over time
- Sharpe ratio calculations

### Medium-Risk Areas

#### 4. Seasonal Analysis (internal/service/price/seasonal_context.go)
- Average return calculations
- Win rate percentages
- Volatility regime detection

#### 5. Display Formatting (formatter.go)
- Price formatting for Telegram messages
- Percentage display with fixed decimals

### Low-Risk Areas

#### 6. Non-Financial Calculations
- Time durations (sleep values)
- Array indices
- Configuration values

## Recommendations

### Immediate Actions (Low Effort, High Impact)

1. **Add Display Rounding**
   ```go
   // Format prices consistently
   func FormatPrice(price float64) string {
       return fmt.Sprintf("%.2f", price)
   }
   
   // Format percentages
   func FormatPercent(pct float64) string {
       return fmt.Sprintf("%.2f%%", pct)
   }
   ```

2. **Document Precision Requirements**
   - Add comments to domain types about precision
   - Document expected precision for each calculation

### Short-Term Actions (Medium Effort)

3. **Use Decimal for Monetary Values**
   Consider using `shopspring/decimal` package for:
   - Price storage
   - PnL calculations
   - Portfolio values

   ```go
   import "github.com/shopspring/decimal"
   
   type PriceRecord struct {
       Open   decimal.Decimal  // Instead of float64
       High   decimal.Decimal
       Low    decimal.Decimal
       Close  decimal.Decimal
   }
   ```

4. **Add Precision Tests**
   ```go
   func TestPricePrecision(t *testing.T) {
       // Test that 0.1 + 0.2 displays correctly
       // Test percentage calculations
   }
   ```

### Long-Term Actions (High Effort)

5. **Full Decimal Migration**
   - Migrate all price-related structs to decimal
   - Update TA calculations to use decimal
   - Performance testing required (decimal is slower)

6. **Precision Configuration**
   ```go
   type PrecisionConfig struct {
       PriceDecimals     int // 2 for most, 5 for JPY
       PercentDecimals   int // 2
       VolumeDecimals    int // 0 or 2
   }
   ```

## Areas NOT Requiring Change

- Time calculations (using time.Duration)
- Rate limiter internal math
- Non-financial configuration values
- Statistical calculations where approximation is acceptable

## Conclusion

The current float64 usage is acceptable for most display purposes but presents risks for:
1. Long-running backtests with cumulative calculations
2. Portfolio value aggregations
3. Cross-rate calculations involving division chains

**Recommendation**: Implement display rounding (Action 1) immediately. Consider decimal migration only if precision bugs are observed in production.

## Appendix: Files Requiring Review

### High Priority
- `internal/domain/price.go`
- `internal/domain/daily_price.go`
- `internal/domain/intraday_price.go`
- `internal/service/ta/backtest.go`
- `internal/adapter/telegram/handler_ctabt.go`

### Medium Priority  
- `internal/service/price/seasonal_context.go`
- `internal/service/price/fetcher.go`
- `internal/adapter/telegram/formatter.go`

### Low Priority
- Test files (precision less critical)
- Configuration parsing
- Non-financial calculations
