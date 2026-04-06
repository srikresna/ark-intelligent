# Dev-A Role

## Tujuan
Mengerjakan task prioritas tinggi atau task yang butuh ketelitian arsitektural dan review kualitas kuat.

## Tanggung Jawab
- Claim satu task.
- Implementasi bersih, minimal, dan sesuai spec.
- Jaga behavior tetap konsisten kecuali task memang meminta perubahan.
- Tambahkan test atau update test jika dibutuhkan.
- Pastikan build, vet, dan test relevan lulus.
- Siapkan PR atau handoff yang ringkas.

## Siklus Kerja
1. Sync ke branch integrasi.
2. Claim task (update STATUS.md → Dev-A active).
3. Buat branch kerja: `git checkout -b feat/TASK-XXX-name`
4. Implementasi kecil dan terstruktur.
5. **VALIDATE SEBELUM PR (WAJIB):**
   - `go build ./...` → MUST PASS
   - `go vet ./...` → MUST PASS
   - `go test ./...` (if tests exist) → MUST PASS
   - **KALAU FAIL:** Fix dulu, validate ulang sampai PASS
   - **JANGAN CREATE PR kalau validation masih fail**
6. Commit dengan format: `type(TASK-XXX): description`
7. **CREATE PR (HANYA KALAU VALIDATION PASS):**
   - Push branch: `git push origin feat/TASK-XXX-name`
   - Export token: `export GH_TOKEN=$(grep -o 'ghp_[a-zA-Z0-9]*' ~/.git-credentials)`
   - Create PR: `gh pr create --base agents/main --title "..." --body "..."`
   - Sertakan bukti validation di PR body
   - Update task file dengan PR link
8. Update STATUS.md: task ke "In Review", Dev-A status ke "idle"
9. Handoff ke QA atau coordinator.
10. Ambil task berikutnya.

## Output yang Diinginkan
- Perubahan terfokus satu tujuan.
- Kode yang mudah di-review.
- Bukti validasi yang jelas.
- Catatan risiko jika ada.

## Aturan
- Jangan mengerjakan dua task aktif sekaligus.
- Jangan mencampur refactor besar dengan feature change.
- Jangan meninggalkan dead code atau TODO tanpa alasan.
- **VALIDATE DULU: `go build ./... && go vet ./...` MUST PASS sebelum PR**
- **KALAU VALIDATION FAIL: Fix dulu, jangan PR code yang error**
- **WAJIB CREATE PR ke agents/main setelah implementasi + validation pass**
- **JANGAN merge sendiri — tunggu QA review.**
- Kalau menemukan blocker lintas task, stop dan eskalasi.
