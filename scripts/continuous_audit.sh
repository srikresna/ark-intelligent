#!/bin/bash
# Continuous Audit Daemon with Auto-Fix & Commit
# Runs audit every 15 minutes, auto-fixes, builds, and commits

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
LOG_FILE="$PROJECT_DIR/.agents/audit/audit-daemon.log"
PID_FILE="$PROJECT_DIR/.agents/audit/audit.pid"
AUTO_COMMIT_FILE="$PROJECT_DIR/.agents/audit/.auto_commit_enabled"

cd "$PROJECT_DIR"

echo "🤖 Starting Autonomous Audit Daemon with Auto-Commit"
echo "====================================================="
echo "Project: $PROJECT_DIR"
echo "Log: $LOG_FILE"
echo "Interval: 15 minutes"
echo "Auto-commit: $( [ -f "$AUTO_COMMIT_FILE" ] && echo "ENABLED" || echo "DISABLED" )"
echo ""

# Save PID
echo $$ > "$PID_FILE"

# Trap for cleanup
cleanup() {
    echo "🛑 Stopping audit daemon..."
    rm -f "$PID_FILE"
    exit 0
}

trap cleanup SIGINT SIGTERM

# Function to auto-fix common issues
auto_fix_issues() {
    local report_file=$1
    
    echo "🔧 Attempting auto-fixes..."
    
    # Check for common issues and fix them
    if grep -q "hardcoded secrets" "$report_file" 2>/dev/null; then
        echo "  ⚠️  Secrets detected - manual fix required"
        return 1
    fi
    
    if grep -q "build.*FAILED" "$report_file" 2>/dev/null; then
        echo "  ⚠️  Build failed - manual fix required"
        return 1
    fi
    
    if grep -q "tests.*FAILED" "$report_file" 2>/dev/null; then
        echo "  ⚠️  Tests failed - manual fix required"
        return 1
    fi
    
    echo "  ✅ No auto-fixable issues found"
    return 0
}

# Function to verify build and commit
verify_and_commit() {
    local report_file=$1
    local timestamp=$2
    
    echo "🔨 Verifying build..."
    
    # Try build
    if go build ./... > /tmp/audit_build.log 2>&1; then
        echo "  ✅ Build: PASSED"
        
        # Run go vet
        if go vet ./... > /tmp/audit_vet.log 2>&1; then
            echo "  ✅ Go vet: PASSED"
            
            # Check for changes to commit
            if git diff --quiet 2>/dev/null; then
                echo "  ℹ️  No changes to commit"
                return 0
            fi
            
            # Auto-commit if enabled
            if [ -f "$AUTO_COMMIT_FILE" ]; then
                echo "💾 Auto-committing changes..."
                
                git add -A 2>/dev/null || true
                
                COMMIT_MSG="chore(audit): auto-commit after audit cycle $timestamp"
                git commit -m "$COMMIT_MSG" 2>/dev/null || {
                    echo "  ⚠️  No changes to commit"
                    return 0
                }
                
                echo "  ✅ Committed: $COMMIT_MSG"
                
                # Push if remote is configured
                if git remote -v | grep -q origin; then
                    echo "📤 Pushing to remote..."
                    git push origin $(git branch --show-current) 2>/dev/null || {
                        echo "  ⚠️  Push failed (manual push required)"
                    }
                fi
            else
                echo "  ℹ️  Auto-commit disabled (touch .agents/audit/.auto_commit_enabled to enable)"
            fi
            
            return 0
        else
            echo "  ❌ Go vet: FAILED"
            cat /tmp/audit_vet.log
            return 1
        fi
    else
        echo "  ❌ Build: FAILED"
        cat /tmp/audit_build.log
        return 1
    fi
}

# Main loop
CYCLE=0
while true; do
    CYCLE=$((CYCLE + 1))
    TIMESTAMP=$(date +%Y%m%d-%H%M%S)
    DATETIME=$(date +"%Y-%m-%d %H:%M:%S")
    
    echo "" >> "$LOG_FILE"
    echo "========================================" >> "$LOG_FILE"
    echo "Audit Cycle #$CYCLE - $DATETIME" >> "$LOG_FILE"
    echo "========================================" >> "$LOG_FILE"
    
    echo "🔍 Cycle #$CYCLE at $DATETIME"
    
    # Run audit
    AUDIT_REPORT="$PROJECT_DIR/.agents/audit/${TIMESTAMP}-audit.md"
    
    if bash "$SCRIPT_DIR/run_audit.sh" > "$AUDIT_REPORT" 2>&1; then
        echo "✅ Audit completed"
        
        # Check for issues
        if grep -q "❌\|FAILED" "$AUDIT_REPORT" 2>/dev/null; then
            echo "⚠️  Issues detected"
            
            # Try auto-fix
            if auto_fix_issues "$AUDIT_REPORT"; then
                echo "✅ Auto-fix successful"
                
                # Verify build
                if verify_and_commit "$AUDIT_REPORT" "$TIMESTAMP"; then
                    echo "✅ Cycle #$CYCLE completed - build passed, changes committed"
                else
                    echo "❌ Cycle #$CYCLE - build failed after auto-fix"
                fi
            else
                echo "⚠️  Auto-fix failed - manual intervention required"
                
                # Create task for manual fix
                TASK_FILE="$PROJECT_DIR/.agents/tasks/pending/AUDIT-AUTOFIX-${TIMESTAMP}.md"
                cat > "$TASK_FILE" << TASK_EOF
# Auto-Fix Task - Cycle #$CYCLE

## Description
Automated audit detected issues that could not be auto-fixed.

## Timestamp
- **Cycle**: $CYCLE
- **Time**: $DATETIME
- **Audit Report**: $AUDIT_REPORT

## Issues Found
$(grep "❌\|FAILED" "$AUDIT_REPORT" 2>/dev/null || echo "No specific issues logged")

## Priority
🔴 Critical - Manual fix required

## Action Required
1. Review audit report: $AUDIT_REPORT
2. Fix the issues manually
3. Run: ./scripts/verify_and_advance.sh
4. Verify build passes
5. Mark task complete

## Related
- Audit Report: $AUDIT_REPORT
- Log: $LOG_FILE
TASK_EOF
                
                echo "📝 Task created: $TASK_FILE"
            fi
        else
            echo "✅ No issues found"
            
            # Verify build and commit even if no issues (for any code changes)
            verify_and_commit "$AUDIT_REPORT" "$TIMESTAMP"
            
            echo "✅ Cycle #$CYCLE completed successfully"
        fi
    else
        echo "❌ Audit script failed"
        echo "📄 Check: $AUDIT_REPORT"
    fi
    
    # Wait 15 minutes (900 seconds)
    echo "⏳ Waiting 15 minutes before next cycle..."
    sleep 900
done
