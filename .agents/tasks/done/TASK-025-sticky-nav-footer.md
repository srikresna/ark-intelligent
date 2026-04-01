# TASK-025: Sticky Navigation Footer di Semua Keyboard Response

**Priority:** HIGH  
**Siklus:** 1 (UX/UI)  
**Effort:** Medium (1-2 hari)  
**File Utama:** `internal/adapter/telegram/keyboard.go`, `internal/adapter/telegram/handler.go`

---

## Problem

Saat ini `MainMenu` keyboard hanya muncul di `/start` dan `/help`.  
Ketika user di dalam view detail (COT detail, Alpha factors, Quant output, dsb), tidak ada tombol untuk kembali ke home atau navigasi ke fitur lain.  
User harus ketik command manual.

## Solution

### 1. Tambah method `NavFooter()` di KeyboardBuilder
```go
// NavFooter returns a single-row navigation footer to append to any keyboard.
func (kb *KeyboardBuilder) NavFooter() []ports.InlineButton {
    return []ports.InlineButton{
        {Text: "🏠", CallbackData: "cmd:help"},
        {Text: "📊 COT", CallbackData: "nav:cot"},
        {Text: "📅 Cal", CallbackData: "cmd:calendar"},
        {Text: "🦅 Outlook", CallbackData: "out:unified"},
        {Text: "⚙️", CallbackData: "cmd:settings"},
    }
}
```

### 2. Tambah helper `AppendNavFooter(kb ports.InlineKeyboard) ports.InlineKeyboard`
Append NavFooter row ke keyboard yang sudah ada.

### 3. Apply ke semua response yang punya keyboard
- COT Overview & Detail keyboard
- Alpha, CTA, Quant, VP keyboards
- Macro, Calendar keyboards
- Settings keyboard

### 4. Handle callback `cmd:help`
- Register `cmd:help` di cbQuickCommand → panggil sendHelp()

## Acceptance Criteria
- [ ] `NavFooter()` method ada di keyboard.go
- [ ] Footer muncul di semua major response (COT, Alpha, Outlook, Macro, Calendar)
- [ ] `cmd:help` callback berfungsi → tampilkan main menu
- [ ] Tidak ada double footer (append hanya sekali)
- [ ] Test manual: dari COT detail, tekan 🏠 → kembali ke help/main menu

## Notes
- Gunakan emoji kecil + singkat agar tidak makan space
- Footer TIDAK perlu di error messages
- Prioritas: terapkan dulu di COT, Alpha, Outlook — lalu yang lain bertahap
