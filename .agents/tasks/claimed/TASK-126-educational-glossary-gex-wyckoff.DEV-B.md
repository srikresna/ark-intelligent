# TASK-126: Educational Glossary Tooltips untuk GEX & Wyckoff

**Priority:** high
**Type:** ux
**Estimated:** M
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 01:00 WIB
**Siklus:** UX

## Deskripsi
GEX dan Wyckoff output menggunakan terminologi advanced tanpa penjelasan. Retail trader tidak mengerti "Gamma Exposure", "Max Pain", "Spring", "SOS", "UTAD". Tambahkan inline educational context atau glossary section di output.

## Konteks
- GEX: `formatter_gex.go:30-36` — "Dealers net long gamma" tanpa penjelasan
- GEX: `formatter_gex.go:49-55` — "Max Pain", "Gamma Wall", "Put Wall" tanpa definisi
- Wyckoff: `formatter_wyckoff.go:82-90` — "Spring", "SOS" (Sign of Strength), "UTAD" (Upthrust After Distribution) tanpa penjelasan
- Target audience = retail forex traders, bukan institutional options desk
- Ref: `.agents/research/2026-04-02-01-ux-new-features-wyckoff-gex-discovery.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] GEX output: tambah inline penjelasan singkat (1 baris) di bawah key terms:
  - "Gamma Exposure (GEX): ukuran sensitivitas harga terhadap posisi options dealer"
  - "Max Pain: harga di mana options expiring cause least pain to holders"
  - "Gamma Wall: strike price dengan gamma tertinggi — magnet harga"
  - "Put Wall: level support terkuat dari options positioning"
- [ ] Wyckoff output: tambah inline penjelasan:
  - "Spring: false breakdown di bawah support — sinyal reversal bullish"
  - "SOS (Sign of Strength): breakout di atas resistance dengan volume tinggi"
  - "UTAD (Upthrust After Distribution): false breakout di atas resistance — sinyal reversal bearish"
- [ ] Penjelasan dalam bahasa Indonesia (sesuai default bot)
- [ ] Tidak membuat output terlalu panjang — max 2 baris per term
- [ ] Pertimbangkan: toggle glossary on/off di user settings (/prefs)

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/formatter_gex.go`
- `internal/adapter/telegram/formatter_wyckoff.go`

## Referensi
- `.agents/research/2026-04-02-01-ux-new-features-wyckoff-gex-discovery.md`
