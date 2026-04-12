package telegram

import (
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Backtest Keyboards
// ---------------------------------------------------------------------------

// BacktestMenu builds the backtest sub-command selection keyboard.
// Organized into three sections: Core, Analysis, Advanced, plus currency drill-down.
func (kb *KeyboardBuilder) BacktestMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			// --- Core ---
			{
				{Text: "📊 Overview", CallbackData: "cmd:backtest:all"},
				{Text: "📋 By Signal Type", CallbackData: "cmd:backtest:signals"},
			},
			// --- Analysis ---
			{
				{Text: "⏱ Timing", CallbackData: "cmd:backtest:timing"},
				{Text: "🔄 Walk-Forward", CallbackData: "cmd:backtest:wf"},
				{Text: "⚖️ Weights", CallbackData: "cmd:backtest:weights"},
			},
			{
				{Text: "🧠 Smart Money", CallbackData: "cmd:backtest:sm"},
				{Text: "📊 MFE/MAE", CallbackData: "cmd:backtest:excursion"},
				{Text: "📈 Trend", CallbackData: "cmd:backtest:trend"},
			},
			// --- Advanced ---
			{
				{Text: "🎯 Baseline", CallbackData: "cmd:backtest:baseline"},
				{Text: "🌐 Regime", CallbackData: "cmd:backtest:regime"},
				{Text: "📐 Matrix", CallbackData: "cmd:backtest:matrix"},
			},
			{
				{Text: "🎲 Monte Carlo", CallbackData: "cmd:backtest:mc"},
				{Text: "📈 Portfolio", CallbackData: "cmd:backtest:portfolio"},
				{Text: "💰 Cost", CallbackData: "cmd:backtest:cost"},
			},
			{
				{Text: "🔗 Dedup", CallbackData: "cmd:backtest:dedup"},
				{Text: "🎰 Ruin", CallbackData: "cmd:backtest:ruin"},
				{Text: "🔍 Audit", CallbackData: "cmd:backtest:audit"},
			},
			// --- Currency drill-down ---
			{
				{Text: "EUR", CallbackData: "cmd:backtest:EUR"},
				{Text: "GBP", CallbackData: "cmd:backtest:GBP"},
				{Text: "JPY", CallbackData: "cmd:backtest:JPY"},
				{Text: "AUD", CallbackData: "cmd:backtest:AUD"},
			},
			{
				{Text: "NZD", CallbackData: "cmd:backtest:NZD"},
				{Text: "CAD", CallbackData: "cmd:backtest:CAD"},
				{Text: "CHF", CallbackData: "cmd:backtest:CHF"},
				{Text: "GOLD", CallbackData: "cmd:backtest:XAU"},
			},
			{
				{Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Seasonal Keyboards
// ---------------------------------------------------------------------------

// SeasonalMenu builds a currency selector keyboard for the /seasonal grid view.
// Provides quick-access buttons for deep-dive into individual currencies.
func (kb *KeyboardBuilder) SeasonalMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			// FX Majors
			{
				{Text: "EUR", CallbackData: "cmd:seasonal:EUR"},
				{Text: "GBP", CallbackData: "cmd:seasonal:GBP"},
				{Text: "JPY", CallbackData: "cmd:seasonal:JPY"},
				{Text: "CHF", CallbackData: "cmd:seasonal:CHF"},
			},
			{
				{Text: "AUD", CallbackData: "cmd:seasonal:AUD"},
				{Text: "NZD", CallbackData: "cmd:seasonal:NZD"},
				{Text: "CAD", CallbackData: "cmd:seasonal:CAD"},
				{Text: "DXY", CallbackData: "cmd:seasonal:USD"},
			},
			// Metals & Energy
			{
				{Text: "🥇 Gold", CallbackData: "cmd:seasonal:XAU"},
				{Text: "🥈 Silver", CallbackData: "cmd:seasonal:XAG"},
				{Text: "🛢 Oil", CallbackData: "cmd:seasonal:OIL"},
				{Text: "🔶 Copper", CallbackData: "cmd:seasonal:COPPER"},
			},
			{
				{Text: "⛽ ULSD", CallbackData: "cmd:seasonal:ULSD"},
				{Text: "⛽ RBOB", CallbackData: "cmd:seasonal:RBOB"},
			},
			// Indices
			{
				{Text: "S&P500", CallbackData: "cmd:seasonal:SPX500"},
				{Text: "Nasdaq", CallbackData: "cmd:seasonal:NDX"},
				{Text: "Dow", CallbackData: "cmd:seasonal:DJI"},
				{Text: "Russell", CallbackData: "cmd:seasonal:RUT"},
			},
			// Bonds
			{
				{Text: "🏛 2Y", CallbackData: "cmd:seasonal:BOND2"},
				{Text: "🏛 5Y", CallbackData: "cmd:seasonal:BOND5"},
				{Text: "🏛 10Y", CallbackData: "cmd:seasonal:BOND"},
				{Text: "🏛 30Y", CallbackData: "cmd:seasonal:BOND30"},
			},
			// Crypto & Crosses
			{
				{Text: "₿ BTC", CallbackData: "cmd:seasonal:BTC"},
				{Text: "Ξ ETH", CallbackData: "cmd:seasonal:ETH"},
			},
			{
				{Text: "XAU/EUR", CallbackData: "cmd:seasonal:XAUEUR"},
				{Text: "XAU/GBP", CallbackData: "cmd:seasonal:XAUGBP"},
				{Text: "XAG/EUR", CallbackData: "cmd:seasonal:XAGEUR"},
				{Text: "XAG/GBP", CallbackData: "cmd:seasonal:XAGGBP"},
			},
			{
				{Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// SeasonalDetailMenu builds a navigation keyboard for a single-currency seasonal deep dive.
func (kb *KeyboardBuilder) SeasonalDetailMenu(currency string) ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackGrid, CallbackData: "cmd:seasonal"},
				{Text: "💹 Price", CallbackData: fmt.Sprintf("cmd:price:%s", currency)},
				{Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// CTA Keyboards
// ---------------------------------------------------------------------------

// CTAMenu builds the inline keyboard for the /cta dashboard.
func (kb *KeyboardBuilder) CTAMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 15m", CallbackData: "cta:tf:15m"},
				{Text: "📊 30m", CallbackData: "cta:tf:30m"},
				{Text: "📊 1H", CallbackData: "cta:tf:1h"},
				{Text: "📊 4H", CallbackData: "cta:tf:4h"},
			},
			{
				{Text: "📊 6H", CallbackData: "cta:tf:6h"},
				{Text: "📊 12H", CallbackData: "cta:tf:12h"},
				{Text: "📊 Daily", CallbackData: "cta:tf:daily"},
			},
			{
				{Text: "🏯 Ichimoku", CallbackData: "cta:ichi"},
				{Text: "📐 Fibonacci", CallbackData: "cta:fib"},
				{Text: "🕯 Patterns", CallbackData: "cta:patterns"},
			},
			{
				{Text: "⚡ Confluence", CallbackData: "cta:confluence"},
				{Text: "📱 Multi-TF", CallbackData: "cta:mtf"},
				{Text: "🎯 Zones", CallbackData: "cta:zones"},
			},
			{
				{Text: "📏 VWAP+Delta", CallbackData: "cta:vwap_delta"},
				{Text: "🔄 Refresh", CallbackData: "cta:refresh"},
			},
		},
	}
}

// CTADetailMenu builds the back-navigation keyboard for CTA detail views.
func (kb *KeyboardBuilder) CTADetailMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackRingkasan, CallbackData: "cta:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// CTATimeframeMenu builds the timeframe selection keyboard for CTA.
func (kb *KeyboardBuilder) CTATimeframeMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 15m", CallbackData: "cta:tf:15m"},
				{Text: "📊 30m", CallbackData: "cta:tf:30m"},
				{Text: "📊 1H", CallbackData: "cta:tf:1h"},
				{Text: "📊 4H", CallbackData: "cta:tf:4h"},
			},
			{
				{Text: "📊 6H", CallbackData: "cta:tf:6h"},
				{Text: "📊 12H", CallbackData: "cta:tf:12h"},
				{Text: "📊 Daily", CallbackData: "cta:tf:daily"},
			},
			{
				{Text: btnBackRingkasan, CallbackData: "cta:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// CTABTMenu builds the inline keyboard for the /ctabt backtest dashboard.
func (kb *KeyboardBuilder) CTABTMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 Daily", CallbackData: "ctabt:daily"},
				{Text: "📊 12H", CallbackData: "ctabt:12h"},
				{Text: "📊 6H", CallbackData: "ctabt:6h"},
			},
			{
				{Text: "📊 4H", CallbackData: "ctabt:4h"},
				{Text: "📊 1H", CallbackData: "ctabt:1h"},
				{Text: "📊 30M", CallbackData: "ctabt:30m"},
				{Text: "📊 15M", CallbackData: "ctabt:15m"},
			},
			{
				{Text: "Grade: A", CallbackData: "ctabt:gradeA"},
				{Text: "Grade: B", CallbackData: "ctabt:gradeB"},
				{Text: "Grade: C", CallbackData: "ctabt:gradeC"},
			},
			{
				{Text: "📋 Detail Trades", CallbackData: "ctabt:trades"},
				{Text: "🔄 Refresh", CallbackData: "ctabt:refresh"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Price Menu
// ---------------------------------------------------------------------------

// PriceMenu builds a categorized currency selection keyboard for the /price command.
func (kb *KeyboardBuilder) PriceMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			// --- FX Majors ---
			{
				{Text: "EUR", CallbackData: "cmd:price:EUR"},
				{Text: "GBP", CallbackData: "cmd:price:GBP"},
				{Text: "JPY", CallbackData: "cmd:price:JPY"},
				{Text: "CHF", CallbackData: "cmd:price:CHF"},
			},
			{
				{Text: "AUD", CallbackData: "cmd:price:AUD"},
				{Text: "NZD", CallbackData: "cmd:price:NZD"},
				{Text: "CAD", CallbackData: "cmd:price:CAD"},
				{Text: "DXY", CallbackData: "cmd:price:USD"},
			},
			// --- Metals & Energy ---
			{
				{Text: "🥇 Gold", CallbackData: "cmd:price:XAU"},
				{Text: "🥈 Silver", CallbackData: "cmd:price:XAG"},
				{Text: "🛢 Oil", CallbackData: "cmd:price:OIL"},
				{Text: "🔶 Copper", CallbackData: "cmd:price:COPPER"},
			},
			// --- Indices ---
			{
				{Text: "📈 S&P500", CallbackData: "cmd:price:SPX500"},
				{Text: "📈 Nasdaq", CallbackData: "cmd:price:NDX"},
				{Text: "📈 Dow", CallbackData: "cmd:price:DJI"},
				{Text: "📈 Russell", CallbackData: "cmd:price:RUT"},
			},
			// --- Bonds ---
			{
				{Text: "🏛 2Y", CallbackData: "cmd:price:BOND2"},
				{Text: "🏛 5Y", CallbackData: "cmd:price:BOND5"},
				{Text: "🏛 10Y", CallbackData: "cmd:price:BOND"},
				{Text: "🏛 30Y", CallbackData: "cmd:price:BOND30"},
			},
			// --- Crypto & Energy ---
			{
				{Text: "₿ BTC", CallbackData: "cmd:price:BTC"},
				{Text: "Ξ ETH", CallbackData: "cmd:price:ETH"},
				{Text: "⛽ ULSD", CallbackData: "cmd:price:ULSD"},
				{Text: "⛽ RBOB", CallbackData: "cmd:price:RBOB"},
			},
			// --- Cross Pairs ---
			{
				{Text: "XAU/EUR", CallbackData: "cmd:price:XAUEUR"},
				{Text: "XAU/GBP", CallbackData: "cmd:price:XAUGBP"},
				{Text: "XAG/EUR", CallbackData: "cmd:price:XAGEUR"},
				{Text: "XAG/GBP", CallbackData: "cmd:price:XAGGBP"},
			},
			{
				{Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Quant Keyboards
// ---------------------------------------------------------------------------

// QuantMenu builds the main /quant dashboard inline keyboard.
func (kb *KeyboardBuilder) QuantMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 Stats", CallbackData: "quant:stats"},
				{Text: "📈 Volatility", CallbackData: "quant:garch"},
				{Text: "🔗 Correlation", CallbackData: "quant:corr"},
			},
			{
				{Text: "📅 Seasonal", CallbackData: "quant:seasonal"},
				{Text: "🔄 Mean Revert", CallbackData: "quant:meanrevert"},
				{Text: "⚡ Granger", CallbackData: "quant:granger"},
			},
			{
				{Text: "🎭 Regime (HMM)", CallbackData: "quant:regime"},
				{Text: "🔗 Cointegration", CallbackData: "quant:coint"},
			},
			{
				{Text: "🧬 PCA", CallbackData: "quant:pca"},
				{Text: "🌐 VAR", CallbackData: "quant:var"},
				{Text: "⚠️ Risk", CallbackData: "quant:risk"},
			},
			{
				{Text: "📋 Full Report", CallbackData: "quant:full"},
			},
			// Backtest button
			{
				{Text: "📊 Backtest All Models", CallbackData: "quant:backtest"},
			},
			{
				{Text: "15m", CallbackData: "quant:tf:15m"},
				{Text: "30m", CallbackData: "quant:tf:30m"},
				{Text: "1H", CallbackData: "quant:tf:1h"},
				{Text: "4H", CallbackData: "quant:tf:4h"},
			},
			{
				{Text: "6H", CallbackData: "quant:tf:6h"},
				{Text: "12H", CallbackData: "quant:tf:12h"},
				{Text: "📊 Daily", CallbackData: "quant:tf:daily"},
			},
		},
	}
}

// QuantDetailMenu builds the back-navigation keyboard for quant detail views.
func (kb *KeyboardBuilder) QuantDetailMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackDashboard, CallbackData: "quant:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Volume Profile Keyboards
// ---------------------------------------------------------------------------

// VPMenu builds the main /vp Volume Profile dashboard keyboard.
func (kb *KeyboardBuilder) VPMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			// Analysis modes
			{
				{Text: "📊 Profile", CallbackData: "vp:profile"},
				{Text: "🕐 Session", CallbackData: "vp:session"},
				{Text: "📐 Shape", CallbackData: "vp:shape"},
			},
			{
				{Text: "🔀 Composite", CallbackData: "vp:composite"},
				{Text: "📏 VWAP", CallbackData: "vp:vwap"},
				{Text: "⏱ TPO", CallbackData: "vp:tpo"},
			},
			{
				{Text: "📈 Delta", CallbackData: "vp:delta"},
				{Text: "🏛 Auction", CallbackData: "vp:auction"},
				{Text: "🎯 Confluence", CallbackData: "vp:confluence"},
			},
			{
				{Text: "📋 Full Report", CallbackData: "vp:full"},
			},
			// TF selector
			{
				{Text: "15m", CallbackData: "vp:tf:15m"},
				{Text: "30m", CallbackData: "vp:tf:30m"},
				{Text: "1H", CallbackData: "vp:tf:1h"},
				{Text: "4H", CallbackData: "vp:tf:4h"},
			},
			{
				{Text: "6H", CallbackData: "vp:tf:6h"},
				{Text: "12H", CallbackData: "vp:tf:12h"},
				{Text: "📅 Daily", CallbackData: "vp:tf:daily"},
				{Text: "🔄 Refresh", CallbackData: "vp:refresh"},
			},
		},
	}
}

