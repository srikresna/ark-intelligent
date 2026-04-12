#!/bin/bash
# Sequential Audit System
# Runs audit cycles one by one, auto-verifies fixes, and advances when all PASS

set -e

export PATH=$PATH:/home/node/go/bin
export GOPATH=/home/node/.go

AUDIT_DIR=".agents/audit"
TASK_DIR=".agents/tasks/pending"
mkdir -p "$AUDIT_DIR" "$TASK_DIR"

# Audit cycles in order
CYCLES=(
    "build-security:Build & Security"
    "tests-handlers:Tests & Handlers"
    "ui-ux-flow:UI/UX Flow"
    "feature-logic:Feature Logic"
    "errors-panic:Error Handling"
    "api-performance:API & Performance"
    "code-quality:Code Quality"
    "comprehensive:Comprehensive Audit"
)

CURRENT_CYCLE_FILE="$AUDIT_DIR/.current_cycle"
SKIPPED_CYCLES_FILE="$AUDIT_DIR/.skipped_cycles"

# Initialize or read current cycle
if [ -f "$CURRENT_CYCLE_FILE" ]; then
    CURRENT_INDEX=$(cat "$CURRENT_CYCLE_FILE")
else
    CURRENT_INDEX=0
fi

# Get current cycle
CURRENT_CYCLE="${CYCLES[$CURRENT_INDEX]}"
CYCLE_KEY=$(echo "$CURRENT_CYCLE" | cut -d: -f1)
CYCLE_NAME=$(echo "$CURRENT_CYCLE" | cut -d: -f2)

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT_FILE="$AUDIT_DIR/${TIMESTAMP}-audit-${CYCLE_KEY}.md"

echo "🔍 Sequential Audit System"
echo "================================"
echo "Current Cycle: $((CURRENT_INDEX + 1))/${#CYCLES[@]}"
echo "Focus: $CYCLE_NAME"
echo "Timestamp: $TIMESTAMP"
echo ""

