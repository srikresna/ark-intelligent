# goldbach_trading_by_Hopiplaka_1 - Summary

**Source:** PDF - goldbach_trading_by_Hopiplaka_1  
**Created:** 2026-04-08 02:06 UTC  
**Language:** id

---

## Executive Summary


================================================================================
GOLDBACH TRADING - COMPLETE TECHNICAL REFERENCE (CODEABLE)
190 HALAMAN | HOPIPLAKA | 2024.02
================================================================================

## 1. POWER OF THREE (PO3) - FORMULA LENGKAP

### Deret PO3:
PO3[n] = 3^n
- PO3[1] = 3
- PO3[2] = 9  
- PO3[3] = 27 (scalper, stop runs)
- PO3[4] = 81 (ADR, day trader)
- PO3[5] = 243 (AWR, short term)
- PO3[6] = 729 (monthly, position)
- PO3[7] = 2187 (yearly, long term)

### Codeable Function:
def calculate_po3(n):
    return 3 ** n

### Calculating Partitions:
Partition[i] = PO3_value × i
Example PO3=27:
- Partition 1: 0-27
- Partition 2: 27-54
- Partition 3: 54-81

### Trader Profile Mapping:
Scalper: Use PO3[3]=27, target 27 pips
Day Trader: Use PO3[4]=81, target 81 pips (ADR)
Short Term: Use PO3[5]=243, target 243 pips (AWR)
Position: Use PO3[6]=729, target monthly range
Long Term: Use PO3[7]=2187, target yearly range

## 2. GOLDBACH LEVELS - EXACT CALCULATIONS

### Level Formulas (dari High/Low range):
Range = High - Low

Level_0_100 = (Low, High) - Range extreme
Level_3_97 = (Low + Range×0.03, High - Range×0.03) - External delimiter
Level_7_93 = (Low + Range×0.07, High - Range×0.07) - LLOD/HHH
Level_11_89 = (Low + Range×0.11, High - Range×0.11) - Internal liquidity
Level_17_83 = (Low + Range×0.17, High - Range×0.17) - GIP (Inversion Point)
Level_29_71 = (Low + Range×0.29, High - Range×0.29) - Flow layer
Level_47_53 = (Low + Range×0.47, High - Range×0.53) - Rebalance (CE/MT)
Level_41_59 = (Low + Range×0.41, High - Range×0.59) - External rebalance

### Codeable Function:
def calculate_goldbach_levels(high, low):
    r = high - low
    return {
        'extreme': (low, high),
        'external': (low + r*0.03, high - r*0.03),
        'llod_hhh': (low + r*0.07, high - r*0.07),
        'internal': (low + r*0.11, high - r*0.11),
        'gip': (low + r*0.17, high - r*0.17),
        'flow': (low + r*0.29, high - r*0.29),
        'rebalance': (low + r*0.47, high - r*0.53),
        'rebalance_ext': (low + r*0.41, high - r*0.59)
    }

## 3. ENTRY RULES - EXACT CONDITIONS

### Flow Layer Entry (29-71):
1. Price di flow layer [29-71]
2. Wait untuk retrace ke dalam flow
3. Gap formation detected
4. Volume confirmation (>1.5x average)
5. SMT divergence confirmed
6. Entry di retrace dengan gap

### Rebalance Entry (47-53):
1. Price consolidate di [47-53]
2. Wait hit external [41-59]
3. Rejection candle confirmed
4. Entry di opposite direction
5. Stop di luar rebalance
6. Target: opposite external

### Einstein Entry (17-83):
1. Consolidation di [0-100]
2. Block created di [3-97] dan [11-89]
3. Aggressive move higher
4. Gap created di [11-89] - [17-83]
5. Retrace ke [17-83] atau gap
6. Entry di GIP [17-83]
7. Partials di [47-53]
8. Exit di opposite extreme

### GIP Validation:
IF price passes [17-83]:
  → Trade plan INVALID
  → Target next PO3 flow layer
ELSE:
  → Trade plan VALID
  → Continue to target

## 4. LOOK BACK PARTITIONS - MONTHLY RULES

### Monthly Numbers:
Jan: 18 (Jan 8)
Feb: 27 (Feb 7)
Mar: 36 (Mar 6)
Apr: 45 (Apr 5)
May: 54 (May 4)
Jun: 63 (Jun 3)
Jul: 72 (Jul 2)
Aug: 81 (Aug 1)
Sep: 90-99 (Sep 9)
Oct: 108 (Oct 8)
Nov: 117 (Nov 7)
Dec: 126 (Dec 6)

### HIPPO Pattern:
High In, Low Out
1. Identify high of period
2. Wait retrace
3. Entry di support
4. Target: low of period

## 5. AMD CYCLE - EXACT PHASES

### Phase 1: Accumulation
- Consolidation di range
- Volume rendah
- Build liquidity
- Duration: variable

### Phase 2: Manipulation  
- Move melawan trend
- Stop hunt
- Create liquidity
- Entry opportunity

### Phase 3: Distribution
- Move sesuai trend
- Volume tinggi
- Take profit
- Target: external liquidity

## 6. 10 TRADE PLANS - EXACT RULES

### Plan 1: Look Back
Entry: Monthly look back date + key level
Exit: External liquidity
Target: 100s pips/month

