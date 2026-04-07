# Research Report: UX/UI Siklus 1 Putaran 7
# Navigation Gaps, Keyboard Inconsistencies, Broken Callbacks
**Date:** 2026-04-02 18:00 WIB
**Siklus:** 1/5 (UX/UI) — Putaran 7
**Author:** Research Agent

## Ringkasan

Audit mendalam terhadap keyboard navigation dan callback registration di handler-handler baru. Ditemukan 5 genuine UX issues: 1 broken callback (view: tidak terdaftar), 2 handler tanpa keyboard sama sekali (wyckoff, levels), 1 missing home button di 3 handler sekaligus (ict/smc/gex), dan 1 gap di Settings menu (OutputMode tidak bisa di-set global).

---

## Temuan 1: `view:` Callback TIDAK Terdaftar — Tombol Compact/Full Mati

`handler.go:1788` mendefinisikan `cbViewToggle` untuk tombol "📖 Detail Lengkap" dan "📊 Compact" di COT dan Macro views. Tombol-tombol ini generate callback data `view:full:cot`, `view:compact:cot`, `view:full:macro`, dll.

Namun di `handler.go:240-251`, daftar `RegisterCallback` **tidak menyertakan `"view:"`**. Semua prefix yang ada: `cot:`, `alert:`, `set:`, `cal:filter:`, `out:`, `cal:nav:`, `cmd:`, `onboard:`, `macro:`, `imp:`, `nav:`, `help:`. Tidak ada `view:`.

**Efek:** Klik tombol compact/expand di /cot dan /macro hasilnya: bot tidak merespons sama sekali (silent drop). User mengira bot hang atau fitur rusak.

**Fix:** Tambah `bot.RegisterCallback("view:", h.cbViewToggle)` di `Register()`.

---

## Temuan 2: `/wyckoff` Tidak Ada Keyboard

`handler_wyckoff.go:112` mengirim hasil analisis dengan `h.bot.SendHTML(ctx, chatID, output)` — plain HTML tanpa keyboard sama sekali. Tidak ada Refresh, tidak ada Home, tidak ada timeframe switcher setelah result ditampilkan.

Pembanding: `/ict` dan `/smc` punya `ictNavKeyboard(symbol, tf)` dan `smcNavKeyboard(symbol, tf)` dengan timeframe selector + Refresh + Kembali.

**Fix:** Buat `wyckoffNavKeyboard(symbol, currentTF string)` dan gunakan `SendWithKeyboard` di `cmdWyckoff`, serta `EditWithKeyboard` di hasil loading.

---

## Temuan 3: `/levels` Handler — Semua Output Tanpa Keyboard

`handler_levels.go:79` dan `handler_levels.go:96` keduanya menggunakan `h.bot.SendHTML(ctx, chatID, msg)` tanpa keyboard. Output levels adalah data berguna yang mungkin ingin di-refresh user atau navigate ke home.

Pembanding: `/price` handler punya keyboard di beberapa path tapi juga ada path yang `SendHTML` biasa (handler_price.go:86,104).

**Fix:** Tambah HomeRow keyboard ke semua final output di handler_levels.go dan path non-keyboard di handler_price.go.

---

## Temuan 4: ICT, SMC, GEX Keyboards Tidak Punya Home Button

- `ictNavKeyboard` (handler_ict.go:295): hanya TF row + [Refresh, ◀ Kembali ke symbol selector]
- `smcNavKeyboard` (handler_smc.go:472): identik dengan ict
- `gexKeyboard` (handler_gex.go:127): hanya symbol switcher + Refresh

Tidak ada `{Text: btnHome, CallbackData: "nav:home"}` di ketiganya. User yang masuk dari /ict EUR H4 terjebak dalam view itu — "◀ Kembali" hanya kembali ke symbol selector, bukan ke main menu.

**Pola yang benar** (dari keyboard.go:478): `{Text: btnBack, ...}, {Text: btnHome, CallbackData: "nav:home"}` di satu row.

**Fix:** Tambah home button row di ictNavKeyboard, smcNavKeyboard, dan gexKeyboard.

---

## Temuan 5: Settings Tidak Ada Global OutputMode Toggle

`domain.UserPrefs.OutputMode` (prefs.go:73) ada di struct dengan nilai `"compact"` atau `"full"`. Ada `cbViewToggle` yang bisa set nilai ini (handler.go:1800-1803). Tapi di `SettingsMenu` keyboard (keyboard.go:320-450) **tidak ada tombol untuk toggle OutputMode secara global**.

User harus klik compact/full toggle per-command di setiap COT/Macro view (yang juga broken per Temuan 1). Tidak ada single toggle di /settings.

**Fix:** Tambah row baru di SettingsMenu:
```
[📊 Mode: Compact -> Full] atau [📖 Mode: Full -> Compact]
```
Callback: `set:outputmode_toggle`. Handle di `cbSettings`.

---

## Implikasi UX

- **Temuan 1 (broken callback)**: Bug critical — fitur yang sudah dibangun tidak berfungsi sama sekali
- **Temuan 2-4 (navigation gaps)**: Inconsistency yang membuat user frustrasi — some commands have home, some don't
- **Temuan 5 (settings gap)**: Discoverability issue — user tidak bisa set default output mode tanpa trial-and-error
