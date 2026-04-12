#!/bin/bash
# Toggle auto-commit on/off

AUDIT_DIR=".agents/audit"
AUTO_COMMIT_FILE="$AUDIT_DIR/.auto_commit_enabled"

mkdir -p "$AUDIT_DIR"

if [ -f "$AUTO_COMMIT_FILE" ]; then
    rm "$AUTO_COMMIT_FILE"
    echo "❌ Auto-commit DISABLED"
    echo "  Run: ./scripts/toggle_auto_commit.sh to enable"
else
    touch "$AUTO_COMMIT_FILE"
    echo "✅ Auto-commit ENABLED"
    echo "  Daemon will automatically commit and push after successful audit"
fi
