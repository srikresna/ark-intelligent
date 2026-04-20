package telegram

// Contextual Help & Standardized Error Messages

import (
	"fmt"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Contextual Help Definitions
// ---------------------------------------------------------------------------

// HelpTopic represents a help topic with explanation and examples.
type HelpTopic struct {
	Title       string
	Description string
	Examples    []string
	Related     []string
}

// helpTopics holds all contextual help definitions.
var helpTopics = map[string]HelpTopic{
	"gex": {
		Title:       "Gamma Exposure (GEX)",
		Description: "Mengukur posisi dealer options dan dampaknya ke pergerakan harga.\n\n" +
			"• <b>Positive GEX</b> = Dealer net long gamma → market range-bound, volatilitas rendah\n" +
			"• <b>Negative GEX</b> = Dealer net short gamma → market trending, volatilitas tinggi\n" +
			"• <b>GEX Flip</b> = Level pivot dimana GEX berubah dari positif ke negatif (atau sebaliknya)\n" +
			"• <b>Max Pain</b> = Harga dimana option holders mengalami kerugian terkecil saat expiry\n" +
			"• <b>Gamma Wall</b> = Strike dengan gamma tertinggi → bertindak sebagai magnet/resistance\n" +
			"• <b>Put Wall</b> = Level support terkuat dari konsentrasi put options",
		Examples: []string{
			"/gex BTC — Lihat GEX Bitcoin",
			"/gex ETH — Lihat GEX Ethereum",
			"/skew BTC — Lihat IV skew untuk BTC",
			"/ivol ETH — Lihat implied volatility surface",
		},
		Related: []string{"skew", "ivol", "vix"},
	},
	"skew": {
		Title:       "IV Skew / Smile Analysis",
		Description: "Menganalisis perbedaan Implied Volatility (IV) antara put dan call options.\n\n" +
			"• <b>Put Skew</b> (skew negatif) = Put IV lebih tinggi → fear/bearish sentiment\n" +
			"• <b>Call Skew</b> (skew positif) = Call IV lebih tinggi → bullish sentiment\n" +
			"• <b>Flat</b> = Put dan Call IV seimbang → neutral sentiment\n" +
			"• <b>Skew Flip</b> = Reversal dari put skew ke call skew (atau sebaliknya) → sinyal penting",
		Examples: []string{
			"/skew BTC — Analisis skew Bitcoin",
			"/skew ETH — Analisis skew Ethereum",
			"/gex BTC — Lihat gamma exposure",
			"/ivol BTC — Lihat IV surface",
		},
		Related: []string{"gex", "ivol", "vix"},
	},
	"ivol": {
		Title:       "Implied Volatility Surface",
		Description: "Menampilkan IV di berbagai strikes dan expiries untuk melihat market expectation.\n\n" +
			"• <b>ATM IV</b> = Implied Volatility at-the-money → baseline volatilitas\n" +
			"• <b>Term Structure</b> = IV vs waktu → backwardation (near-term tinggi) atau contango\n" +
			"• <b>Skew per Expiry</b> = Perbedaan IV put/call di setiap expiry\n" +
			"• <b>IV Signal</b> = Fear (IV tinggi) atau Greed (IV rendah)",
		Examples: []string{
			"/ivol BTC — IV surface Bitcoin",
			"/ivol ETH — IV surface Ethereum",
			"/gex BTC — Gamma exposure",
			"/skew BTC — Skew analysis",
		},
		Related: []string{"gex", "skew", "vix"},
	},
	"cot": {
		Title:       "Commitment of Traders (COT)",
		Description: "Data posisi institusional dari CFTC setiap Jumat.\n\n" +
			"• <b>Large Speculators</b> = Hedge funds, CTAs → biasanya contrarian\n" +
			"• <b>Commercials</b> = Hedgers (perusahaan) → usually right\n" +
			"• <b>Small Speculators</b> = Retail traders → biasanya wrong\n" +
			"• <b>Net Position</b> = Long - Short → arah positioning\n" +
			"• <b>Conviction Score</b> = Kekuatan sinyal berdasarkan konsistensi positioning",
		Examples: []string{
			"/cot EUR — COT Euro",
			"/cot GBP — COT Pound",
			"/cot JPY — COT Yen",
			"/cot GOLD — COT Emas",
		},
		Related: []string{"signal", "rank", "bias"},
	},
	"cta": {
		Title:       "Classical Technical Analysis",
		Description: "Analisis teknikal lengkap dengan multiple indicators.\n\n" +
			"• <b>Trend</b> = MA, EMA, ADX untuk identifikasi trend\n" +
			"• <b>Momentum</b> = RSI, MACD, Stochastic\n" +
			"• <b>Volatility</b> = Bollinger Bands, ATR\n" +
			"• <b>Volume</b> = OBV, Volume profile\n" +
			"• <b>Signal</b> = Buy/Sell berdasarkan konfluensi indicators",
		Examples: []string{
			"/cta EUR — CTA dashboard Euro",
			"/cta XAU 4h — CTA Emas timeframe 4H",
			"/ctabt EUR — Backtest CTA strategy",
			"/vp EUR — Volume Profile",
		},
		Related: []string{"quant", "vp", "ict", "smc"},
	},
	"quant": {
		Title:       "Quantitative / Econometric Analysis",
		Description: "Analisis statistik advanced untuk trading signals.\n\n" +
			"• <b>GARCH</b> = Volatility clustering & forecasting\n" +
			"• <b>Regime Detection</b> = HMM untuk identify market state\n" +
			"• <b>PCA</b> = Principal Component Analysis untuk factor extraction\n" +
			"• <b>Correlation</b> = Cross-asset correlation matrix\n" +
			"• <b>Backtest</b> = Strategy performance metrics",
		Examples: []string{
			"/quant EUR — Quant analysis Euro",
			"/quant XAU 4h — Quant Emas 4H",
			"/regime — Market regime dashboard",
			"/intermarket — Cross-asset signals",
		},
		Related: []string{"cta", "vp", "regime"},
	},
	"quantbt": {
		Title:       "Quantitative Backtest (ML-Enhanced)",
		Description: "Backtest strategi dengan machine learning dan analisis statistik advanced.\n\n" +
			"• <b>7 Timeframes</b> = 15m, 30m, 1h, 4h, 6h, 12h, daily\n" +
			"• <b>Grade Filter</b> = A (best), B, C (all trades)\n" +
			"• <b>Metrics</b> = Win rate, Sharpe, drawdown, profit factor\n" +
			"• <b>ML Models</b> = Random Forest, XGBoost, Neural Network\n" +
			"• <b>Chart</b> = Equity curve dengan drawdown visualization\n" +
			"• <b>Trade Details</b> = Entry/exit/PnL setiap trade",
		Examples: []string{
			"/quantbt — Menu symbol selection",
			"/quantbt EUR — Backtest Euro",
			"/quantbt XAU 4h — Backtest Gold 4H",
			"/quantbt BTC 1h A — Backtest Bitcoin 1H Grade A",
		},
		Related: []string{"ctabt", "cta", "backtest", "quant"},
	},
	"vix": {
		Title:       "Volatility Index (VIX) & Volatility Suite",
		Description: "Dashboard untuk mengukur fear & greed market.\n\n" +
			"• <b>VIX</b> = Fear index S&P 500 → >20 = fear, <15 = complacency\n" +
			"• <b>VVIX</b> = Volatility of VIX → sentiment changes\n" +
			"• <b>VXN</b> = Volatility Nasdaq 100\n" +
			"• <b>VXRT</b> = Volatility Russell 2000\n" +
			"• <b>Term Structure</b> = VIX futures curve → contango/backwardation",
		Examples: []string{
			"/vix — VIX dashboard",
			"/sentiment — Sentiment surveys",
			"/risk — Risk metrics",
		},
		Related: []string{"gex", "skew", "sentiment"},
	},
	"outlook": {
		Title:       "AI Unified Outlook",
		Description: "Analisis AI yang menggabungkan semua data sources + web search.\n\n" +
			"• <b>Data Integration</b> = COT, price, news, macro data\n" +
			"• <b>Web Search</b> = Real-time news & sentiment\n" +
			"• <b>Synthesis</b> = AI reasoning untuk directional bias\n" +
			"• <b>Conviction</b> = Confidence level berdasarkan data quality",
		Examples: []string{
			"/outlook — AI outlook global",
			"/outlook EUR — AI outlook Euro",
			"/macro — Macro regime",
			"/impact NFP — Impact NFP event",
		},
		Related: []string{"macro", "impact", "sentiment"},
	},
	"calendar": {
		Title:       "Economic Calendar",
		Description: "Jadwal rilis data ekonomi penting.\n\n" +
			"• <b>High Impact</b> = NFP, CPI, FOMC, Rate decisions\n" +
			"• <b>Medium Impact</b> = GDP, PMI, Retail sales\n" +
			"• <b>Filters</b> = Bisa filter by impact level & currency\n" +
			"• <b>Alerts</b> = Set alert untuk high impact events",
		Examples: []string{
			"/calendar — Kalender minggu ini",
			"/calendar week — Kalender 7 hari",
			"/calendar EUR — Event untuk Euro",
			"/impact NFP — Impact NFP",
		},
		Related: []string{"impact", "outlook", "macro"},
	},
	"price": {
		Title:       "Daily Price Data",
		Description: "Harga terkini dan perubahan.\n\n" +
			"• <b>OHLC</b> = Open, High, Low, Close\n" +
			"• <b>Change</b> = Perubahan harian (%) dan absolute\n" +
			"• <b>Range</b> = High-Low range\n" +
			"• <b>Trend</b> = 1D, 1W, 1M trend direction",
		Examples: []string{
			"/price EUR — Harga Euro",
			"/price XAU — Harga Emas",
			"/price BTC — Harga Bitcoin",
			"/levels EUR — Support/Resistance",
		},
		Related: []string{"levels", "cot", "cta"},
	},
}

// getContextualHelp returns formatted HTML help for a given topic.
func getContextualHelp(topic string) string {
	t, ok := helpTopics[topic]
	if !ok {
		return fmt.Sprintf("❓ <b>%s</b>\n\n<i>Help untuk command ini belum tersedia.</i>", topic)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("❓ <b>%s</b>\n\n", t.Title))
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n\n", t.Description))

	if len(t.Examples) > 0 {
		sb.WriteString("<b>📝 Contoh:</b>\n")
		for _, ex := range t.Examples {
			sb.WriteString(fmt.Sprintf("• %s\n", ex))
		}
		sb.WriteString("\n")
	}

	if len(t.Related) > 0 {
		sb.WriteString("<b>🔗 Terkait:</b>\n")
		for _, rel := range t.Related {
			sb.WriteString(fmt.Sprintf("• /%s\n", rel))
		}
	}

	return sb.String()
}

// getHelpKeyboard returns inline keyboard with "Got it" and "Try Example" buttons.
func getHelpKeyboard(topic string) ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "✅ Got it", CallbackData: "help:close"},
				{Text: "📝 Try Example", CallbackData: fmt.Sprintf("help:try:%s", topic)},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Standardized Error Messages
