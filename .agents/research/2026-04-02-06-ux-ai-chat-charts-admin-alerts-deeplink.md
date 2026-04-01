# Research Report: UX Siklus 1 Putaran 4 — AI Chat, Charts, Admin, Alerts, Deep Links

**Tanggal:** 2026-04-02 06:00 WIB
**Fokus:** UX/UI Improvement (Siklus 1, Putaran 4)
**Siklus:** 1/5

---

## Ringkasan

Deep dive ke 5 area UX baru: AI chat experience, chart/image UX, admin actions, alert fatigue, deep link support. 20 issues ditemukan across all areas. 5 task dipilih berdasarkan impact tertinggi dan feasibility.

---

## Area 1: AI Chat Experience

### Temuan Kritis
- **No streaming** — user lihat "Thinking..." statis selama 20+ detik (`handler.go:2509-2546`)
- **Token usage hidden** — metrics di-collect (`chat_service.go:142-157`) tapi TIDAK ditampilkan ke user
- **Fallback vague** — Claude fail → Gemini fallback message (`chat_service.go:171-197`) tanpa guidance kapan retry
- **Voice message** — `chat.go:95-103` kirim placeholder "[Voice not supported]" ke context, wasting tokens

## Area 2: Chart/Image UX

### Temuan Kritis
- **No alt-text/caption** — charts dikirim tanpa deskripsi untuk accessibility (`api.go:273-281`)
- **Silent chart failure** — jika Python chart generation gagal, fallback ke text tanpa memberitahu user (`handler_cta.go:174-209`)
- **No colorblind legend** — charts rely purely on color without text legend

## Area 3: Admin UX

### Temuan Kritis
- **No confirmation** — `/ban <userID>` langsung execute, tanpa "Are you sure?" (`handler.go:2388-2464`)
- **No audit trail** — admin actions tidak di-log ke owner atau affected user
- **Unban loses tier** — `/unban` selalu set ke RoleFree, kehilangan tier sebelumnya (`handler.go:2456`)

## Area 4: Alert UX

### Temuan Kritis
- **No inline unsubscribe** — COT/FRED alerts tanpa button "Disable alerts" (`scheduler.go:329-356`)
- **Alert fatigue** — 18+ alert types, no batching/throttling, user bisa dapat 3-5 alerts/hari
- **Missing actionable context** — alerts explain what happened tapi TIDAK suggest what to do
- **No quiet hours** — alerts fire at any time including 3 AM

## Area 5: Deep Link

### Temuan Kritis
- **`args` parameter ignored** — `/start` handler receives deep link args tapi tidak pakai (`handler.go:257-282`)
- **No referral structure** — no tracking, no bonus, no growth mechanism

---

## Task Recommendations (Top 5 High-Impact)

1. **TASK-150**: Alert inline unsubscribe + actionable context [HIGH] — alert fatigue #1 cause of bot muting
2. **TASK-151**: Chart failure user notification + text fallback message [HIGH] — silent degradation
3. **TASK-152**: Admin confirmation flow for destructive actions [HIGH] — ban/unban safety
4. **TASK-153**: AI chat token usage display + fallback guidance [MEDIUM] — transparency
5. **TASK-154**: Deep link parameter handling di /start [MEDIUM] — growth enabler
