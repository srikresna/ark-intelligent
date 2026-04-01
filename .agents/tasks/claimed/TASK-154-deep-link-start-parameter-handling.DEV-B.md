# TASK-154: Deep Link Parameter Handling di /start

**Priority:** medium
**Type:** feature
**Estimated:** M
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 06:00 WIB
**Siklus:** UX

## Deskripsi
/start command menerima `args` parameter dari Telegram deep links (t.me/botname?start=PARAM) tapi TIDAK menggunakannya. Implement parameter handling untuk: referral tracking, command pre-fill, dan onboarding customization.

## Konteks
- `handler.go:257-282` — cmdStart receives `args` tapi ignored
- Telegram format: `t.me/bot?start=cmd_cta_EUR` → args = "cmd_cta_EUR"
- Use cases:
  - `start=ref_12345` → track referral dari user 12345
  - `start=cmd_cot_EUR` → setelah onboarding, auto-execute /cot EUR
  - `start=pro` → skip basic onboarding, show pro features
- Ref: `.agents/research/2026-04-02-06-ux-ai-chat-charts-admin-alerts-deeplink.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Parse `args` di cmdStart:
  - `ref_<userID>` → store referrer in user profile, log referral
  - `cmd_<command>_<symbol>` → cache intent, execute after onboarding completes
  - `<other>` → ignore gracefully (backward compatible)
- [ ] Referral: store ReferrerID di user profile (add field if needed)
- [ ] Command pre-fill: cache di BadgerDB dengan TTL 10 menit, auto-execute setelah onboarding selesai
- [ ] Log semua deep link params untuk analytics

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler.go` (cmdStart)
- `internal/domain/user.go` (add ReferrerID field if needed)
- `internal/adapter/storage/user_repo.go` (store referral)

## Referensi
- `.agents/research/2026-04-02-06-ux-ai-chat-charts-admin-alerts-deeplink.md`
