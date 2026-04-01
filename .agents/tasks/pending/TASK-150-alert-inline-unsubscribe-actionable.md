# TASK-150: Alert Inline Unsubscribe + Actionable Context

**Priority:** high
**Type:** ux
**Estimated:** M
**Area:** internal/scheduler + internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 06:00 WIB
**Siklus:** UX

## Deskripsi
COT dan FRED alerts dikirim tanpa opsi unsubscribe inline dan tanpa actionable context. User harus ingat untuk ke /settings untuk disable. Ini menyebabkan alert fatigue → user mute bot sepenuhnya. Tambahkan inline keyboard + suggested commands.

## Konteks
- `scheduler.go:329-356` — COT broadcast tanpa unsubscribe button
- `scheduler.go:508-550` — FRED alerts sama
- `fred/alerts.go:54-77` — Alert content explain what happened tapi tidak suggest what to do
- 18+ alert types exist — user bisa dapat 3-5 alerts/hari
- Ref: `.agents/research/2026-04-02-06-ux-ai-chat-charts-admin-alerts-deeplink.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Setiap alert message include inline keyboard:
  - [📊 Lihat Detail] → navigate ke relevant command (/cot, /macro, /sentiment)
  - [🔕 Matikan Alert Ini] → toggle specific alert type off di user prefs
  - [⚙️ Pengaturan Alert] → navigate ke /settings
- [ ] Setiap alert include 1 actionable suggestion: "Lihat dampak: /macro" atau "Cek positioning: /cot EUR"
- [ ] Alert content include brief "Apa yang harus dilakukan" section
- [ ] Callback handler untuk inline unsubscribe — update user prefs, show toast "✅ Alert dimatikan"

## File yang Kemungkinan Diubah
- `internal/scheduler/scheduler.go` (COT + FRED broadcast formatting)
- `internal/service/fred/alerts.go` (alert content enhancement)
- `internal/adapter/telegram/keyboard.go` (alert keyboard builder)
- `internal/adapter/telegram/handler.go` (callback for unsubscribe)

## Referensi
- `.agents/research/2026-04-02-06-ux-ai-chat-charts-admin-alerts-deeplink.md`