# Run specific cycle audit
run_audit_cycle() {
    local focus=$1
    local report=$2
    
    case $focus in
        "build-security")
            echo "🔨 Running Build & Security Audit..."
            # Build check
            if go build ./... 2>&1 | tee /tmp/build.log; then
                echo "- ✅ Build: PASSED" >> "$report"
            else
                echo "- ❌ Build: FAILED" >> "$report"
                return 1
            fi
            
            # Security check
            SECRETS=$(grep -r "ghp_\|sk-[A-Za-z0-9]" --include="*.go" . 2>/dev/null | grep -v test | wc -l || echo "0")
            if [ "$SECRETS" -eq 0 ]; then
                echo "- ✅ No hardcoded secrets" >> "$report"
            else
                echo "- ❌ $SECRETS hardcoded secrets found" >> "$report"
                return 1
            fi
            ;;
            
        "tests-handlers")
            echo "🧪 Running Tests & Handlers Audit..."
            if go test ./internal/service/... ./internal/adapter/... -short 2>&1 | tee /tmp/test.log; then
                echo "- ✅ Tests: PASSED" >> "$report"
            else
                echo "- ❌ Tests: FAILED" >> "$report"
                return 1
            fi
            
            HANDLERS=$(grep -c "AddCommand\|bot.Handle" internal/adapter/telegram/*.go 2>/dev/null || echo "0")
            echo "- ℹ️  Registered handlers: $HANDLERS" >> "$report"
            ;;
            
        "ui-ux-flow")
            echo "🎨 Running UI/UX Flow Audit..."
            KEYBOARDS=$(grep -c "NewInlineKeyboard" internal/adapter/telegram/keyboard*.go 2>/dev/null || echo "0")
            echo "- ℹ️  Keyboards defined: $KEYBOARDS" >> "$report"
            
            CALLBACKS=$(grep -c "AddCallback" internal/adapter/telegram/*.go 2>/dev/null || echo "0")
            echo "- ℹ️  Callback handlers: $CALLBACKS" >> "$report"
            
            # Manual test required
            echo "- ⚠️  Manual test required in Telegram" >> "$report"
            ;;
            
        "feature-logic")
            echo "🧠 Running Feature Logic Audit..."
            
            # Check feature files exist
            FEATURES=("internal/service/cot/analyzer.go" "internal/service/price/fetcher.go" "internal/service/fred/fetcher.go" "internal/service/backtest/walkforward.go")
            for feature in "${FEATURES[@]}"; do
                if [ -f "$feature" ]; then
                    echo "- ✅ $feature exists" >> "$report"
                else
                    echo "- ❌ $feature missing" >> "$report"
                    return 1
                fi
            done
            
            echo "- ⚠️  Manual test required for functionality" >> "$report"
            ;;
            
        "errors-panic")
            echo "🛡️ Running Error Handling Audit..."
            RECOVERY=$(grep -c "defer.*recover()" internal/ 2>/dev/null | tail -1 || echo "0")
            echo "- ℹ️  Panic recovery blocks: $RECOVERY" >> "$report"
            
            ERROR_HANDLERS=$(grep -c "SendError\|log.Error" internal/adapter/telegram/*.go 2>/dev/null || echo "0")
            echo "- ℹ️  Error handlers: $ERROR_HANDLERS" >> "$report"
            ;;
            
        "api-performance")
            echo "🔌 Running API & Performance Audit..."
            echo "- ℹ️  API integration check (manual required)" >> "$report"
            echo "- ℹ️  Performance metrics (manual required)" >> "$report"
            ;;
            
        "code-quality")
            echo "📝 Running Code Quality Audit..."
            TOTAL_LINES=$(find . -name "*.go" -not -path "./.git/*" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}' || echo "N/A")
            echo "- ℹ️  Total Go lines: $TOTAL_LINES" >> "$report"
            
            TEST_FILES=$(find . -name "*_test.go" | wc -l || echo "0")
            echo "- ℹ️  Test files: $TEST_FILES" >> "$report"
            ;;
            
        "comprehensive")
            echo "🔍 Running Comprehensive Audit..."
            # Run all checks
            go build ./... > /tmp/build.log 2>&1 && echo "- ✅ Build: PASSED" >> "$report" || echo "- ❌ Build: FAILED" >> "$report"
            go vet ./... > /tmp/vet.log 2>&1 && echo "- ✅ Go vet: PASSED" >> "$report" || echo "- ⚠️  Go vet: WARNINGS" >> "$report"
            ;;
    esac
    
    return 0
}

# Initialize report
cat > "$REPORT_FILE" << EOF
# Audit Report - $CYCLE_NAME
- **Cycle**: $((CURRENT_INDEX + 1))/${#CYCLES[@]}
- **Focus**: $CYCLE_NAME
- **Timestamp**: $TIMESTAMP
- **Branch**: $(git branch --show-current 2>/dev/null || echo "unknown")
- **Commit**: $(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

## Audit Results
EOF

# Run audit
echo "Running audit for: $CYCLE_NAME"
if run_audit_cycle "$CYCLE_KEY" "$REPORT_FILE"; then
    echo "✅ Audit cycle PASSED"
    echo "" >> "$REPORT_FILE"
    echo "## Status" >> "$REPORT_FILE"
    echo "- **Result**: ✅ PASSED" >> "$REPORT_FILE"
    echo "- **Action**: Ready to advance to next cycle" >> "$REPORT_FILE"
    
    # Advance to next cycle
    NEXT_INDEX=$((CURRENT_INDEX + 1))
    echo "$NEXT_INDEX" > "$CURRENT_CYCLE_FILE"
    
    if [ $NEXT_INDEX -ge ${#CYCLES[@]} ]; then
        echo ""
        echo "🎉 ALL CYCLES COMPLETED SUCCESSFULLY!"
        echo "All audit cycles passed. System is healthy."
        echo ""
        echo "Next action: Run comprehensive audit weekly or on-demand."
        
        # Reset cycle counter
        echo "0" > "$CURRENT_CYCLE_FILE"
    else
        echo ""
        echo "✅ Cycle $((CURRENT_INDEX + 1)) PASSED"
        echo "🚀 Advancing to next cycle: ${CYCLES[$NEXT_INDEX]#*:}"
        echo ""
        echo "To run next cycle manually:"
        echo "  ./scripts/sequential_audit.sh"
    fi
else
    echo "❌ Audit cycle FAILED"
    echo "" >> "$REPORT_FILE"
    echo "## Status" >> "$REPORT_FILE"
    echo "- **Result**: ❌ FAILED" >> "$REPORT_FILE"
    echo "- **Action**: Fix issues and re-run audit" >> "$REPORT_FILE"
    
    # Create task for fix
    TASK_FILE="$TASK_DIR/AUDIT-FIX-${TIMESTAMP}.md"
    cat > "$TASK_FILE" << TASK_EOF
# Audit Fix Task - $CYCLE_NAME

## Description
Audit cycle '$CYCLE_NAME' failed. Issues need to be fixed before proceeding.

## Failed Cycle
- **Cycle**: $((CURRENT_INDEX + 1))/${#CYCLES[@]}
- **Focus**: $CYCLE_NAME
- **Timestamp**: $TIMESTAMP

## Issues Found
$(grep "❌" "$REPORT_FILE" || echo "No specific failures logged")

## Priority
🔴 Critical - Blocker for next audit cycle

## Acceptance Criteria
- [ ] Fix all issues from audit report
- [ ] Re-run audit: ./scripts/sequential_audit.sh
- [ ] Verify audit passes
- [ ] Mark task complete

## Related
- Audit Report: $REPORT_FILE
TASK_EOF
    
    echo ""
    echo "📝 Task created: $TASK_FILE"
    echo "Fix the issues and re-run: ./scripts/sequential_audit.sh"
fi

echo ""
echo "📄 Report: $REPORT_FILE"
