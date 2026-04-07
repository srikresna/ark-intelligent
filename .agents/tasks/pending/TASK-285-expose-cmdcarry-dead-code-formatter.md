# TASK-285: Expose /carry Command — FormatCarryRanking Dead Code

**Priority:** high
**Type:** feature
**Estimated:** S
**Area:** internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 27:00 WIB

## Deskripsi

`FormatCarryRanking()` di `formatter_quant.go:279` dan `FetchCarryRanking()` di `service/fred/rate_differential.go` keduanya sudah **fully implemented** dan production-ready. Namun **tidak ada command `/carry` yang terdaftar** — formatter adalah dead code yang tidak bisa diakses user.

**Verifikasi:**
- `grep -rn "RegisterCommand.*carry\|cmdCarry" internal/` → empty
- `grep -rn "FormatCarryRanking" internal/` → 1 result, hanya definisi, tidak pernah dipanggil

Carry ranking saat ini hanya digunakan internally oleh scheduler untuk `CarryAdjustment` di ConvictionScore COT. User tidak bisa melihat ranking carry trade secara standalone.

## Perubahan yang Diperlukan

### 1. Tambah `cmdCarry` di `internal/adapter/telegram/handler.go`

```go
// cmdCarry shows the carry trade interest rate differential ranking.
func (h *Handler) cmdCarry(ctx context.Context, chatID string, _ int64, _ string) error {
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, "💹 Fetching carry ranking... ⏳")

	ctx2, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cr, err := h.fredEngine.FetchCarryRanking(ctx2)
	if err != nil {
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"⚠️ Gagal mengambil data carry ranking. Coba lagi nanti.")
	}

	htmlMsg := h.fmt.FormatCarryRanking(cr)
	return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
}
```

### 2. Daftarkan `/carry` command di `registerCommands()` (sekitar baris 209)

```go
bot.RegisterCommand("/carry", h.cmdCarry)
```

### 3. Tambah `/carry` ke help text di `sendHelpSubCategory` (kategori "market")

Di sekitar baris 421 (Market & Data Commands section):
```
/carry — Interest rate differential &amp; carry trade ranking
```

### 4. Tambah `/carry` ke Quick Command keyboard jika ada

Cek `keyboard.go` untuk `cbQuickCommand` — tambahkan carry jika ada slot tersedia.

## File yang Harus Diubah

1. `internal/adapter/telegram/handler.go` — tambah `cmdCarry()` + `RegisterCommand`
2. `internal/adapter/telegram/handler.go` — tambah ke help text

**Tidak perlu** mengubah `formatter_quant.go` atau `service/fred/rate_differential.go` — keduanya sudah siap.

## Verifikasi

```bash
go build ./...
# Manual: /carry → tampilkan carry ranking 7 pairs
# Cek: 🥇 best carry, 🔴 negative carry pairs ditampilkan
```

## Acceptance Criteria

- [ ] `/carry` terdaftar dan menampilkan carry ranking 7 G8 pairs vs USD
- [ ] Format: rate, differential, carry score bar, best/worst carry summary
- [ ] `/help` menyebut `/carry` di seksi Market & Data
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-27-feature-index-gaps-carry-gjrgarch-oi4w-hmm4-vix-putaran19.md` — GAP 1
- `internal/adapter/telegram/formatter_quant.go:279` — FormatCarryRanking (siap pakai)
- `internal/service/fred/rate_differential.go:28` — FetchCarryRanking() 
- `internal/scheduler/scheduler.go:428` — bukti FetchCarryRanking sudah dipakai di internal