// ---------------------------------------------------------------------------

// StandardError represents a structured error with user-friendly message.
type StandardError struct {
	Title   string
	Message string
	Tips    []string
	Action  string
}

// Error implements error interface.
func (e StandardError) Error() string {
	return e.Message
}

// FormatError creates a standardized error message.
func FormatError(err error, command string) string {
	// Parse error and create user-friendly message
	var title, message string
	var tips []string

	// Categorize error
	errMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errMsg, "timeout"):
		title = "⏱️ Request Timeout"
		message = fmt.Sprintf("Request untuk <b>%s</b> timeout setelah 30 detik.", command)
		tips = []string{
			"Tap 🔄 untuk retry",
			"Data source mungkin sedang lambat",
			"Coba lagi dalam 1-2 menit",
		}
	case strings.Contains(errMsg, "unavailable") || strings.Contains(errMsg, "not found"):
		title = "🚫 Data Tidak Tersedia"
		message = fmt.Sprintf("Data untuk <b>%s</b> saat ini tidak tersedia.", command)
		tips = []string{
			"Tap 🔄 untuk retry",
			"Data mungkin belum dirilis",
			"Coba command alternatif",
		}
	case strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "not supported"):
		title = "⚠️ Parameter Invalid"
		message = "Parameter yang kamu masukkan tidak valid."
		tips = []string{
			"Gunakan format: /command SYMBOL",
			"Contoh: /gex BTC, /cot EUR",
			"Ketik /help untuk panduan",
		}
	case strings.Contains(errMsg, "rate limit"):
		title = "🐌 Rate Limit"
		message = "Terlalu banyak request. Tolong tunggu sebentar."
		tips = []string{
			"Tunggu 30-60 detik",
			"Tap 🔄 untuk retry",
			"Gunakan command lain dulu",
		}
	case strings.Contains(errMsg, "network") || strings.Contains(errMsg, "connection"):
		title = "🌐 Network Error"
		message = "Gagal terhubung ke data source."
		tips = []string{
			"Periksa koneksi internet",
			"Tap 🔄 untuk retry",
			"Server mungkin maintenance",
		}
	default:
		title = "⚠️ Error"
		message = fmt.Sprintf("Terjadi kesalahan saat menjalankan <b>%s</b>.", command)
		tips = []string{
			"Tap 🔄 untuk retry",
			"Jika error berlanjut, laporkan ke admin",
			"Coba command lain dulu",
		}
	}

	// Build formatted message
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s <b>%s</b>\n\n", getEmojiForTitle(title), title))
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n\n", message))

	if len(tips) > 0 {
		sb.WriteString("<b>💡 Tips:</b>\n")
		for _, tip := range tips {
			sb.WriteString(fmt.Sprintf("• %s\n", tip))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("<i>Error ID: ")
	sb.WriteString(generateErrorID(command))
	sb.WriteString("</i>")

	return sb.String()
}

// getEmojiForTitle returns appropriate emoji for error title.
func getEmojiForTitle(title string) string {
	switch title {
	case "⏱️ Request Timeout":
		return "⏱️"
	case "🚫 Data Tidak Tersedia":
		return "🚫"
	case "⚠️ Parameter Invalid":
		return "⚠️"
	case "🐌 Rate Limit":
		return "🐌"
	case "🌐 Network Error":
		return "🌐"
	default:
		return "⚠️"
	}
}

// generateErrorID creates a short error ID for tracking.
func generateErrorID(command string) string {
	// Simple hash-based ID (not cryptographically secure, just for tracking)
	return fmt.Sprintf("ERR-%s-%d", strings.ToUpper(command), time.Now().Unix()%10000)
}

// CreateErrorKeyboard returns keyboard for error messages.
func CreateErrorKeyboard(command string) ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "🔄 Retry", CallbackData: fmt.Sprintf("err:retry:%s", command)},
				{Text: "📚 Help", CallbackData: fmt.Sprintf("help:%s", command)},
			},
			{
				{Text: "🏠 Home", CallbackData: "nav:home"},
			},
		},
	}
}
