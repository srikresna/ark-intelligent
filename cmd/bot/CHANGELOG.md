# Changelog - ARK Intelligence Terminal

## [2.0.0] - 2026-03-17
### Economic Calendar & Notification Engine

**📅 Economic Calendar (/calendar)**
- Integrasi penuh MQL5 Economic Calendar — zero API key, scrape langsung dari endpoint tersembunyi MQL5
- View harian, mingguan, dan bulanan dengan navigasi tombol inline
- Actual release values tampil real-time dengan indikator arah 🟢🔴⚪ (numeric parsing, bukan string compare)
- Fix timezone: MQL5 FullDate adalah UTC murni — sebelumnya salah di-parse sebagai New York time sehingga event malam hari "lompat" ke hari berikutnya
- Filter impact: All / High Only / Med+ dengan checkmark ✅ pada filter aktif
- Filter currency per-currency: USD EUR GBP JPY AUD CAD CHF NZD
- Filter tersimpan per-user — tetap aktif saat buka /calendar lagi
- Auto-chunking pesan panjang (> 4096 karakter Telegram limit)

**⏰ Notification Engine**
- Pre-event reminder: notif X menit sebelum event sesuai preferensi user (60/15/5 dll)
- Actual release alert: notif otomatis saat nilai actual keluar dari MQL5
  - Indikator arah 🟢 beat forecast / 🔴 miss forecast / ⚪ neutral
  - AI flash analysis (3 kalimat) jika Gemini tersedia
- Micro-scrape diperluas ke 7 checkpoint: +1, +3, +5, +10, +15, +20, +30 menit setelah event
- Startup check: saat bot restart, langsung cek missed actuals hari ini
- Daily morning summary jam 06:00 WIB per-user sesuai filter preferensi
- Weekly sync otomatis setiap Minggu 23:00 WIB untuk data minggu depan

**⚙️ Settings (/settings)**
- Alert minutes preset: 3 pilihan tombol [60/15/5] [15/5/1] [5/1] dengan ✅ aktif
- Currency filter toggle per-currency: tap untuk aktif/nonaktif, tombol All Currencies untuk reset
- Display settings menampilkan filter aktif dan alert minutes yang dipilih
- Cache GetAllActive 5 menit TTL — DB tidak dipanggil setiap menit, cukup sekali per 5 menit

**🧠 AI Outlook Sinkronisasi**
- /outlook news & /outlook combine kini pakai data calendar yang sama persis (newsRepo.GetByWeek)
- Actual values dari micro-scrape ikut masuk ke prompt AI secara otomatis
- Format prompts mencakup Forecast vs Actual vs Previous untuk analisis trajectory

**🔧 Technical**
- Arsitektur clean ports/adapters: domain → ports → service → adapter
- BadgerDB untuk semua persistence (events, COT, prefs)
- golangci-lint bersih: 0 error pada internal/ dan cmd/
- CHANGELOG.md di-embed ke dalam binary (tidak bergantung working directory runtime)

---

## [1.0.0] - 2026-03-16
### Initial Institutional Release

**Rebranding & UI**
- Rebranded core bot identity to **ARK Community Intelligent**
- Redid /start introductory layout for premium institutional alignment
- Reset project baseline to v1.0.0 representing fully integrated state
- Cleaned up inactive legacy News configuration interfaces
- Removed AI placeholders and generic disclaimers from narratives

**Core COT Diagnostics (Institutional Core)**
- Implemented Disaggregated (Gold/Oil) & TFF (Currency) dual parser engine
- Added high-fidelity Williams COT Index metrics (52W range)
- Implemented crowd heuristics and structural sentiment counters (-100 to +100)

**Scalper / Intraday Intel**
- Added absolute Volume metrics WoW tracking open interest shifts
- Added visual OI trend directions (Rising/Falling/Flat)
- Added Short Term Bias heuristics (Strong Buy / Sell Rallies, etc.)

**Anomali & Struct Shift Alerting**
- Implemented Z-Score variance detector monitoring Asset Managers
- Configured visual warning alerts triggers top-header inside detail analysis screens
- Programmed prompt AI alerts referencing structural shifts

**Raw Data Navigation toggle**
- Created high-speed mapping toggle using callback switching directly in Telegram
- Direct RAW toggle direct output from Socrata database fields
