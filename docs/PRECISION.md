# Financial Calculation Precision Requirements

## Overview

This document outlines precision requirements and guidelines for financial calculations throughout the ARK Intelligent codebase to prevent rounding errors and ensure consistent display formatting.

## Position Count Precision

### Raw Position Data (COT Records)
- **Type**: `int64` (whole contracts)
- **Fields**: All position counts (DealerLong, LevFundLong, ManagedMoneyLong, etc.)
- **Rationale**: Position counts are always whole numbers (you can't hold 0.5 of a futures contract)
- **Status**: ✅ Already using int64 - no precision issues

### Computed Net Positions
- **Type**: `float64` (converted from int64 arithmetic)
- **Fields**: NetPosition, LevFundNet, ManagedMoneyNet, CommercialNet
- **Calculation**: `float64(longCount - shortCount)`
- **Precision**: Exact for values up to 9,007,199,254,740,991 (no floating point error for integer values in this range)

## Price and Monetary Value Precision

### Current Implementation
- **Type**: `float64` throughout the system
- **Use Cases**: Spot prices, portfolio values, percentage calculations

### Precision Considerations

| Value Range | float64 Precision | Financial Impact |
|-------------|-------------------|------------------|
| < $100,000 | ~15 decimal digits | Negligible (< $0.00001) |
| $100,000 - $1M | ~10 decimal digits | Minor (< $0.01) |
| > $1M | ~9 decimal digits | May affect cents |

### Recommendations for High-Precision Requirements

If exact decimal precision is required for monetary values (e.g., accounting), consider:

```go
import "github.com/shopspring/decimal"

// For exact decimal arithmetic
total := decimal.NewFromFloat(100.01).Add(decimal.NewFromFloat(0.02))
// Result: exactly 100.03 (not 100.029999999994)
```

**Note**: Currently using `float64` is acceptable because:
1. Position counts use int64 (no rounding)
2. Price precision errors are sub-cent at normal trading ranges
3. Percentage calculations (COT Index, etc.) don't require exact decimal precision

## Percentage Calculation Precision

### COT Index Calculation
```
Index = (CurrentNet - MinNet) / (MaxNet - MinNet) * 100
```

**Precision Guard**: Division by zero returns 50.0 (neutral)
**Clamp**: Results clamped to [0, 100] range

### Percentage of Open Interest
```
PctOfOI = NetPosition / OpenInterest * 100
```

**Precision**: Using float64 division - acceptable for display purposes
**Rounding**: Not explicitly rounded; format at display layer

## Display Formatting Consistency

### Price Formatting
- **Format function**: `pkg/fmtutil/format.go:FmtPrice()`
- **Rules**:
  - Forex pairs: 4-5 decimal places (pips)
  - Crypto: 2-6 decimal places depending on price magnitude
  - Indices: 2 decimal places
  - Commodities: 2-4 decimal places

### Percentage Formatting
- **COT Index**: 1 decimal place in display
- **Percentage of OI**: 1 decimal place
- **Momentum/Margins**: 2 decimal places

## Edge Cases and Tests

### Critical Precision Edge Cases

1. **Division by Zero**
   - All division operations must check for zero denominator
   - Use `mathutil.SafeDiv()` for safe division with fallback

2. **Very Large Numbers**
   - Position counts can exceed 1 million contracts
   - Open interest can exceed 10 million
   - Ensure no integer overflow in int64 (max: ~9 quintillion)

3. **Very Small Numbers**
   - Momentum values near zero
   - Percentage changes on small bases
   - Use epsilon comparisons: `math.Abs(val) < 1e-9`

4. **NaN and Infinity**
   - Check: `mathutil.IsFinite()`
   - Never store NaN/Inf in database
   - Sanitize before JSON serialization

### Precision Test Checklist

See `precision_test.go` for implementation:

- [x] Large position counts don't lose precision
- [x] Zero division returns safe fallback values
- [x] NaN/Inf values are sanitized
- [x] Percentage calculations clamp to valid ranges
- [x] COT Index computation is numerically stable

## Guidelines for Future Development

### DO:
- ✅ Use `int64` for all position counts and contract quantities
- ✅ Use `mathutil.SafeDiv()` for division operations
- ✅ Clamp results to valid ranges before storage
- ✅ Format at the display layer, not calculation layer
- ✅ Add epsilon tolerance for float comparisons

### DON'T:
- ❌ Use float64 for position counts (always use int64)
- ❌ Compare floats with `==` (use epsilon tolerance)
- ❌ Store NaN/Infinity in database
- ❌ Round intermediate calculations (only final display values)
- ❌ Mix monetary addition with float64 (consider decimal for accounting)

## Testing

Run precision tests:
```bash
go test ./internal/domain/... -run TestPrecision -v
go test ./pkg/mathutil/... -v
```

## Related Files

- `internal/domain/cot.go` - COT record and analysis types
- `pkg/mathutil/stats.go` - Statistical calculations with precision guards
- `pkg/mathutil/safe.go` - Safe arithmetic helpers
- `pkg/fmtutil/format.go` - Display formatting
