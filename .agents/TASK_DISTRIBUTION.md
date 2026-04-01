# Task Distribution — Option B (Dev-A Review-Focused)

> Updated: 2026-04-01
> Total remaining: 140 tasks
> Strategy: Dev-A fokus review + quality, Dev-B/C high-volume implementors

---

## Dev-A (Senior Developer + Reviewer) — 22 tasks + ALL PR reviews

### Refactor / Code Quality (12)
- TASK-043: Error sentinel package
- TASK-067: Config cross-validation
- TASK-068: Structured log component
- TASK-069: Command latency middleware
- TASK-072: Dead code quant handler
- TASK-090: fmt.Printf to zerolog migration
- TASK-094: DI framework evaluation
- TASK-188: Consolidate duplicate ICT
- TASK-190: Test coverage COT/FRED critical
- TASK-191: Error sentinel package (v2)
- TASK-192: Config startup validation
- TASK-193: Formatter consolidation fmtutil

### Trading Engine — Critical (10)
- TASK-014: Estimated delta orderflow
- TASK-110: Market regime overlay engine
- TASK-113: Confluence score v2 unified signal
- TASK-114: Session analysis engine
- TASK-135: Volatility cone analysis
- TASK-136: Carry trade monitor/unwind
- TASK-138: Proactive regime change alert
- TASK-139: Multi-strategy backtester
- TASK-160: Elliott wave auto counter
- TASK-164: IV skew/smile analysis

### Additional Duties
- Review ALL PRs from Dev-B and Dev-C
- Code audit after each merge batch
- Conflict resolution for rebases

---

## Dev-B — 47 tasks

### Trading Engine (22)
- TASK-015: Split formatter per domain (done via TECH-001, SKIP)
- TASK-016: Split handler per domain
- TASK-029: Daily briefing command
- TASK-033: Sentiment cache BadgerDB persistence
- TASK-039: Wyckoff phase detection (already have wyckoff service, may SKIP)
- TASK-052: Smart alerts per pair
- TASK-057: Fed speeches FOMC RSS monitor
- TASK-063: USD aggregate COT signal [CLAIMED by Dev-C, reassign]
- TASK-081: Fed speeches scraper (v2)
- TASK-082: WorldBank macro API (already have worldbank service, may SKIP)
- TASK-085: ICT FVG + OB in ta package (already in ict service, may SKIP)
- TASK-098: Impact recorder detached context
- TASK-099: Callback empty chatID guard
- TASK-111: Regime-aware correlation dashboard
- TASK-112: Risk parity position sizer
- TASK-137: Microstructure flow enhancement
- TASK-161: VWAP delta profile
- TASK-162: Cross-asset flow divergence
- TASK-163: Monte Carlo scenario generator
- TASK-185: AMT day type classification
- TASK-186: AMT opening type analysis
- TASK-189: AMT rotation/close/migration

### Data/API Integration (25)
- TASK-105: ECB SDW API integration
- TASK-106: SNB balance sheet FX intervention
- TASK-107: Treasury TIPS breakeven inflation
- TASK-108: OECD CLI leading indicators
- TASK-109: DTCC FX swap data
- TASK-130: Deribit IV surface/skew
- TASK-131: Deribit DVOL crypto VIX
- TASK-132: Deribit expanded assets SOL/AVAX
- TASK-133: TradingEconomics macro scraper
- TASK-134: Finviz cross-asset scraper
- TASK-155: BIS statistics API
- TASK-156: Bybit funding rate history
- TASK-157: EIA natural gas expansion
- TASK-158: Blockchain BTC on-chain metrics
- TASK-159: MOVE index bond volatility
- TASK-180: DeFiLlama TVL/DEX/stablecoin
- TASK-181: Coinmetrics exchange flows
- TASK-182: Eurostat EU macro data
- TASK-183: TreasuryDirect auction results
- TASK-184: Alternative.me extended crypto
- TASK-205: CBOE volatility index suite
- TASK-206: SEC EDGAR 13F institutional
- TASK-207: CryptoCompare exchange volume
- TASK-208: CBOE SKEW/VIX ratio alert
- TASK-209: CBOE OVX/GVZ cross-asset vol

---

## Dev-C — 70 tasks

