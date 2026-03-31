# AMT Advanced Upgrade Plan

## Current State (Intermediate)
- Auction state: balance/breakout/responsive (static, 1 snapshot)
- Initiative/Responsive: simple bar count (last 10 bars)
- Excess/Poor: volume + rejection tail at extremes
- Single prints: 1-2 touch zones
- No multi-day context, no day type, no opening analysis

## Data Available
- 15m bars: ~52 days → best for session/day analysis
- 30m bars: ~52 days → TPO letters (standard Market Profile uses 30m)
- 1h/4h/6h/12h/daily: various depths
- Volume: tick (FX) / actual (futures)
- Timestamps: UTC with hour precision

---

## Upgrade Plan — 5 Modules

### Module 1: Day Type Classification (Dalton's 6 types)
**What**: Classify each trading day into one of 6 Market Profile day types.
**Why**: Day type tells you the CHARACTER of the day — is it trending, balanced, or transitioning?

Types:
1. **Normal Day** — 85% of range in IB, tight VA, no extension. Balanced, fade extremes.
2. **Normal Variation Day** — IB extends 50-100%, one-sided. Moderate trend.
3. **Trend Day** — Open on one extreme, close on other. IB < 30% of range. Strong conviction.
4. **Double Distribution Day** — Two separate VAs. Starts balanced, event causes migration.
5. **P-shape Day** — Heavy volume up top, long tail down. Rally day or long liquidation.
6. **b-shape Day** — Heavy volume at bottom, long tail up. Sell-off or short covering.

Implementation:
- Split data into individual trading days
- For each day: compute IB, VA, range, extension, shape
- Classify using IB/range ratio, extension direction, profile shape
- Track last N days' types → pattern recognition

### Module 2: Opening Type Analysis (Dalton's 4 types)
**What**: Classify how the market opens relative to yesterday's VA.
**Why**: Opening type predicts the day's character within the first 30-60 minutes.

Types:
1. **Open Drive (OD)** — Opens outside VA and aggressively moves AWAY. Strongest signal. Don't fade.
2. **Open Test Drive (OTD)** — Opens outside VA, tests back to VA edge, then drives away. Confirms direction.
3. **Open Rejection Reverse (ORR)** — Opens outside VA, tries to extend, fails, reverses INTO VA. Fade signal.
4. **Open Auction (OA)** — Opens inside VA, auctions within. Balanced, wait for breakout.

Implementation:
- Detect yesterday's VA (from previous day's 15m/30m bars)
- Check where today opened relative to yesterday's VA
- Track first 30-60 min price action to classify
- Need at minimum 2 days of intraday data

### Module 3: Rotation Factor & Market Balance
**What**: Count how many times price rotates between VA extremes.
**Why**: High rotation = balanced/range day. Low rotation + directional = trend day.

Implementation:
- Track each time price crosses from upper half to lower half of VA (or vice versa)
- Rotation Factor = number of half-rotations per session
- RF > 4 = very balanced, fade extremes
- RF < 2 = directional, follow momentum
- Also: time in VA vs time outside VA ratio

### Module 4: Close Location & Follow-Through
**What**: Where price closes relative to day's VP levels.
**Why**: Close location is the BEST predictor of next-day direction.

Close types:
- **Close in VA** → likely balance continuation
- **Close above VAH** → bullish continuation probability high
- **Close below VAL** → bearish continuation probability high
- **Close at POC** → maximum uncertainty
- **Close in upper/lower 25%** of range → directional bias

Follow-through analysis:
- Track close location for last N days
- Calculate empirical follow-through rate
- "Last 5 days closed above VAH → 80% bullish continuation"

### Module 5: Multi-Day Auction Migration & MGI
**What**: Track how value (POC/VA) migrates across multiple days.
**Why**: This is the CORE institutional view — where is value going?

Sub-components:
1. **Value Migration Map** — POC position for last N days, direction & velocity
   - POC rising 3+ days = strong bullish auction
   - POC oscillating = balanced, range
   - POC declining = bearish auction

2. **Developing Value Area** — VA boundaries per day overlaid
   - Expanding VA = increasing acceptance at current level
   - Contracting VA = indecision, breakout pending
   - Migrating VA = directional value discovery

3. **Market-Generated Information (MGI)**
   - When price breaks out of VA: is it ACCEPTED (stays out 2+ bars) or REJECTED (returns)?
   - Acceptance = new value being discovered → follow
   - Rejection = responsive activity → fade
   - Track acceptance/rejection rate per level

4. **Composite Value Area Context**
   - Where is current price relative to WEEKLY and MONTHLY composite VA?
   - Inside composite VA = long-term balance
   - Outside = long-term value discovery

---

## UI Structure

```
/vp EUR → 🏛 Auction (current button)
  Now becomes a sub-menu:

🏛 Auction Market Theory
├── 📊 Current State     — auction state + strategy
├── 📅 Day Type          — today's classification + last 5 days
├── 🌅 Opening           — today's opening type + implication
├── 🔄 Rotation          — rotation factor + balance score
├── 🌙 Close Analysis    — yesterday's close + follow-through stats
├── 📈 Value Migration   — multi-day POC/VA migration map
├── ⚡ MGI Signals       — acceptance/rejection at key levels
├── 📋 Full AMT Report   — everything synthesized
```

OR: keep it as 1 enhanced "Auction" mode that includes ALL of the above in a comprehensive output.

**Recommendation**: Single enhanced mode. Too many sub-menus = user fatigue. One "🏛 Auction" click should give the FULL institutional AMT picture with sections.

---

## Estimated Effort
| Module | Complexity | Time |
|--------|-----------|------|
| Day Type Classification | Medium | 30 min |
| Opening Type Analysis | Medium | 25 min |
| Rotation Factor | Easy | 15 min |
| Close Location | Easy | 15 min |
| Multi-Day Migration + MGI | High | 40 min |
| Chart upgrade | Medium | 20 min |
| **Total** | | **~2.5 hours** |

## Feasibility Notes
- All feasible with current data (15m/30m bars have ~52 days = plenty for multi-day analysis)
- No new data sources needed
- Need to split bars into per-day groups → use trade_date grouping (already in naked POC code)
- 30m bars are ideal for TPO letters (standard Market Profile convention)
- Opening type needs previous day VA → requires at least 2 days of bars
