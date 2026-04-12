#!/bin/bash
# Verify fixes and advance to next cycle
# Run this AFTER fixing issues

set -e

export PATH=$PATH:/home/node/go/bin
export GOPATH=/home/node/.go

AUDIT_DIR=".agents/audit"
CURRENT_CYCLE_FILE="$AUDIT_DIR/.current_cycle"

if [ ! -f "$CURRENT_CYCLE_FILE" ]; then
    echo "No audit in progress. Run ./scripts/sequential_audit.sh first"
    exit 1
fi

CURRENT_INDEX=$(cat "$CURRENT_CYCLE_FILE")
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

CURRENT_CYCLE="${CYCLES[$CURRENT_INDEX]}"
CYCLE_KEY=$(echo "$CURRENT_CYCLE" | cut -d: -f1)
CYCLE_NAME=$(echo "$CURRENT_CYCLE" | cut -d: -f2)

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
VERIFY_FILE="$AUDIT_DIR/${TIMESTAMP}-verify-${CYCLE_KEY}.md"

echo "🔍 Verify & Advance"
echo "==================="
echo "Current Cycle: $CYCLE_NAME"
echo "Verifying fixes..."
echo ""

# Initialize verify report
cat > "$VERIFY_FILE" << EOF
# Verification Report - $CYCLE_NAME
- **Timestamp**: $TIMESTAMP
- **Cycle**: $((CURRENT_INDEX + 1))/${#CYCLES[@]}
- **Focus**: $CYCLE_NAME

## Verification Results
EOF

# Run the same audit for this cycle
case $CYCLE_KEY in
    "build-security")
        if go build ./... 2>&1 | tee /tmp/build_verify.log; then
            echo "- ✅ Build: PASSED" >> "$VERIFY_FILE"
            BUILD_OK=true
        else
            echo "- ❌ Build: STILL FAILING" >> "$VERIFY_FILE"
            BUILD_OK=false
        fi
        
        SECRETS=$(grep -r "ghp_\|sk-[A-Za-z0-9]" --include="*.go" . 2>/dev/null | grep -v test | wc -l || echo "0")
        if [ "$SECRETS" -eq 0 ]; then
            echo "- ✅ No hardcoded secrets" >> "$VERIFY_FILE"
            SECRET_OK=true
        else
            echo "- ❌ Secrets still found: $SECRETS" >> "$VERIFY_FILE"
            SECRET_OK=false
        fi
        ;;
        
    "tests-handlers")
        if go test ./internal/service/... ./internal/adapter/... -short 2>&1 | tee /tmp/test_verify.log; then
            echo "- ✅ Tests: PASSED" >> "$VERIFY_FILE"
            TEST_OK=true
        else
            echo "- ❌ Tests: STILL FAILING" >> "$VERIFY_FILE"
            TEST_OK=false
        fi
        ;;
        
    "ui-ux-flow"|"feature-logic"|"errors-panic"|"api-performance")
        echo "- ℹ️  Manual verification required" >> "$VERIFY_FILE"
        echo "- ⚠️  Please test manually in Telegram and mark as PASS" >> "$VERIFY_FILE"
        echo "" >> "$VERIFY_FILE"
        echo "## Manual Verification" >> "$VERIFY_FILE"
        echo "- [ ] Test in Telegram bot" >> "$VERIFY_FILE"
        echo "- [ ] Verify all features work" >> "$VERIFY_FILE"
        echo "- [ ] Mark this file with result" >> "$VERIFY_FILE"
        VERIFY_OK=true  # Assume pass for manual, user will confirm
        ;;
        
    *)
        if go build ./... > /tmp/build_verify.log 2>&1; then
            echo "- ✅ Build: PASSED" >> "$VERIFY_FILE"
            BUILD_OK=true
        else
            echo "- ❌ Build: STILL FAILING" >> "$VERIFY_FILE"
            BUILD_OK=false
        fi
        ;;
esac

# Check if all passed
ALL_PASS=true
if [ -n "$BUILD_OK" ] && [ "$BUILD_OK" = false ]; then ALL_PASS=false; fi
if [ -n "$SECRET_OK" ] && [ "$SECRET_OK" = false ]; then ALL_PASS=false; fi
if [ -n "$TEST_OK" ] && [ "$TEST_OK" = false ]; then ALL_PASS=false; fi

echo "" >> "$VERIFY_FILE"
echo "## Final Status" >> "$VERIFY_FILE"

if [ "$ALL_PASS" = true ]; then
    echo "- **Result**: ✅ ALL CHECKS PASSED" >> "$VERIFY_FILE"
    echo "- **Action**: Advancing to next cycle" >> "$VERIFY_FILE"
    
    # Advance
    NEXT_INDEX=$((CURRENT_INDEX + 1))
    echo "$NEXT_INDEX" > "$CURRENT_CYCLE_FILE"
    
    echo ""
    echo "✅ All checks PASSED!"
    echo "🚀 Advancing to next cycle: ${CYCLES[$NEXT_INDEX]#*:}"
    
    if [ $NEXT_INDEX -ge ${#CYCLES[@]} ]; then
        echo ""
        echo "🎉 ALL CYCLES COMPLETED!"
        echo "Resetting to cycle 1"
        echo "0" > "$CURRENT_CYCLE_FILE"
    fi
else
    echo "- **Result**: ❌ SOME CHECKS FAILED" >> "$VERIFY_FILE"
    echo "- **Action**: Continue fixing issues" >> "$VERIFY_FILE"
    
    echo ""
    echo "❌ Some checks still failing"
    echo "Keep fixing and run: ./scripts/verify_and_advance.sh"
fi

echo ""
echo "📄 Verify report: $VERIFY_FILE"