### Plan 2: HIPPO
Entry: Retrace after high
Exit: Low of period
Target: 50-75 pips/week

### Plan 3: OSOK (One Shot One Kill)
Entry: PO3 setup confirmed
Exit: Opposite PO3 level
Risk: Single trade only

### Plan 4: Hidden OTE
Entry: 62-79% retracement between 2 Goldbach levels
Exit: Previous high/low
Confirmation: FVG present

### Plan 5: Order Block
Entry: Return to OB + rejection
Exit: Breaker level
Stop: Below OB

### Plan 6: Breaker
Entry: Failed OB retrace
Exit: High/Low
Stop: Outside breaker

### Plan 7: Stop Run
Entry: After liquidity sweep
Exit: Breaker
Stop: Outside sweep

### Plan 8: Equilibrium
Entry: 50% + mitigation block
Exit: External liquidity
Stop: Outside equilibrium

### Plan 9: FVG
Entry: Return to FVG + propulsion
Exit: Next FVG
Stop: Outside FVG

### Plan 10: Einstein
Entry: GIP [17-83] or gap
Exit: Partials at [47-53], full at opposite
Stop: Below confluence

## 7. RISK MANAGEMENT - EXACT FORMULAS

### Position Size:
position_size = (account_balance × risk_percent) / (stop_loss_pips × pip_value)

Example:
Account: $10,000
Risk: 2% = $200
Stop: 50 pips
Pip value: $10
Position = $200 / (50 × $10) = 0.4 lots

### Risk Parameters:
Max risk/trade: 2% (standard), 3% (A+ only)
Daily max: 6%
Weekly max: 12%
Min RR: 1:2
Optimal RR: 1:3

## 8. COMPLETE CODEABLE SYSTEM

```python
class GoldbachTrader:
    def __init__(self, account):
        self.account = account
        
    def get_po3(self, n):
        return 3 ** n
        
    def get_levels(self, high, low):
        r = high - low
        return {
            'flow': (low + r*0.29, high - r*0.29),
            'rebalance': (low + r*0.47, high - r*0.53),
            'gip': (low + r*0.17, high - r*0.17),
            'internal': (low + r*0.11, high - r*0.11)
        }
        
    def check_einstein(self, price, high, low):
        levels = self.get_levels(high, low)
        at_gip = levels['gip'][0] <= price <= levels['gip'][1]
        return at_gip
        
    def calculate_position(self, entry, stop):
        risk = self.account * 0.02
        sl_pips = abs(entry - stop)
        return risk / (sl_pips * 10)
        
    def execute_einstein_trade(self, price, high, low):
        if not self.check_einstein(price, high, low):
            return None
            
        levels = self.get_levels(high, low)
        entry = levels['gip'][0]
        stop = levels['internal'][0]
        target1 = levels['rebalance'][0]
        target2 = levels['extreme'][1]
        
        size = self.calculate_position(entry, stop)
        
        return {
            'entry': entry,
            'stop': stop,
            'target1': target1,
            'target2': target2,
            'size': size,
            'risk': 2.0
        }
```

================================================================================


---

## Key Points

1. ================================================================================ GOLDBACH TRADING - COMPLETE TECHNICAL REFERENCE (CODEABLE) 190 HALAMAN | HOPIPLAKA | 2024.
2. 02 ================================================================================  ## 1.
3. POWER OF THREE (PO3) - FORMULA LENGKAP  ### Deret PO3: PO3[n] = 3^n - PO3[1] = 3 - PO3[2] = 9   - PO3[3] = 27 (scalper, stop runs) - PO3[4] = 81 (ADR, day trader) - PO3[5] = 243 (AWR, short term) - PO3[6] = 729 (monthly, position) - PO3[7] = 2187 (yearly, long term)  ### Codeable Function: def calculate_po3(n):     return 3 ** n  ### Calculating Partitions: Partition[i] = PO3_value × i Example PO3=27: - Partition 1: 0-27 - Partition 2: 27-54 - Partition 3: 54-81  ### Trader Profile Mapping: Scalper: Use PO3[3]=27, target 27 pips Day Trader: Use PO3[4]=81, target 81 pips (ADR) Short Term: Use PO3[5]=243, target 243 pips (AWR) Position: Use PO3[6]=729, target monthly range Long Term: Use PO3[7]=2187, target yearly range  ## 2.
4. GOLDBACH LEVELS - EXACT CALCULATIONS  ### Level Formulas (dari High/Low range): Range = High - Low  Level_0_100 = (Low, High) - Range extreme Level_3_97 = (Low + Range×0.
5. 03) - External delimiter Level_7_93 = (Low + Range×0.


---

## Actionable Insights

- Review the full transcript for detailed examples
- Practice the strategies mentioned in simulated trading
- Create a trading journal based on the rules learned
- Set up alerts for the patterns discussed
- Backtest the strategies on historical data

---

## Tags

trading, risk, management, entry, exit, pattern, price, volume, trend

---

## Related Content

- [Full Text](./full_text.md)
- [Diagrams](./diagrams/)

---

**Metadata:**
- Source URL: full_extraction_detailed
- Processed At: 2026-04-08T02:06:48.948180Z
- OCR Used: False
- OCR Pages: 0
- Total Pages: 190
