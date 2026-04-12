#!/bin/bash
# Autonomous Audit Script
# Runs comprehensive audit every 15 minutes

set -e

# Setup Go environment
export PATH=$PATH:/home/node/go/bin
export GOPATH=/home/node/.go

AUDIT_DIR=".agents/audit"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT_FILE="$AUDIT_DIR/${TIMESTAMP}-audit.md"
FIXES_FILE="$AUDIT_DIR/${TIMESTAMP}-fixes.md"

# Create audit directory if not exists
mkdir -p "$AUDIT_DIR"

echo "🔍 Starting Autonomous Audit at $(date)"
echo "========================================"

# Initialize report
cat > "$REPORT_FILE" << EOF
# Audit Report - $(date)

## Summary
- **Timestamp**: $TIMESTAMP
- **Branch**: $(git branch --show-current)
- **Commit**: $(git rev-parse --short HEAD)

## Audit Categories
EOF

# 1. BUILD & COMPILATION CHECK
echo "### 🔨 Build Check" >> "$REPORT_FILE"
if go build ./... 2>&1 | tee /tmp/build.log; then
    echo "- ✅ Build: PASSED" >> "$REPORT_FILE"
else
    echo "- ❌ Build: FAILED" >> "$REPORT_FILE"
    echo "Build errors:" >> "$REPORT_FILE"
    cat /tmp/build.log >> "$REPORT_FILE"
    echo "🔴 CRITICAL: Build failed - audit aborted"
    exit 1
fi

# 2. STATIC ANALYSIS
echo "" >> "$REPORT_FILE"
echo "### 📊 Static Analysis" >> "$REPORT_FILE"
if go vet ./... 2>&1 | tee /tmp/vet.log; then
    echo "- ✅ Go vet: PASSED" >> "$REPORT_FILE"
else
    echo "- ⚠️  Go vet: WARNINGS" >> "$REPORT_FILE"
    cat /tmp/vet.log >> "$REPORT_FILE"
fi

# 3. TEST SUITE
echo "" >> "$REPORT_FILE"
echo "### 🧪 Test Suite" >> "$REPORT_FILE"
TEST_OUTPUT=$(go test ./internal/service/... ./internal/adapter/... -short 2>&1 || echo "TESTS_FAILED")
if echo "$TEST_OUTPUT" | grep -q "PASS"; then
    echo "- ✅ Tests: PASSED" >> "$REPORT_FILE"
    echo "  $(echo "$TEST_OUTPUT" | grep -E "^(ok|FAIL)" | head -5)" >> "$REPORT_FILE"
else
    echo "- ❌ Tests: FAILED" >> "$REPORT_FILE"
    echo "$TEST_OUTPUT" >> "$REPORT_FILE"
fi

# 4. HANDLER FUNCTIONALITY CHECK
echo "" >> "$REPORT_FILE"
echo "### ⚡ Handler Functionality" >> "$REPORT_FILE"

