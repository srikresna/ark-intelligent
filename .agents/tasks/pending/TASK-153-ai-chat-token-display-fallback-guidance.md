# TASK-153: AI Chat Token Usage Display + Fallback Guidance

**Priority:** medium
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram + internal/service/ai
**Created by:** Research Agent
**Created at:** 2026-04-02 06:00 WIB
**Siklus:** UX

## Deskripsi
AI chat collects token metrics (InputTokens, OutputTokens, CacheReadTokens) tapi tidak tampilkan ke user. User tidak tahu quota usage. Juga saat Claude fail → Gemini fallback, message tidak explain kapan retry.

## Konteks
- `chat_service.go:142-157` — token metrics collected tapi unused
- `handler.go:2574-2580` — response sent tanpa token info
- `chat_service.go:171-197` — fallback message vague: "[⚠️ Claude endpoint unreachable]" tanpa guidance
- Ref: `.agents/research/2026-04-02-06-ux-ai-chat-charts-admin-alerts-deeplink.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Append token summary ke setiap AI response (compact format):
  `"\n\n<i>📊 Tokens: {input}+{output} | Cache: {cache}</i>"`
- [ ] Bisa toggle on/off di user prefs (default: off untuk casual users, on untuk power users)
- [ ] Saat Claude fail + Gemini fallback:
  - Jelaskan: "⚠️ Claude sedang tidak tersedia. Response ini dari model alternatif (Gemini)."
  - Guidance: "Kualitas mungkin berbeda. Coba lagi dalam 5-10 menit untuk Claude."
- [ ] Saat semua AI fail (template fallback): suggest manual commands (/cta, /cot, /macro)

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler.go` (append token info)
- `internal/service/ai/chat_service.go` (improve fallback messages)
- `internal/adapter/telegram/formatter.go` (token display formatter)

## Referensi
- `.agents/research/2026-04-02-06-ux-ai-chat-charts-admin-alerts-deeplink.md`
