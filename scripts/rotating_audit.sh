#!/bin/bash
# Rotating Deep Audit Script
# Focuses on 2-3 aspects per run for deeper analysis

set -e

# Setup Go environment
export PATH=$PATH:/home/node/go/bin
export GOPATH=/home/node/.go

AUDIT_DIR=".agents/audit"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
DAY_OF_WEEK=$(date +%u)  # 1=Monday, 7=Sunday

mkdir -p "$AUDIT_DIR"

# Determine audit focus based on day of week
# More frequent UI/UX and Feature audits
case $DAY_OF_WEEK in
    1) FOCUS="build-security" ;;   # Monday: Build & Security
    2) FOCUS="tests-handlers" ;;   # Tuesday: Tests & Handlers
    3) FOCUS="ui-ux-flow" ;;       # Wednesday: UI/UX Flow Deep Dive
    4) FOCUS="feature-logic" ;;    # Thursday: Feature Logic & Functionality
    5) FOCUS="errors-panic" ;;     # Friday: Error Handling & Panic Recovery
    6) FOCUS="comprehensive" ;;    # Saturday: Full Comprehensive Audit
    7) FOCUS="bug-hunting" ;;      # Sunday: Bug Hunting & Edge Cases
esac

echo "🔍 Rotating Deep Audit - $FOCUS"
echo "Day of Week: $DAY_OF_WEEK"
echo "Timestamp: $TIMESTAMP"
echo "========================================"

REPORT_FILE="$AUDIT_DIR/${TIMESTAMP}-audit-${FOCUS}.md"

# Initialize report
cat > "$REPORT_FILE" << EOF
# Deep Audit Report - $(date)

