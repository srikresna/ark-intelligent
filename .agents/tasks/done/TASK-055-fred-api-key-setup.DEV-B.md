# TASK-055: Tambah FRED_API_KEY ke .env.example + Startup Warning

**Priority:** 🔴 CRITICAL  
**Cycle:** Siklus 2 — Data & Integrasi  
**Effort:** ~1 jam  
**Assignee:** Dev-A atau Dev-B

---

## Problem

`FRED_API_KEY` tidak ada di `.env` dan tidak ada di `.env.example`. Namun FRED API **memerlukan** API key — tanpa key, semua request ke FRED mengembalikan HTTP 400. 

Dampak: `/macro` command, yield curve analysis, labor indicators, inflation data — semua bisa gagal secara silent (log debug saja, tidak ada user-facing error).

**Verified:** `curl "https://api.stlouisfed.org/fred/series/observations?series_id=WALCL&limit=2&file_type=json"` → 400 Bad Request (tanpa key).

FRED API key gratis di: https://fred.stlouisfed.org/docs/api/api_key.html

---

## Acceptance Criteria

1. **`.env.example` diupdate** — tambah entry:
   ```
   # Federal Reserve Economic Data (FRED) — Free API key at https://fred.stlouisfed.org/docs/api/api_key.html
   FRED_API_KEY=your_fred_api_key_here
   ```

2. **Startup warning** di `internal/service/fred/fetcher.go` atau `internal/config/config.go`:
   ```go
   if os.Getenv("FRED_API_KEY") == "" {
       log.Warn().Msg("FRED_API_KEY not set — macro data (yields, inflation, labor) will be unavailable. Get free key at https://fred.stlouisfed.org/docs/api/api_key.html")
   }
   ```

3. **README.md atau docs/** — tambah FRED_API_KEY ke setup instructions jika ada.

---

## Files to Edit

- `.env.example` — tambah `FRED_API_KEY`
- `internal/service/fred/fetcher.go` — tambah startup warning di `FetchAll()`
- Optional: `internal/config/config.go` — tambah `FREDAPIKey string` field

---

## Notes

- Jangan hardcode key apapun
- Tidak perlu refactor logic fetch — hanya tambah warning dan dokumentasi
- FRED key gratis dan approved secara instan setelah registrasi online
