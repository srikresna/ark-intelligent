# Sequential Audit System - User Guide

## 🎯 How It Works

Sistem audit yang **sequential dan smart**:
1. Run audit cycle by cycle (bukan paralel)
2. Jika PASS → auto advance ke cycle berikutnya
3. Jika FAIL → create task, stop, tunggu fix
4. Setelah fix → run verify → auto advance jika PASS

## 📋 Audit Cycles (8 Total)

| # | Cycle | Focus | Auto-Verify |
|---|-------|-------|-------------|
| 1 | build-security | Build & Security | ✅ Yes |
| 2 | tests-handlers | Tests & Handlers | ✅ Yes |
| 3 | ui-ux-flow | UI/UX Flow | ⚠️ Manual |
| 4 | feature-logic | Feature Logic | ⚠️ Manual |
| 5 | errors-panic | Error Handling | ✅ Yes |
| 6 | api-performance | API & Performance | ⚠️ Manual |
| 7 | code-quality | Code Quality | ✅ Yes |
| 8 | comprehensive | Full Audit | ✅ Yes |

## 🚀 Quick Start

### **Run First Audit:**
```bash
cd /path/to/ark-intelligent
./scripts/sequential_audit.sh
```

### **After Fixing Issues:**
```bash
# Verify fixes and advance
./scripts/verify_and_advance.sh
```

### **Check Current Status:**
```bash
cat .agents/audit/.current_cycle
ls -lt .agents/audit/*.md | head -5
```

## 🔄 Workflow

### **Scenario 1: All Pass (Happy Path)**
```bash
$ ./scripts/sequential_audit.sh
🔍 Running Build & Security Audit...
✅ Audit cycle PASSED
🚀 Advancing to next cycle: Tests & Handlers

$ ./scripts/sequential_audit.sh
🔍 Running Tests & Handlers Audit...
✅ Audit cycle PASSED
🚀 Advancing to next cycle: UI/UX Flow

... (continue until all 8 cycles complete)

🎉 ALL CYCLES COMPLETED SUCCESSFULLY!
```

### **Scenario 2: Fail - Fix - Verify**
```bash
$ ./scripts/sequential_audit.sh
🔍 Running Build & Security Audit...
❌ Build: FAILED
📝 Task created: .agents/tasks/pending/AUDIT-FIX-20260407-123456.md

# Fix the issues...

$ ./scripts/verify_and_advance.sh
🔍 Verifying fixes...
✅ All checks PASSED!
🚀 Advancing to next cycle: Tests & Handlers
```

### **Scenario 3: Manual Verification Required**
For UI/UX, Feature Logic, API Performance - manual test required:

```bash
$ ./scripts/sequential_audit.sh
🔍 Running UI/UX Flow Audit...
⚠️  Manual test required in Telegram

# Manual test in Telegram:
# - Test all buttons
# - Check loading states
# - Verify error messages

$ ./scripts/verify_and_advance.sh
🔍 Verifying...
ℹ️  Manual verification required
# Edit verify report and mark as PASS
```

## 📊 Status Tracking

### **Current Cycle:**
```bash
cat .agents/audit/.current_cycle
# Output: 3 (means currently on cycle 3)
```

### **Audit Reports:**
```bash
ls -lt .agents/audit/*.md
# Shows all audit reports with timestamps
```

### **Pending Tasks:**
```bash
ls .agents/tasks/pending/
# Shows all failed audit tasks that need fixing
```

## 🛠️ Commands Reference

| Command | Description |
|---------|-------------|
| `./scripts/sequential_audit.sh` | Run next audit cycle |
| `./scripts/verify_and_advance.sh` | Verify fixes and advance |
| `./scripts/feature_audit.sh` | Run feature-specific audit |
| `./scripts/run_audit.sh` | Run full quick audit |

## ⚠️ Important Notes

1. **Manual Tests Required**
   - UI/UX Flow (Cycle 3)
   - Feature Logic (Cycle 4)
   - API Performance (Cycle 6)
   
   Untuk cycle ini, kamu perlu **manual test di Telegram bot**:
   ```bash
   # Test commands:
   /start, /outlook, /cot, /calendar, /bias, /radar
   ```

2. **Auto-Verify Only for Code Checks**
   - Build, tests, security, code quality = auto-verify
   - UI/UX, features, API = manual verify

3. **Reset After Complete**
   Setelah semua 8 cycle complete, sistem auto-reset ke cycle 1.

## 📈 Progress Tracking

### **View Latest Audit:**
```bash
cat .agents/audit/$(ls -t .agents/audit/*.md | head -1)
```

### **View All Failures:**
```bash
grep -l "FAILED\|❌" .agents/audit/*.md
```

### **View All Tasks:**
```bash
ls .agents/tasks/pending/AUDIT-*
```

## 🎯 Best Practices

1. **Run sequentially** - Jangan skip cycle
2. **Fix immediately** - Jangan tunda fix task
3. **Verify after fix** - Selalu run verify script
4. **Manual test thoroughly** - Test di Telegram untuk manual cycles
5. **Document issues** - Catat semua masalah yang ditemukan

## 🔧 Troubleshooting

### **Stuck on same cycle?**
```bash
# Check what's failing
cat .agents/audit/*-audit-*.md | grep "FAILED\|❌"

# Fix the issues
# Re-run verify
./scripts/verify_and_advance.sh
```

### **Want to reset?**
```bash
echo "0" > .agents/audit/.current_cycle
```

### **Want to skip cycle?**
```bash
# Advance manually (NOT RECOMMENDED)
CURRENT=$(cat .agents/audit/.current_cycle)
echo $((CURRENT + 1)) > .agents/audit/.current_cycle
```

## ✅ Success Criteria

Audit dianggap complete ketika:
- ✅ All 8 cycles passed
- ✅ No pending AUDIT-FIX tasks
- ✅ Manual tests verified for cycles 3, 4, 6
- ✅ Latest report shows "ALL CYCLES COMPLETED"

---

**Ready to start?**
```bash
./scripts/sequential_audit.sh
```
