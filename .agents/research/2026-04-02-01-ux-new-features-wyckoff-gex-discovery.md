# Research Report: UX Siklus 1 Putaran 3 — UX Gaps di Fitur Baru (Wyckoff, GEX, Shortcuts, Sentiment)

**Tanggal:** 2026-04-02 01:00 WIB
**Fokus:** UX/UI Improvement (Siklus 1, Putaran 3)
**Siklus:** 1/5

---

## Ringkasan

Audit UX terhadap fitur yang baru di-merge ke agents/main: Wyckoff (TASK-011), GEX (TASK-012), Command Shortcuts (TASK-028), Crypto Fear & Greed (TASK-056), dan Circuit Breaker Sentiment (TASK-066). Ditemukan gap signifikan di area: educational context, language consistency, feature discovery, keyboard navigation, dan error recovery.

---

## Temuan Utama

### 1. HIGH: Wyckoff & GEX Missing dari /help + Shortcuts Tidak Didokumentasikan
- **File:** `handler.go:413-432` — /help "Research" category lists 9 commands tapi TIDAK termasuk `/wyckoff`
- **File:** `handler.go:220-228` — 11 shortcuts (`/c`, `/cal`, `/out`, dll) terdaftar tapi TIDAK muncul di /help
- **Impact:** User tidak tahu fitur ini exist. Wyckoff engine di-develop tapi tidak discoverable.

### 2. HIGH: Educational Context Missing di GEX & Wyckoff Output
- **GEX:** `formatter_gex.go:30-36` — "Dealers net long gamma — volatility damping" tanpa penjelasan apa itu "gamma" untuk retail trader
- **GEX:** `formatter_gex.go:49-55` — "Max Pain", "Gamma Wall", "Put Wall" shown tanpa definisi
- **Wyckoff:** `formatter_wyckoff.go:82-90` — "Spring", "SOS" (Sign of Strength), "UTAD" tanpa penjelasan
- **Impact:** Retail traders bingung, output hanya berguna untuk institutional traders yang sudah paham terminologi

### 3. HIGH: Language Inconsistency di Fitur Baru
- **Wyckoff:** Indonesian errors (`"tidak tersedia"`, `"tidak dikenal"`) + English phase names
- **GEX:** Full English errors (`"is not configured"`, `"not supported"`)
- **Sentiment:** Mix (`"Penurunan tajam"` + `"Contrarian BUY"`)
- **Impact:** Tidak ada consistency. Sama feature type (error messages) pakai bahasa berbeda.

### 4. MEDIUM: Wyckoff Tidak Punya Keyboard Navigation
- **File:** `handler_wyckoff.go:107-113` — Result dikirim via `SendHTML()` saja, tanpa inline keyboard
- **Contrast:** GEX punya symbol switcher + refresh button (`handler_gex.go:100-106`)
- **Missing:** Symbol selector, timeframe toggle, refresh, related commands
- **Impact:** Dead-end UX — user harus type command manual untuk analysis berikutnya

### 5. MEDIUM: No Error Retry Buttons di Semua Fitur Baru
- **Wyckoff:** `handler_wyckoff.go:96-104` — Error tanpa retry button
- **GEX:** `handler_gex.go:88-92` — "Please try again" tapi harus type manual
- **Impact:** Poor mobile UX, especially di connection buruk

### 6. MEDIUM: GEX Bar Chart Mobile-Unfriendly
- **File:** `formatter_gex.go:147, 156-160` — Hard-coded bar width 8 chars, `▓░` Unicode blocks
- **Impact:** Di narrow screens (split-screen, small phones), bars wrap dan misalign

---

## Task Recommendations (Top 5)

1. **TASK-125**: Update /help — tambah /wyckoff + shortcuts section [HIGH, quick win]
2. **TASK-126**: Educational glossary tooltips untuk GEX & Wyckoff output [HIGH]
3. **TASK-127**: Language standardization di fitur baru (Wyckoff, GEX, Sentiment) [HIGH]
4. **TASK-128**: Wyckoff keyboard navigation + related commands suggestion [MEDIUM]
5. **TASK-129**: Error retry buttons di semua command handlers [MEDIUM]