// VPDetailMenu builds the back-navigation keyboard for VP detail views.
func (kb *KeyboardBuilder) VPDetailMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackDashboard, CallbackData: "vp:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// VPSymbolMenu builds a symbol selector for /vp (no argument).
func (kb *KeyboardBuilder) VPSymbolMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			// FX Majors
			{
				{Text: "EUR", CallbackData: "vp:sym:EUR"},
				{Text: "GBP", CallbackData: "vp:sym:GBP"},
				{Text: "JPY", CallbackData: "vp:sym:JPY"},
				{Text: "CHF", CallbackData: "vp:sym:CHF"},
			},
			{
				{Text: "AUD", CallbackData: "vp:sym:AUD"},
				{Text: "NZD", CallbackData: "vp:sym:NZD"},
				{Text: "CAD", CallbackData: "vp:sym:CAD"},
				{Text: "DXY", CallbackData: "vp:sym:USD"},
			},
			// Metals & Energy
			{
				{Text: "🥇 Gold", CallbackData: "vp:sym:XAU"},
				{Text: "🥈 Silver", CallbackData: "vp:sym:XAG"},
				{Text: "🛢 Oil", CallbackData: "vp:sym:OIL"},
				{Text: "🔶 Copper", CallbackData: "vp:sym:COPPER"},
			},
			// Indices
			{
				{Text: "S&P500", CallbackData: "vp:sym:SPX500"},
				{Text: "Nasdaq", CallbackData: "vp:sym:NDX"},
				{Text: "Dow", CallbackData: "vp:sym:DJI"},
				{Text: "Russell", CallbackData: "vp:sym:RUT"},
			},
			// Bonds & Crypto
			{
				{Text: "🏛 10Y", CallbackData: "vp:sym:BOND"},
				{Text: "🏛 30Y", CallbackData: "vp:sym:BOND30"},
				{Text: "₿ BTC", CallbackData: "vp:sym:BTC"},
				{Text: "Ξ ETH", CallbackData: "vp:sym:ETH"},
			},
			// Energy & Crosses
			{
				{Text: "⛽ ULSD", CallbackData: "vp:sym:ULSD"},
				{Text: "⛽ RBOB", CallbackData: "vp:sym:RBOB"},
				{Text: "XAU/EUR", CallbackData: "vp:sym:XAUEUR"},
				{Text: "XAU/GBP", CallbackData: "vp:sym:XAUGBP"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Alpha Keyboards
// ---------------------------------------------------------------------------

// AlphaMenu builds the inline keyboard for the unified /radar dashboard.
func (kb *KeyboardBuilder) AlphaMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 Factor Ranking", CallbackData: "alpha:factors"},
				{Text: "📡 Signal Intensity", CallbackData: "alpha:heat"},
			},
			{
				{Text: "🔄 Regime & Transisi", CallbackData: "alpha:transition"},
				{Text: "⚡ Crypto Alpha", CallbackData: "alpha:crypto"},
			},
			{
				{Text: "🔄 Refresh Data", CallbackData: "alpha:refresh"},
			},
		},
	}
}