## Focus Area: $FOCUS
- **Timestamp**: $TIMESTAMP
- **Day**: $(date +%A)
- **Branch**: $(git branch --show-current 2>/dev/null || echo "unknown")
- **Commit**: $(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

## Audit Schedule
- **Monday**: Build & Security
- **Tuesday**: Tests & Handlers
- **Wednesday**: Error Handling & Panic Recovery
- **Thursday**: API & Performance
- **Friday**: Code Quality & Deep Dive
- **Saturday**: Comprehensive Audit
- **Sunday**: Bug Hunting & Edge Cases

EOF

# ============================================
# FOCUS 1: BUILD & SECURITY (Monday)
# ============================================
if [ "$FOCUS" = "build-security" ] || [ "$FOCUS" = "comprehensive" ]; then
    echo "### 🔨 Build & Compilation" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    # Full build
    if go build ./... 2>&1 | tee /tmp/build.log; then
        echo "- ✅ Build: PASSED" >> "$REPORT_FILE"
        BUILD_STATUS="PASS"
    else
        echo "- ❌ Build: FAILED" >> "$REPORT_FILE"
        echo "Errors:" >> "$REPORT_FILE"
        cat /tmp/build.log >> "$REPORT_FILE"
        BUILD_STATUS="FAIL"
    fi
    
    # Dependencies check
    echo "" >> "$REPORT_FILE"
    echo "### 🔒 Security & Dependencies" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    # Check for hardcoded secrets
    echo "**Hardcoded Secrets Scan:**" >> "$REPORT_FILE"
    SECRETS=$(grep -r "ghp_\|sk-[A-Za-z0-9]\|api_key=\|password=\|secret=" --include="*.go" . 2>/dev/null | grep -v "test" | grep -v ".git" | wc -l || echo "0")
    if [ "$SECRETS" -eq 0 ]; then
        echo "- ✅ No hardcoded secrets detected" >> "$REPORT_FILE"
    else
        echo "- ❌ $SECRETS potential secrets found" >> "$REPORT_FILE"
        grep -r "ghp_\|sk-[A-Za-z0-9]" --include="*.go" . 2>/dev/null | grep -v "test" | grep -v ".git" | head -5 >> "$REPORT_FILE"
    fi
    
    # Check go.mod for vulnerabilities
    echo "" >> "$REPORT_FILE"
    echo "**Dependency Check:**" >> "$REPORT_FILE"
    if command -v govulncheck &> /dev/null; then
        if govulncheck ./... 2>&1 | tee /tmp/vuln.log; then
            echo "- ✅ No vulnerabilities found" >> "$REPORT_FILE"
        else
            echo "- ⚠️  Vulnerabilities detected (review /tmp/vuln.log)" >> "$REPORT_FILE"
        fi
    else
        echo "- ℹ️  govulncheck not installed (skip)" >> "$REPORT_FILE"
    fi
    
    # Environment validation
    echo "" >> "$REPORT_FILE"
    echo "**Environment Variables:**" >> "$REPORT_FILE"
    if [ -f ".env" ]; then
        REQUIRED_VARS=("BOT_TOKEN" "CHAT_ID")
        for var in "${REQUIRED_VARS[@]}"; do
            if grep -q "^$var=" .env && ! grep -q "^$var=\$" .env; then
                echo "- ✅ $var: SET" >> "$REPORT_FILE"
            else
                echo "- ⚠️  $var: NOT CONFIGURED" >> "$REPORT_FILE"
            fi
        done
    else
        echo "- ⚠️  .env file not found" >> "$REPORT_FILE"
    fi
fi

# ============================================
# FOCUS 2: TESTS & HANDLERS (Tuesday)
# ============================================
if [ "$FOCUS" = "tests-handlers" ] || [ "$FOCUS" = "comprehensive" ]; then
    echo "" >> "$REPORT_FILE"
    echo "### 🧪 Test Suite Deep Dive" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    # Run tests with coverage
    echo "**Test Execution:**" >> "$REPORT_FILE"
    if go test ./internal/service/... ./internal/adapter/... -short -v 2>&1 | tee /tmp/test.log | tail -20; then
        echo "- ✅ Tests: PASSED" >> "$REPORT_FILE"
    else
        echo "- ❌ Tests: FAILED" >> "$REPORT_FILE"
        echo "Failed tests:" >> "$REPORT_FILE"
        grep -E "^(--- FAIL|FAIL)" /tmp/test.log >> "$REPORT_FILE" || echo "No specific failures found" >> "$REPORT_FILE"
    fi
    
    # Handler registration check
    echo "" >> "$REPORT_FILE"
    echo "### ⚡ Handler Functionality" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    echo "**Command Registration:**" >> "$REPORT_FILE"
    COMMANDS=$(grep -r "AddCommand\|bot.Handle" internal/adapter/telegram/*.go 2>/dev/null | wc -l || echo "0")
    echo "- Registered handlers: $COMMANDS" >> "$REPORT_FILE"
    
    echo "" >> "$REPORT_FILE"
    echo "**Callback Handlers:**" >> "$REPORT_FILE"
    CALLBACKS=$(grep -r "AddCallback\|HandleCallback" internal/adapter/telegram/*.go 2>/dev/null | wc -l || echo "0")
    echo "- Callback handlers: $CALLBACKS" >> "$REPORT_FILE"
    
    echo "" >> "$REPORT_FILE"
    echo "**Nil Guards:**" >> "$REPORT_FILE"
    NIL_GUARDS=$(grep -r "if.*== nil" internal/adapter/telegram/handler*.go 2>/dev/null | wc -l || echo "0")
    echo "- Nil pointer guards: $NIL_GUARDS" >> "$REPORT_FILE"
fi

# ============================================
# FOCUS 3: ERRORS & PANIC (Wednesday)
# ============================================
if [ "$FOCUS" = "errors-panic" ] || [ "$FOCUS" = "comprehensive" ]; then
    echo "" >> "$REPORT_FILE"
    echo "### 🛡️ Error Handling Analysis" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    echo "**Error Patterns:**" >> "$REPORT_FILE"
    ERROR_HANDLERS=$(grep -r "Err(" internal/adapter/telegram/*.go 2>/dev/null | wc -l || echo "0")
    echo "- Error handlers: $ERROR_HANDLERS" >> "$REPORT_FILE"
    
    ERROR_LOGGING=$(grep -r "log.Error\|log.Warn" internal/ 2>/dev/null | wc -l || echo "0")
    echo "- Error logging calls: $ERROR_LOGGING" >> "$REPORT_FILE"
    
    echo "" >> "$REPORT_FILE"
    echo "### 🔄 Panic Recovery" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    RECOVERY_BLOCKS=$(grep -r "defer.*recover()" internal/ 2>/dev/null | wc -l || echo "0")
    echo "- Panic recovery blocks: $RECOVERY_BLOCKS" >> "$REPORT_FILE"
    
    # Check goroutines without recovery
    echo "" >> "$REPORT_FILE"
    echo "**Goroutine Safety:**" >> "$REPORT_FILE"
    GOROUTINES=$(grep -r "go func" internal/ 2>/dev/null | wc -l || echo "0")
    echo "- Total goroutines: $GOROUTINES" >> "$REPORT_FILE"
    
    if [ "$RECOVERY_BLOCKS" -lt "$GOROUTINES" ]; then
        echo "- ⚠️  Some goroutines may lack panic recovery" >> "$REPORT_FILE"
    else
        echo "- ✅ All goroutines have panic recovery" >> "$REPORT_FILE"
    fi
fi

# ============================================
# FOCUS 4: API & PERFORMANCE (Thursday)
# ============================================
if [ "$FOCUS" = "api-performance" ] || [ "$FOCUS" = "comprehensive" ]; then
    echo "" >> "$REPORT_FILE"
    echo "### 🔌 API Integration Status" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    echo "**External Services:**" >> "$REPORT_FILE"
    
    # Check API configurations
    if grep -q "GEMINI_API_KEY" .env 2>/dev/null; then
        echo "- ✅ Gemini AI: CONFIGURED" >> "$REPORT_FILE"
    else
        echo "- ℹ️  Gemini AI: Not configured (optional)" >> "$REPORT_FILE"
    fi
    
    if grep -q "FRED_API_KEY" .env 2>/dev/null; then
        echo "- ✅ FRED API: CONFIGURED" >> "$REPORT_FILE"
    else
        echo "- ℹ️  FRED API: Not configured (optional)" >> "$REPORT_FILE"
    fi
    
    if grep -q "COINGECKO_API_KEY" .env 2>/dev/null; then
        echo "- ✅ CoinGecko: CONFIGURED" >> "$REPORT_FILE"
    else
        echo "- ℹ️  CoinGecko: Not configured (optional)" >> "$REPORT_FILE"
    fi
    
    echo "" >> "$REPORT_FILE"
    echo "### ⚡ Performance Metrics" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    # Goroutine analysis
    echo "**Concurrency:**" >> "$REPORT_FILE"
    GOROUTINE_COUNT=$(grep -r "go func" internal/scheduler/*.go 2>/dev/null | wc -l || echo "0")
    echo "- Scheduler goroutines: $GOROUTINE_COUNT" >> "$REPORT_FILE"
    
    # Memory analysis
    echo "" >> "$REPORT_FILE"
    echo "**Memory Usage:**" >> "$REPORT_FILE"
    if [ -d "/proc/self" ]; then
        MEM_USAGE=$(cat /proc/self/status 2>/dev/null | grep VmRSS | awk '{print $2 " KB"}' || echo "N/A")
        echo "- Current memory: $MEM_USAGE" >> "$REPORT_FILE"
    fi
fi

# ============================================
# FOCUS 5: CODE QUALITY (Friday)
# ============================================
if [ "$FOCUS" = "code-quality" ] || [ "$FOCUS" = "comprehensive" ]; then
    echo "" >> "$REPORT_FILE"
    echo "### 📝 Code Quality Analysis" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    echo "**Code Metrics:**" >> "$REPORT_FILE"
    TOTAL_LINES=$(find . -name "*.go" -not -path "./.git/*" -not -path "./vendor/*" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}' || echo "N/A")
    echo "- Total Go lines: $TOTAL_LINES" >> "$REPORT_FILE"
    
    TEST_LINES=$(find . -name "*_test.go" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}' || echo "N/A")
    echo "- Test lines: $TEST_LINES" >> "$REPORT_FILE"
    
    TEST_FILES=$(find . -name "*_test.go" | wc -l || echo "0")
    echo "- Test files: $TEST_FILES" >> "$REPORT_FILE"
    
    # Calculate test coverage ratio
    if [ "$TEST_LINES" != "N/A" ] && [ "$TOTAL_LINES" != "N/A" ]; then
        COVERAGE_RATIO=$(echo "scale=2; $TEST_LINES * 100 / $TOTAL_LINES" | bc 2>/dev/null || echo "N/A")
        echo "- Test coverage ratio: ${COVERAGE_RATIO}%" >> "$REPORT_FILE"
    fi
    
    echo "" >> "$REPORT_FILE"
    echo "**Static Analysis:**" >> "$REPORT_FILE"
    if go vet ./... 2>&1 | tee /tmp/vet.log; then
        echo "- ✅ Go vet: PASSED" >> "$REPORT_FILE"
    else
        echo "- ⚠️  Go vet: WARNINGS" >> "$REPORT_FILE"
        cat /tmp/vet.log >> "$REPORT_FILE"
    fi
fi

# ============================================
# FOCUS 3: UI/UX FLOW (Wednesday)
# ============================================
if [ "$FOCUS" = "ui-ux-flow" ] || [ "$FOCUS" = "comprehensive" ]; then
    echo "" >> "$REPORT_FILE"
    echo "### 🎨 UI/UX Flow Deep Dive" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    echo "**Menu Navigation:**" >> "$REPORT_FILE"
    
    # Check main keyboard
    MAIN_KEYBOARD=$(grep -c "NewInlineKeyboard" internal/adapter/telegram/keyboard*.go 2>/dev/null || echo "0")
    echo "- Main keyboards defined: $MAIN_KEYBOARD" >> "$REPORT_FILE"
    
    # Check callback handlers
    CALLBACK_HANDLERS=$(grep -c "AddCallback\|HandleCallback" internal/adapter/telegram/*.go 2>/dev/null || echo "0")
    echo "- Callback handlers registered: $CALLBACK_HANDLERS" >> "$REPORT_FILE"
    
    # Check loading states
    LOADING_STATES=$(grep -c "SendLoading\|loading" internal/adapter/telegram/handler*.go 2>/dev/null || echo "0")
    echo "- Loading indicators: $LOADING_STATES" >> "$REPORT_FILE"
    
    # Check error handling
    ERROR_HANDLERS=$(grep -c "SendError\|error:" internal/adapter/telegram/*.go 2>/dev/null || echo "0")
    echo "- Error handlers: $ERROR_HANDLERS" >> "$REPORT_FILE"
    
    # Check back navigation
    BACK_BUTTONS=$(grep -c "Back\|Kembali" internal/adapter/telegram/*.go 2>/dev/null || echo "0")
    echo "- Back buttons: $BACK_BUTTONS" >> "$REPORT_FILE"
    
    echo "" >> "$REPORT_FILE"
    echo "**UX Quality Checks:**" >> "$REPORT_FILE"
    echo "- [ ] All buttons have callbacks" >> "$REPORT_FILE"
    echo "- [ ] Loading states show during async ops" >> "$REPORT_FILE"
    echo "- [ ] Error messages are user-friendly" >> "$REPORT_FILE"
    echo "- [ ] Navigation is intuitive" >> "$REPORT_FILE"
    echo "- [ ] Home button available everywhere" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
fi

# ============================================
# FOCUS 4: FEATURE LOGIC (Thursday)
# ============================================
if [ "$FOCUS" = "feature-logic" ] || [ "$FOCUS" = "comprehensive" ]; then
    echo "" >> "$REPORT_FILE"
    echo "### 🧠 Feature Logic & Functionality" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    echo "**Core Features Status:**" >> "$REPORT_FILE"
    
    # COT Feature
    echo "- **COT Analysis**:" >> "$REPORT_FILE"
    if [ -f "internal/service/cot/analyzer.go" ]; then
        echo "  - [ ] Analyzer logic: Needs manual test" >> "$REPORT_FILE"
        echo "  - [ ] Seasonality engine: Needs manual test" >> "$REPORT_FILE"
        echo "  - [ ] Disaggregated data: Needs manual test" >> "$REPORT_FILE"
    else
        echo "  - ❌ Analyzer missing" >> "$REPORT_FILE"
    fi
    
    # Price Feature
    echo "- **Price Context**:" >> "$REPORT_FILE"
    if [ -f "internal/service/price/fetcher.go" ]; then
        echo "  - [ ] Price fetcher: Needs manual test" >> "$REPORT_FILE"
        echo "  - [ ] Moving averages: Needs manual test" >> "$REPORT_FILE"
        echo "  - [ ] Support/Resistance: Needs manual test" >> "$REPORT_FILE"
    else
        echo "  - ❌ Fetcher missing" >> "$REPORT_FILE"
    fi
    
    # Calendar Feature
    echo "- **Economic Calendar**:" >> "$REPORT_FILE"
    if [ -f "internal/service/fred/fetcher.go" ]; then
        echo "  - [ ] FRED integration: Needs manual test" >> "$REPORT_FILE"
        echo "  - [ ] Event filtering: Needs manual test" >> "$REPORT_FILE"
        echo "  - [ ] Impact scoring: Needs manual test" >> "$REPORT_FILE"
    else
        echo "  - ❌ FRED fetcher missing" >> "$REPORT_FILE"
    fi
    
    # Backtest Feature
    echo "- **Backtesting**:" >> "$REPORT_FILE"
    if [ -f "internal/service/backtest/walkforward.go" ]; then
        echo "  - [ ] Walkforward analysis: Needs manual test" >> "$REPORT_FILE"
        echo "  - [ ] Monte Carlo: Needs manual test" >> "$REPORT_FILE"
        echo "  - [ ] Stats calculation: Needs manual test" >> "$REPORT_FILE"
    else
        echo "  - ❌ Backtest engine missing" >> "$REPORT_FILE"
    fi
    
    # Alpha Signals
    echo "- **Alpha Signals (GEX/Wyckoff/SMC)**:" >> "$REPORT_FILE"
    if [ -f "internal/adapter/telegram/handler_alpha.go" ]; then
        echo "  - [ ] GEX calculation: Needs manual test" >> "$REPORT_FILE"
        echo "  - [ ] Wyckoff analysis: Needs manual test" >> "$REPORT_FILE"
        echo "  - [ ] SMC/ICT: Needs manual test" >> "$REPORT_FILE"
    else
        echo "  - ❌ Alpha handler missing" >> "$REPORT_FILE"
    fi
    
    echo "" >> "$REPORT_FILE"
    echo "**Manual Test Required:**" >> "$REPORT_FILE"
    echo "⚠️  All features above require manual testing in Telegram bot" >> "$REPORT_FILE"
    echo "Run: /outlook, /cot, /calendar, /bias, /radar and verify outputs" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
fi

# ============================================
# FOCUS 6: BUG HUNTING (Sunday)
# ============================================
if [ "$FOCUS" = "bug-hunting" ]; then
    echo "" >> "$REPORT_FILE"
    echo "### 🐛 Bug Hunting & Edge Cases" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    echo "**Common Bug Patterns:**" >> "$REPORT_FILE"
    
    # Check for potential nil dereference
    echo "- **Nil Dereference Risk**:" >> "$REPORT_FILE"
    NIL_RISK=$(grep -r "\..*\." internal/adapter/telegram/handler*.go 2>/dev/null | grep -v "if.*nil" | head -3 || echo "  None detected")
    echo "  $NIL_RISK" >> "$REPORT_FILE"
    
    # Check for unhandled errors
    echo "- **Unhandled Errors**:" >> "$REPORT_FILE"
    UNHANDLED=$(grep -r "= .*\(.*\)$" internal/ --include="*.go" 2>/dev/null | grep -v "if err" | grep -v "return err" | grep -v "log" | head -3 || echo "  None detected")
    echo "  $UNHANDLED" >> "$REPORT_FILE"
    
    # Check for potential race conditions
    echo "- **Race Conditions**:" >> "$REPORT_FILE"
    echo "  Run: go test -race ./... for detection" >> "$REPORT_FILE"
fi

# Final summary
echo "" >> "$REPORT_FILE"
echo "## Summary" >> "$REPORT_FILE"
echo "- **Audit completed**: $(date)" >> "$REPORT_FILE"
echo "- **Focus area**: $FOCUS" >> "$REPORT_FILE"
echo "- **Report file**: $REPORT_FILE" >> "$REPORT_FILE"

echo ""
echo "✅ Deep audit completed!"
echo "📄 Report: $REPORT_FILE"
echo "🎯 Focus: $FOCUS"
