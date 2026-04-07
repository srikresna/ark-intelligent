#!/bin/bash
# Feature Functionality Audit
# Tests if features actually work end-to-end

set -e

export PATH=$PATH:/home/node/go/bin
export GOPATH=/home/node/.go

AUDIT_DIR=".agents/audit"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
mkdir -p "$AUDIT_DIR"

REPORT_FILE="$AUDIT_DIR/${TIMESTAMP}-audit-features.md"

echo "🔍 Feature Functionality Audit"
echo "================================"

cat > "$REPORT_FILE" << EOF
# Feature Functionality Audit - $(date)

## Overview
- **Timestamp**: $TIMESTAMP
- **Branch**: $(git branch --show-current 2>/dev/null || echo "unknown")
- **Commit**: $(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

## Feature Categories Audited
EOF

# ============================================
# COMMAND HANDLER AUDIT
# ============================================
echo "### ⚡ Command Handler Functionality" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# List all command handlers
echo "**Registered Commands:**" >> "$REPORT_FILE"
COMMANDS=$(grep -r "bot.Handle\|AddCommand" internal/adapter/telegram/*.go 2>/dev/null | grep -oP '(?<=Handle["\x27])[^"\x27]+' | sort -u || echo "None found")
if [ -n "$COMMANDS" ]; then
    echo "$COMMANDS" | while read cmd; do
        echo "- [ ] \`/$cmd\` - Need manual test" >> "$REPORT_FILE"
    done
else
    echo "- No commands found (check handler registration)" >> "$REPORT_FILE"
fi

echo "" >> "$REPORT_FILE"

# ============================================
# FEATURE LOGIC AUDIT
# ============================================
echo "### 🧠 Feature Logic Validation" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

echo "**Core Features:**" >> "$REPORT_FILE"

# COT Feature
echo "- **COT Analysis**:" >> "$REPORT_FILE"
if [ -f "internal/service/cot/analyzer.go" ]; then
    echo "  - [ ] Analyzer logic: Need verification" >> "$REPORT_FILE"
    echo "  - [ ] Seasonality engine: Need verification" >> "$REPORT_FILE"
    echo "  - [ ] Disaggregated data: Need verification" >> "$REPORT_FILE"
else
    echo "  - ❌ Analyzer not found" >> "$REPORT_FILE"
fi

# Price Feature
echo "- **Price Context**:" >> "$REPORT_FILE"
if [ -f "internal/service/price/fetcher.go" ]; then
    echo "  - [ ] Price fetcher: Need verification" >> "$REPORT_FILE"
    echo "  - [ ] Moving averages: Need verification" >> "$REPORT_FILE"
    echo "  - [ ] Support/Resistance: Need verification" >> "$REPORT_FILE"
else
    echo "  - ❌ Fetcher not found" >> "$REPORT_FILE"
fi

# Calendar Feature
echo "- **Economic Calendar**:" >> "$REPORT_FILE"
if [ -f "internal/service/fred/fetcher.go" ]; then
    echo "  - [ ] FRED integration: Need verification" >> "$REPORT_FILE"
    echo "  - [ ] Event filtering: Need verification" >> "$REPORT_FILE"
    echo "  - [ ] Impact scoring: Need verification" >> "$REPORT_FILE"
else
    echo "  - ❌ FRED fetcher not found" >> "$REPORT_FILE"
fi

# Backtest Feature
echo "- **Backtesting**:" >> "$REPORT_FILE"
if [ -f "internal/service/backtest/walkforward.go" ]; then
    echo "  - [ ] Walkforward analysis: Need verification" >> "$REPORT_FILE"
    echo "  - [ ] Monte Carlo: Need verification" >> "$REPORT_FILE"
    echo "  - [ ] Stats calculation: Need verification" >> "$REPORT_FILE"
else
    echo "  - ❌ Backtest engine not found" >> "$REPORT_FILE"
fi

# Sentiment Feature
echo "- **Sentiment Analysis**:" >> "$REPORT_FILE"
if [ -f "internal/service/sentiment/sentiment.go" ]; then
    echo "  - [ ] AAII sentiment: Need verification" >> "$REPORT_FILE"
    echo "  - [ ] News sentiment: Need verification" >> "$REPORT_FILE"
    echo "  - [ ] Cache layer: Need verification" >> "$REPORT_FILE"
else
    echo "  - ❌ Sentiment service not found" >> "$REPORT_FILE"
fi

# Alpha Signals
echo "- **Alpha Signals**:" >> "$REPORT_FILE"
if [ -f "internal/adapter/telegram/handler_alpha.go" ]; then
    echo "  - [ ] GEX calculation: Need verification" >> "$REPORT_FILE"
    echo "  - [ ] Wyckoff analysis: Need verification" >> "$REPORT_FILE"
    echo "  - [ ] SMC/ICT: Need verification" >> "$REPORT_FILE"
else
    echo "  - ❌ Alpha handler not found" >> "$REPORT_FILE"
fi

echo "" >> "$REPORT_FILE"

# ============================================
# INTEGRATION POINTS
# ============================================
echo "### 🔌 External Integrations" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

echo "**API Status:**" >> "$REPORT_FILE"

# Telegram
echo "- **Telegram Bot**:" >> "$REPORT_FILE"
if grep -q "BOT_TOKEN" .env 2>/dev/null && ! grep -q "BOT_TOKEN=\$" .env 2>/dev/null; then
    echo "  - ✅ Configured" >> "$REPORT_FILE"
else
    echo "  - ⚠️  Not configured" >> "$REPORT_FILE"
fi

# Gemini AI
echo "- **Gemini AI**:" >> "$REPORT_FILE"
if grep -q "GEMINI_API_KEY" .env 2>/dev/null && ! grep -q "GEMINI_API_KEY=\$" .env 2>/dev/null; then
    echo "  - ✅ Configured" >> "$REPORT_FILE"
else
    echo "  - ⚠️  Not configured (optional)" >> "$REPORT_FILE"
fi

# FRED
echo "- **FRED API**:" >> "$REPORT_FILE"
if grep -q "FRED_API_KEY" .env 2>/dev/null && ! grep -q "FRED_API_KEY=\$" .env 2>/dev/null; then
    echo "  - ✅ Configured" >> "$REPORT_FILE"
else
    echo "  - ⚠️  Not configured (optional)" >> "$REPORT_FILE"
fi

# CoinGecko
echo "- **CoinGecko**:" >> "$REPORT_FILE"
if grep -q "COINGECKO_API_KEY" .env 2>/dev/null && ! grep -q "COINGECKO_API_KEY=\$" .env 2>/dev/null; then
    echo "  - ✅ Configured" >> "$REPORT_FILE"
else
    echo "  - ⚠️  Not configured (optional)" >> "$REPORT_FILE"
fi

echo "" >> "$REPORT_FILE"

# ============================================
# UI/UX FLOW AUDIT
# ============================================
echo "### 🎨 UI/UX Flow Audit" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

echo "**Navigation Flow:**" >> "$REPORT_FILE"

# Check keyboard handlers
echo "- **Main Menu**:" >> "$REPORT_FILE"
if grep -q "NewInlineKeyboard" internal/adapter/telegram/keyboard*.go 2>/dev/null; then
    KEYBOARD_COUNT=$(grep -c "NewInlineKeyboard" internal/adapter/telegram/keyboard*.go 2>/dev/null || echo "0")
    echo "  - [ ] Main keyboard: $KEYBOARD_COUNT found" >> "$REPORT_FILE"
else
    echo "  - ❌ No keyboard found" >> "$REPORT_FILE"
fi

# Check callback handlers
echo "- **Callback Routes**:" >> "$REPORT_FILE"
CALLBACKS=$(grep -c "AddCallback\|HandleCallback" internal/adapter/telegram/*.go 2>/dev/null || echo "0")
echo "  - [ ] Registered callbacks: $CALLBACKS" >> "$REPORT_FILE"

# Check loading indicators
echo "- **Loading States**:" >> "$REPORT_FILE"
LOADING=$(grep -c "SendLoading\|loading" internal/adapter/telegram/handler*.go 2>/dev/null || echo "0")
echo "  - [ ] Loading indicators: $LOADING found" >> "$REPORT_FILE"

# Check error handling
echo "- **Error Messages**:" >> "$REPORT_FILE"
ERROR_MSG=$(grep -c "SendError\|error:" internal/adapter/telegram/*.go 2>/dev/null || echo "0")
echo "  - [ ] Error handlers: $ERROR_MSG found" >> "$REPORT_FILE"

echo "" >> "$REPORT_FILE"

# ============================================
# MANUAL TEST CHECKLIST
# ============================================
echo "### 🧪 Manual Test Checklist" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

echo "**Must Test Manually:**" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "1. **Onboarding**:" >> "$REPORT_FILE"
echo "   - [ ] /start command works" >> "$REPORT_FILE"
echo "   - [ ] Welcome message displays" >> "$REPORT_FILE"
echo "   - [ ] Main menu keyboard shows" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "2. **Core Commands**:" >> "$REPORT_FILE"
echo "   - [ ] /outlook returns market outlook" >> "$REPORT_FILE"
echo "   - [ ] /cot shows COT data" >> "$REPORT_FILE"
echo "   - [ ] /calendar shows events" >> "$REPORT_FILE"
echo "   - [ ] /bias shows market bias" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "3. **Navigation**:" >> "$REPORT_FILE"
echo "   - [ ] All buttons respond" >> "$REPORT_FILE"
echo "   - [ ] Back buttons work" >> "$REPORT_FILE"
echo "   - [ ] Home button returns to main" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "4. **Loading & Errors**:" >> "$REPORT_FILE"
echo "   - [ ] Loading indicator shows during long ops" >> "$REPORT_FILE"
echo "   - [ ] Error messages are user-friendly" >> "$REPORT_FILE"
echo "   - [ ] Retry buttons work" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# ============================================
# SUMMARY
# ============================================
echo "## Summary" >> "$REPORT_FILE"
echo "- **Audit completed**: $(date)" >> "$REPORT_FILE"
echo "- **Report file**: $REPORT_FILE" >> "$REPORT_FILE"
echo "- **Action items**: Review checklist and test manually" >> "$REPORT_FILE"

echo ""
echo "✅ Feature audit completed!"
echo "📄 Report: $REPORT_FILE"
echo ""
echo "⚠️  IMPORTANT: Many items require MANUAL testing in Telegram!"
echo "   Open the bot and test each feature listed in the report."