// AlphaDetailMenu builds the back-navigation keyboard for alpha detail views.
func (kb *KeyboardBuilder) AlphaDetailMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackRingkasan, CallbackData: "alpha:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// AlphaCryptoDetailMenu builds the back-navigation keyboard for alpha crypto detail views
// with individual crypto symbol buttons.
func (kb *KeyboardBuilder) AlphaCryptoDetailMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "₿ BTC", CallbackData: "alpha:crypto:BTC"},
				{Text: "Ξ ETH", CallbackData: "alpha:crypto:ETH"},
				{Text: "◎ SOL", CallbackData: "alpha:crypto:SOL"},
				{Text: "🔶 BNB", CallbackData: "alpha:crypto:BNB"},
			},
			{
				{Text: btnBackRingkasan, CallbackData: "alpha:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Symbol Selectors (reusable)
// ---------------------------------------------------------------------------

// CTASymbolMenu builds a symbol selector for /cta.
func (kb *KeyboardBuilder) CTASymbolMenu() ports.InlineKeyboard {
	return kb.buildSymbolMenu("cta")
}

// CTABTSymbolMenu builds a symbol selector for /ctabt.
func (kb *KeyboardBuilder) CTABTSymbolMenu() ports.InlineKeyboard {
	return kb.buildSymbolMenu("ctabt")
}

// QuantSymbolMenu builds a symbol selector for /quant.
func (kb *KeyboardBuilder) QuantSymbolMenu() ports.InlineKeyboard {
	return kb.buildSymbolMenu("quant")
}

// buildSymbolMenu creates a reusable symbol selector grid.
func (kb *KeyboardBuilder) buildSymbolMenu(prefix string) ports.InlineKeyboard {
	p := func(sym string) string { return prefix + ":sym:" + sym }
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "EUR", CallbackData: p("EUR")},
				{Text: "GBP", CallbackData: p("GBP")},
				{Text: "JPY", CallbackData: p("JPY")},
				{Text: "CHF", CallbackData: p("CHF")},
			},
			{
				{Text: "AUD", CallbackData: p("AUD")},
				{Text: "NZD", CallbackData: p("NZD")},
				{Text: "CAD", CallbackData: p("CAD")},
				{Text: "DXY", CallbackData: p("USD")},
			},
			{
				{Text: "🥇 Gold", CallbackData: p("XAU")},
				{Text: "🥈 Silver", CallbackData: p("XAG")},
				{Text: "🛢 Oil", CallbackData: p("OIL")},
				{Text: "🔶 Copper", CallbackData: p("COPPER")},
			},
			{
				{Text: "S&P500", CallbackData: p("SPX500")},
				{Text: "Nasdaq", CallbackData: p("NDX")},
				{Text: "Dow", CallbackData: p("DJI")},
				{Text: "Russell", CallbackData: p("RUT")},
			},
			{
				{Text: "🏛 10Y", CallbackData: p("BOND")},
				{Text: "🏛 30Y", CallbackData: p("BOND30")},
				{Text: "₿ BTC", CallbackData: p("BTC")},
				{Text: "Ξ ETH", CallbackData: p("ETH")},
			},
			{
				{Text: "⛽ ULSD", CallbackData: p("ULSD")},
				{Text: "⛽ RBOB", CallbackData: p("RBOB")},
				{Text: "XAU/EUR", CallbackData: p("XAUEUR")},
				{Text: "XAU/GBP", CallbackData: p("XAUGBP")},
			},
		},
	}
}