### Bug Fix / Defensive (35)
- TASK-115: Bounded LRU user mutex
- TASK-116: chartPath defer cleanup quant/vp
- TASK-117: Context-aware retry Gemini
- TASK-118: HTTP client factory pooling
- TASK-119: Unified retry market data clients
- TASK-120: OBV return bounds guard
- TASK-121: Volatility avgATR zero guard
- TASK-122: Walk-forward empty slice safety
- TASK-123: Defensive slice bounds formatter
- TASK-124: NaN/Inf propagation guard
- TASK-140: Fix sprintf multipart API
- TASK-141: VIX fetcher EOF vs parse error
- TASK-142: VIX cache error propagation
- TASK-143: Log silenced errors bot handler
- TASK-144: interface→any cleanup
- TASK-145: GEX spot zero guard
- TASK-146: Deribit bookSummary expired filter
- TASK-147: Wyckoff phase boundary -1 guard
- TASK-148: VIX slopePct zero/NaN guard
- TASK-149: Circuit breaker negative duration race
- TASK-165: Panic recovery scheduler goroutines
- TASK-166: Goroutine pool bot dispatch
- TASK-167: Tempfile lifecycle Python subprocess
- TASK-168: Subprocess stderr capture fix
- TASK-169: HTTP transport connection pooling
- TASK-170: Correlation min datapoints guard
- TASK-171: HMM minimum returns boundary
- TASK-172: COT broadcast dedup guard
- TASK-173: FRED composites nil pointer guard
- TASK-174: Seasonal nil pointer new contracts
- TASK-195: Callback nil chatID guard
- TASK-196: Chart zero-byte validation
- TASK-197: BadgerDB dropAll timeout
- TASK-198: AI contentBlocks nil guard
- TASK-199: News scheduler TimeWIB zero guard

### UX / UI (29)
- TASK-031: BIS REER/NEER exchange rates
- TASK-051: Reaction feedback buttons
- TASK-076: Standardize back button language
- TASK-078: Pinned commands personalized keyboard
- TASK-100: Callback error user-friendly
- TASK-101: Unified session expired template
- TASK-102: Settings toggle confirmation toast
- TASK-103: Message chunk ID tracking
- TASK-104: Rate limit wait duration UX
- TASK-125: Help add wyckoff/shortcuts
- TASK-126: Educational glossary GEX/Wyckoff
- TASK-127: Language standardization new features
- TASK-128: Wyckoff keyboard navigation
- TASK-129: Error retry buttons all handlers
- TASK-150: Alert inline unsubscribe actionable
- TASK-151: Chart failure notification fallback
- TASK-152: Admin confirmation destructive actions
- TASK-153: AI chat token display fallback guidance
- TASK-154: Deep link start parameter handling
- TASK-175: Auto-reload last currency
- TASK-176: Default timeframe preference
- TASK-177: Related next-steps keyboard
- TASK-178: Multi-step progress Python subprocess
- TASK-179: Page indicator error retry buttons
- TASK-200: Mobile sparkline compact mode
- TASK-201: Emoji accessibility context
- TASK-202: Quiet hours alert granularity
- TASK-203: Multi-word command aliases
- TASK-204: Onboarding completion tracking

### Test Coverage (6)
- TASK-084: BIS REER test coverage
- TASK-088: Wyckoff phase detector tests
- TASK-089: Elliott wave swing labeler tests
- TASK-092: Test coverage microstructure/strategy
- TASK-097: SplitMessage unit tests
- TASK-194: Test coverage price/backtest

---

## Priority Order (per agent)

### Dev-A: Bug fixes first, then critical engines
1. TASK-190 (test coverage COT/FRED) — safety net before refactors
2. TASK-043 (error sentinels) — foundation
3. TASK-192 (config validation) — startup safety
4. TASK-113 (confluence score v2) — highest value engine
5. TASK-138 (regime change alert) — proactive feature

### Dev-B: Data integrations, then trading
1. TASK-016 (split handler.go) — high-impact refactor
2. TASK-105-109 (ECB, SNB, Treasury, OECD, DTCC) — institutional data
3. TASK-052 (smart alerts) — user-facing value
4. TASK-111 (regime correlation dashboard)
5. TASK-163 (Monte Carlo)

### Dev-C: Bug fixes first (quick wins), then UX
1. TASK-115-124 (defensive coding batch) — 10 quick fixes
2. TASK-140-149 (more defensive) — 10 more
3. TASK-165-174 (stability) — 10 more
4. TASK-125-129 (UX polish)
5. TASK-150-154 (UX advanced)