# Check if all handlers are registered
HANDLER_COUNT=$(grep -r "AddCommand\|AddCallback" internal/adapter/telegram/*.go 2>/dev/null | wc -l)
echo "- Registered handlers: $HANDLER_COUNT" >> "$REPORT_FILE"

# Check for nil pointer guards
NIL_CHECKS=$(grep -r "if.*== nil" internal/adapter/telegram/handler*.go 2>/dev/null | wc -l)
echo "- Nil pointer guards: $NIL_CHECKS" >> "$REPORT_FILE"

# 5. ERROR HANDLING
echo "" >> "$REPORT_FILE"
echo "### 🛡️ Error Handling" >> "$REPORT_FILE"
ERROR_HANDLERS=$(grep -r "Err(" internal/adapter/telegram/*.go 2>/dev/null | wc -l)
echo "- Error handlers: $ERROR_HANDLERS" >> "$REPORT_FILE"

# 6. PANIC RECOVERY
echo "" >> "$REPORT_FILE"
echo "### 🔄 Panic Recovery" >> "$REPORT_FILE"
RECOVERY_BLOCKS=$(grep -r "defer.*recover" internal/ 2>/dev/null | wc -l)
echo "- Panic recovery blocks: $RECOVERY_BLOCKS" >> "$REPORT_FILE"

# 7. API INTEGRATION STATUS
echo "" >> "$REPORT_FILE"
echo "### 🔌 API Integrations" >> "$REPORT_FILE"

# Check environment variables
if [ -f ".env" ]; then
    BOT_TOKEN_SET=$(grep -c "BOT_TOKEN=" .env || echo "0")
    CHAT_ID_SET=$(grep -c "CHAT_ID=" .env || echo "0")
    
    if [ "$BOT_TOKEN_SET" -gt 0 ] && [ "$CHAT_ID_SET" -gt 0 ]; then
        echo "- ✅ Telegram Bot: CONFIGURED" >> "$REPORT_FILE"
    else
        echo "- ⚠️  Telegram Bot: NOT CONFIGURED" >> "$REPORT_FILE"
    fi
else
    echo "- ⚠️  .env file: NOT FOUND" >> "$REPORT_FILE"
fi

# 8. PERFORMANCE METRICS
echo "" >> "$REPORT_FILE"
echo "### ⚡ Performance" >> "$REPORT_FILE"

# Count goroutines in scheduler
GOROUTINE_COUNT=$(grep -r "go func" internal/scheduler/*.go 2>/dev/null | wc -l)
echo "- Active goroutines: $GOROUTINE_COUNT" >> "$REPORT_FILE"

# 9. CODE QUALITY
echo "" >> "$REPORT_FILE"
echo "### 📝 Code Quality" >> "$REPORT_FILE"
TOTAL_LINES=$(find . -name "*.go" -not -path "./.git/*" -not -path "./vendor/*" | xargs wc -l | tail -1 | awk '{print $1}')
echo "- Total Go lines: $TOTAL_LINES" >> "$REPORT_FILE"

# Test files count
TEST_FILES=$(find . -name "*_test.go" | wc -l)
echo "- Test files: $TEST_FILES" >> "$REPORT_FILE"

# 10. SECURITY CHECKS
echo "" >> "$REPORT_FILE"
echo "### 🔒 Security" >> "$REPORT_FILE"

# Check for hardcoded secrets
HARDCODED_SECRETS=$(grep -r "ghp_\|sk-\|api_key" --include="*.go" . 2>/dev/null | grep -v "test" | wc -l || echo "0")
if [ "$HARDCODED_SECRETS" -eq 0 ]; then
    echo "- ✅ No hardcoded secrets: PASSED" >> "$REPORT_FILE"
else
    echo "- ❌ Hardcoded secrets detected: $HARDCODED_SECRETS" >> "$REPORT_FILE"
fi

# Final summary
echo "" >> "$REPORT_FILE"
echo "## Final Status" >> "$REPORT_FILE"
echo "- **Audit completed**: $(date)" >> "$REPORT_FILE"
echo "- **Report file**: $REPORT_FILE" >> "$REPORT_FILE"

echo ""
echo "✅ Audit completed successfully!"
echo "📄 Report: $REPORT_FILE"

# Create task if issues found
if grep -q "❌" "$REPORT_FILE"; then
    echo "⚠️  Issues detected - creating task..."
    
    TASK_ID=$(date +%Y%m%d%H%M%S)
    cat > ".agents/tasks/pending/AUDIT-$TASK_ID.md" << TASK_EOF
# Audit Fix Task - $TASK_ID

## Description
Automated audit detected issues that need attention.

## Issues Found
$(grep "❌\|⚠️" "$REPORT_FILE")

## Priority
🟠 High - Address within 1 hour

## Acceptance Criteria
- [ ] Fix all critical errors
- [ ] Re-run audit: ./scripts/run_audit.sh
- [ ] Verify build passes
- [ ] Update this task with fixes

## Related
- Audit Report: $REPORT_FILE
TASK_EOF
    
    echo "📝 Task created: .agents/tasks/pending/AUDIT-$TASK_ID.md"
fi

echo "Audit cycle complete. Next run in 15 minutes..."