// buildSymbolMenuWithLast is like buildSymbolMenu but prepends a "Same as last" row
// if lastCurrency is non-empty and recognized.
func (kb *KeyboardBuilder) buildSymbolMenuWithLast(prefix, lastCurrency string) ports.InlineKeyboard {
	base := kb.buildSymbolMenu(prefix)
	if lastCurrency == "" {
		return base
	}
	// Build label for last currency button
	label := "🔄 Same as last: " + lastCurrency
	lastRow := []ports.InlineButton{
		{Text: label, CallbackData: prefix + ":sym:" + lastCurrency},
	}
	// Prepend the shortcut row
	rows := append([][]ports.InlineButton{lastRow}, base.Rows...)
	return ports.InlineKeyboard{Rows: rows}
}

// CTASymbolMenuWithLast builds a CTA symbol menu with a "same as last" shortcut.
func (kb *KeyboardBuilder) CTASymbolMenuWithLast(lastCurrency string) ports.InlineKeyboard {
	return kb.buildSymbolMenuWithLast("cta", lastCurrency)
}

// QuantSymbolMenuWithLast builds a Quant symbol menu with a "same as last" shortcut.
func (kb *KeyboardBuilder) QuantSymbolMenuWithLast(lastCurrency string) ports.InlineKeyboard {
	return kb.buildSymbolMenuWithLast("quant", lastCurrency)
}

