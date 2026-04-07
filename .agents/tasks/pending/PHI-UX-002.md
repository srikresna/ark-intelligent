# PHI-UX-002: Add Command Aliases

**ID:** PHI-UX-002  
**Title:** Add Command Aliases  
**Priority:** LOW  
**Type:** ux  
**Estimated:** S (<2h)  
**Area:** internal/adapter  
**Assignee:** 

---

## Deskripsi

Tambah command aliases (shortcuts) untuk commands yang sering dipakai. Ini meningkatkan UX untuk power users yang ingin akses cepat.

## Konteks

Dari UX audit, user overwhelmed dengan 30+ commands. Command aliases membantu power users untuk akses cepat tanpa mengetik command full.

## Acceptance Criteria

- [ ] `/c` → `/cot`
- [ ] `/m` → `/macro`
- [ ] `/cal` → `/calendar`
- [ ] `/out` → `/outlook`
- [ ] `/q` → `/quant`
- [ ] Update command router di `handler.go` atau buat `command_router.go`
- [ ] Tambah help text untuk aliases di `/help`
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean

## Alias Mapping

| Alias | Full Command | Keterangan |
|-------|--------------|------------|
| `/c` | `/cot` | COT positioning |
| `/m` | `/macro` | Macro dashboard |
| `/cal` | `/calendar` | Economic calendar |
| `/out` | `/outlook` | AI outlook |
| `/q` | `/quant` | Quant analysis |

## Files yang Akan Dibuat/Diubah

- `internal/adapter/telegram/handler.go` — tambah alias mapping (modifikasi)
- Atau buat baru: `internal/adapter/telegram/command_router.go`

## Referensi

- `.agents/UX_AUDIT.md` — bagian "Command Shortcuts"

---

## Claim Instructions

1. Pastikan PHI-SETUP-001 sudah selesai
2. Copy file ini ke `.agents/tasks/in-progress/PHI-UX-002.md`
3. Update field **Assignee** dengan `Dev-B`
4. Update `.agents/STATUS.md`
5. Buat branch: `git checkout -b ux/PHI-002-command-aliases`
6. Implement dan test
7. Setelah selesai, move ke `done/` dan update STATUS.md
