package telegram

import (
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// KeyboardBuilder — Telegram inline keyboard construction
// ---------------------------------------------------------------------------

// KeyboardBuilder creates inline keyboards for interactive bot messages.
// All callback_data values use a prefix-based routing scheme:
//   - "cot:XXX"   -> COT detail for currency/contract
//   - "set:XXX"   -> Settings toggle action
//   - "alert:XXX" -> Alert action (mute, dismiss)
// Standardized button label constants.
const (
	btnExpand  = "📖 Detail Lengkap"
	btnCompact = "📊 Compact"
)

type KeyboardBuilder struct{}

// NewKeyboardBuilder creates a new KeyboardBuilder.
func NewKeyboardBuilder() *KeyboardBuilder {
	return &KeyboardBuilder{}
}


// ---------------------------------------------------------------------------
// Standardized Button Labels
// ---------------------------------------------------------------------------

const (
	// Navigation — generic
	btnBack       = "◀ Kembali"
	btnHome       = "🏠 Menu Utama"
	btnPrevDay    = "◀ Kemarin"
	btnNextDay    = "Besok ▶"
	btnPrevWeek   = "◀ Minggu Lalu"
	btnNextWeek   = "Minggu Depan ▶"
	btnPrevMonth  = "◀ Bulan Lalu"
	btnNextMonth  = "Bulan Depan ▶"

	// Navigation — context-specific back buttons (Indonesian, per UX standard)
	btnBackRingkasan = "◀ Ringkasan"   // back to summary/overview
	btnBackDashboard = "◀ Dashboard"   // back to main section dashboard
	btnBackKategori  = "◀ Kategori"    // back to category list
	btnBackGrid      = "◀ Grid"        // back to seasonal grid overview

	// Calendar
	btnThisMonth = "Bulan Ini"

	// Actions
	btnRefresh = "🔄 Refresh"
	btnClose   = "✖ Tutup"
)


// HomeRow returns a single-row keyboard with the home button.
func (kb *KeyboardBuilder) HomeRow() []ports.InlineButton {
	return []ports.InlineButton{
		{Text: btnHome, CallbackData: "nav:home"},
	}
}