// ---------------------------------------------------------------------------
// History & Backtest Navigation
// ---------------------------------------------------------------------------

// HistoryNavKeyboard builds the week-switcher keyboard for /history command.
// currentWeeks: 4, 8, or 12 — highlights the active selection with a ✓ prefix.
func (kb *KeyboardBuilder) HistoryNavKeyboard(currency string, currentWeeks int) ports.InlineKeyboard {
	label := func(w int) string {
		if w == currentWeeks {
			return fmt.Sprintf("✓ %dW", w)
		}
		return fmt.Sprintf("%dW", w)
	}
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: label(4), CallbackData: fmt.Sprintf("hist:%s:4", currency)},
				{Text: label(8), CallbackData: fmt.Sprintf("hist:%s:8", currency)},
				{Text: label(12), CallbackData: fmt.Sprintf("hist:%s:12", currency)},
			},
			kb.HomeRow(),
		},
	}
}

// BacktestBackRow returns a keyboard with [📊 Backtest Menu] [🏠 Menu Utama] buttons.
func (kb *KeyboardBuilder) BacktestBackRow() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 Backtest Menu", CallbackData: "cmd:backtest"},
				{Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Quant Backtest Keyboards
// ---------------------------------------------------------------------------

// QBacktestMenu builds the quant backtest result navigation keyboard.
func (kb *KeyboardBuilder) QBacktestMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "🔄 Refresh", CallbackData: "qbacktest:refresh"},
				{Text: "🔬 Quant Dashboard", CallbackData: "qbacktest:back"},
			},
			{
				{Text: "🏠 Home", CallbackData: "nav:home"},
			},
		},
	}
}

// QuantBacktestModelMenu builds model selection for quant backtest.
func (kb *KeyboardBuilder) QuantBacktestModelMenu(symbol string) ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 Stats", CallbackData: "qbacktest:model:stats"},
				{Text: "📉 GARCH", CallbackData: "qbacktest:model:garch"},
				{Text: "🔗 Correlation", CallbackData: "qbacktest:model:correlation"},
			},
			{
				{Text: "🎭 Regime", CallbackData: "qbacktest:model:regime"},
				{Text: "🔄 Mean Revert", CallbackData: "qbacktest:model:meanrevert"},
				{Text: "⚡ Granger", CallbackData: "qbacktest:model:granger"},
			},
			{
				{Text: "🔗 Cointegration", CallbackData: "qbacktest:model:cointegration"},
				{Text: "🧬 PCA", CallbackData: "qbacktest:model:pca"},
				{Text: "🌐 VAR", CallbackData: "qbacktest:model:var"},
			},
			{
				{Text: "⚠️ Risk", CallbackData: "qbacktest:model:risk"},
				{Text: "📋 All Models", CallbackData: "qbacktest:model:"},
			},
			{
				{Text: "🔙 Back", CallbackData: "qbacktest:back"},
			},
		},
	}
}
