# Research: UX/UI — Onboarding & Navigation (Siklus 1)

**Date:** 2026-04-01 01:00 WIB
**Agent:** Research
**Fokus:** UX/UI Improvement (Siklus 1)
**Referensi:** .agents/UX_AUDIT.md

---

## Temuan Utama

### 1. Onboarding Gap Kritis

Analisis handler.go menunjukkan `/start` hanya menampilkan welcome message statis + daftar command.
Tidak ada guided tour, tidak ada interactive keyboard untuk "mulai dari mana", tidak ada step-by-step.

User baru melihat 28+ command sekaligus — sangat overwhelming.

**Impact:** User churn tinggi di first session. User tidak tahu command mana yang relevan untuk kebutuhan mereka.

**Fix yang diusulkan:**
- `/start` → tampilkan role selector (Trader pemula / Intermediate / Pro)
- Setiap role → tampilkan "starter kit" keyboard (3-4 command yang paling relevan)
- Setelah pilih role → brief tutorial interaktif 3 langkah

---

### 2. Navigation Inconsistency

Audit string di formatter.go dan handler.go menunjukkan mixed language di button labels:
- `<< Kembali ke Ringkasan`
- `<< Back to Overview`
- `<< Kembali ke Dashboard`
- `↩ Back`

Dan tidak ada "home" button universal. User yang masuk deep di drill-down COT atau Macro harus `/start` ulang.

**Fix yang diusulkan:**
- Standarisasi semua back button: `🏠 Menu Utama` dan `◀ Kembali`
- Tambah home button di semua multi-step keyboard
- Buat `keyboard.go` constants untuk semua button label (DRY)

---

### 3. Response Time Feedback

Command `/outlook`, `/quant`, `/cta` bisa butuh 5-15 detik (AI + data fetching).
Tidak ada feedback bahwa bot sedang memproses. User sering kirim command ulang karena mengira hang.

**Fix yang diusulkan:**
- Kirim "⏳ Menganalisis..." segera setelah command diterima
- Edit message dengan progress: "⏳ Fetching data... (1/3)"
- Atau gunakan Telegram `sendChatAction` typing indicator

---

### 4. Output Density di Mobile

Beberapa output `/cot` dan `/macro` sangat panjang (>4000 karakter).
Telegram memotong message atau user harus scroll panjang di mobile.

Cek di formatter.go: `formatCOTResult()` menghasilkan output ~3000-5000 chars dengan tabel ASCII.

**Fix yang diusulkan:**
- Default output = "compact" view (summary + key numbers)
- Button "📖 Detail Lengkap" untuk expand
- Simpan preference user (compact/full) di BadgerDB

---

### 5. Error Messages Technical

Error messages langsung expose internal error string ke user.
Contoh handler.go: `return bot.sendMessage(chatID, fmt.Sprintf("Error: %v", err))`

User trader tidak perlu tahu "context deadline exceeded" atau "badger: key not found".

**Fix yang diusulkan:**
- Buat `internal/adapter/telegram/errors.go` dengan user-friendly error messages
- Map error types ke pesan yang actionable
- Log technical error internally, tampilkan pesan ramah ke user

---

## Task yang Direkomendasikan

1. **TASK-001:** Interactive onboarding dengan role selector (HIGH)
2. **TASK-002:** Standardisasi button labels + home button universal (MEDIUM)
3. **TASK-003:** Typing indicator / progress feedback untuk long commands (HIGH)
4. **TASK-004:** Compact mode default output + expand button (MEDIUM)
5. **TASK-005:** User-friendly error messages layer (HIGH)
