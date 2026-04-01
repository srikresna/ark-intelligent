# TASK-087: ICT Killzone & Session Activity Classifier

**Priority:** high
**Type:** feature
**Estimated:** S (<2h)
**Area:** internal/service/ta
**Created by:** Research Agent
**Created at:** 2026-04-01 15:00 WIB
**Siklus:** Fitur (Siklus 3)

## Deskripsi
Implementasi ICT Killzone classifier di `internal/service/ta/killzone.go` — identifikasi sesi trading aktif saat ini berdasarkan waktu UTC dan tampilkan di /cta output untuk konteks entry timing.

## Konteks
ICT Killzones adalah window waktu dengan aktivitas institusional tertinggi. Mengetahui apakah kita dalam Killzone sangat penting untuk menentukan apakah setup (FVG, OB) valid atau tidak — setup di luar KZ lebih rendah probabilitasnya. Implementasi ini pure time-based, tidak butuh data atau API baru.

Ref: `.agents/research/2026-04-01-15-fitur-baru-ict-smc-wyckoff.md#2e`

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Buat `internal/service/ta/killzone.go` dengan:
  - Struct `KillzoneResult` berisi:
    - ActiveKillzone string ("LONDON_OPEN" / "NY_OPEN" / "NY_CLOSE" / "LONDON_CLOSE" / "ASIA" / "OFF_HOURS")
    - IsActive bool
    - SessionDescription string (human-readable)
    - MinutesUntilNext int (menit hingga killzone berikutnya)
    - NextKillzone string
    - IntersessionOverlap bool (true jika London + NY overlap: 13:00-16:00 UTC)
  - `ClassifyKillzone(t time.Time) KillzoneResult`:
    - London Open: 07:00–10:00 UTC
    - NY Open: 12:00–15:00 UTC
    - London Close: 15:00–16:00 UTC (overlap dengan NY Open)
    - NY Close: 20:00–22:00 UTC
    - Asia Session: 00:00–04:00 UTC
    - IntersessionOverlap = true jika 13:00–16:00 UTC (London-NY overlap, paling volatile)
  - `NextKillzoneInfo(t time.Time) (name string, startsIn int)`:
    - Return nama KZ berikutnya dan berapa menit lagi dimulai
- [ ] Tambahkan field `Killzone *KillzoneResult` ke `IndicatorSnapshot` di `types.go`
- [ ] Panggil `ClassifyKillzone(time.Now().UTC())` dari `engine.go`
- [ ] Tampilkan di /cta output: "⏰ Killzone: LONDON_OPEN aktif (optimal entry window)" atau "Next: NY_OPEN dalam 45 menit"
- [ ] Unit test: test setiap sesi + edge cases (boundary times, midnight)

## File yang Kemungkinan Diubah
- `internal/service/ta/killzone.go` (baru)
- `internal/service/ta/types.go`
- `internal/service/ta/engine.go`
- Formatter /cta output

## Referensi
- `.agents/research/2026-04-01-15-fitur-baru-ict-smc-wyckoff.md`
