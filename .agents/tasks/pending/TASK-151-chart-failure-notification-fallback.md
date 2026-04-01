# TASK-151: Chart Failure User Notification + Text Fallback Message

**Priority:** high
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 06:00 WIB
**Siklus:** UX

## Deskripsi
Saat chart generation gagal (Python crash, timeout, disk full), bot silently fallback ke text tanpa memberitahu user. User tidak tahu chart feature ada atau sedang bermasalah. Tambahkan notifikasi saat chart gagal.

## Konteks
- `handler_cta.go:174-177` — chartErr only logged, never shown
- `handler_cta.go:188-209` — if chartPNG nil, send text tanpa explanation
- Pattern serupa di handler_quant.go, handler_vp.go, handler_ctabt.go
- Ref: `.agents/research/2026-04-02-06-ux-ai-chat-charts-admin-alerts-deeplink.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Saat chart generation fails, prepend message: "📊 <i>Chart sementara tidak tersedia. Menampilkan analisis teks.</i>\n\n"
- [ ] Apply di semua handler yang generate charts: CTA, Quant, VP, CTABT
- [ ] Log chart errors dengan context (symbol, timeframe, error detail) untuk debugging
- [ ] Jangan block text response jika chart gagal — text harus tetap terkirim

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler_cta.go`
- `internal/adapter/telegram/handler_quant.go`
- `internal/adapter/telegram/handler_vp.go`
- `internal/adapter/telegram/handler_ctabt.go` (jika ada chart generation)

## Referensi
- `.agents/research/2026-04-02-06-ux-ai-chat-charts-admin-alerts-deeplink.md`
