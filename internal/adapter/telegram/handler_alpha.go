package telegram

// handler_alpha.go — Handlers for new factor/strategy/microstructure commands:
//   /alpha       — unified dashboard with inline keyboard navigation
//   /xfactors    — cross-sectional factor ranking
//   /playbook    — strategy playbook (top long/short + macro context)
//   /heat        — portfolio exposure heat
//   /rankx       — compact rank leaderboard
//   /transition  — regime transition warning
//   /cryptoalpha — Bybit microstructure confirmation for top crypto signals

import (
	"github.com/arkcode369/ark-intelligent/internal/config"
	"context"
	"fmt"
	"html"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/factors"
	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/defillama"
	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/cryptocompare"
	"github.com/arkcode369/ark-intelligent/internal/service/microstructure"
	"github.com/arkcode369/ark-intelligent/internal/service/strategy"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// ---------------------------------------------------------------------------
// AlphaServices — optional new engine dependencies
// ---------------------------------------------------------------------------

// AlphaServices holds optional services for the new alpha commands.
// All fields may be nil — commands degrade gracefully.
type AlphaServices struct {
	FactorEngine   *factors.Engine
	StrategyEngine *strategy.Engine
	MicroEngine    *microstructure.Engine
	ProfileBuilder AssetProfileBuilder
}

// AssetProfileBuilder builds AssetProfile slices from available repository data.
type AssetProfileBuilder interface {
	BuildProfiles(ctx context.Context) ([]factors.AssetProfile, error)
	GetMacroRegime(ctx context.Context) string
	GetCOTBias(ctx context.Context) map[string]string
	GetVolRegime(ctx context.Context) map[string]string
	GetCarryBps(ctx context.Context) map[string]float64
	GetTransitionProb(ctx context.Context) (prob float64, from, to string)
}

// ---------------------------------------------------------------------------
// alphaState — cached computation results for the unified /alpha dashboard
// ---------------------------------------------------------------------------

// alphaState caches all computed data for the unified alpha dashboard.
// It is computed once in cmdAlpha and reused by callback navigations.
type alphaState struct {
	ranking    *factors.RankingResult
	playbook   *strategy.PlaybookResult
	crypto     map[string]*microstructure.Signal
	cryptoSyms []string
	computedAt time.Time
}

var alphaStateTTL = config.AlphaStateTTL

// alphaStateCache stores per-chat alpha state with TTL.
type alphaStateCache struct {
	mu    sync.Mutex
	store map[string]*alphaState // chatID -> state
}

func newAlphaStateCache() *alphaStateCache {
	return &alphaStateCache{
		store: make(map[string]*alphaState),
	}
}

func (c *alphaStateCache) get(chatID string) *alphaState {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.store[chatID]
	if !ok || time.Since(s.computedAt) > alphaStateTTL {
		return nil
	}
	return s
}

func (c *alphaStateCache) set(chatID string, s *alphaState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Opportunistic cleanup
	if len(c.store) > 50 {
		now := time.Now()
		for k, v := range c.store {
			if now.Sub(v.computedAt) > alphaStateTTL*2 {
				delete(c.store, k)
			}
		}
	}
	c.store[chatID] = s
}

// WithAlpha injects AlphaServices into the handler and registers alpha commands.
func (h *Handler) WithAlpha(a *AlphaServices) *Handler {
	h.alpha = a
	if a != nil {
		h.alphaCache = newAlphaStateCache()
		h.registerAlphaCommands()
	}
	return h
}

// registerAlphaCommands wires the new commands into the bot.
func (h *Handler) registerAlphaCommands() {
	// Unified alpha dashboard
	h.bot.RegisterCommand("/alpha", h.cmdAlpha)
	h.bot.RegisterCallback("alpha:", h.handleAlphaCallback)

	// Legacy individual commands (backward compatible)
	h.bot.RegisterCommand("/xfactors", h.cmdXFactors)
	h.bot.RegisterCommand("/playbook", h.cmdPlaybook)
	h.bot.RegisterCommand("/heat", h.cmdHeat)
	h.bot.RegisterCommand("/rankx", h.cmdRankX)
	h.bot.RegisterCommand("/transition", h.cmdTransition)
	h.bot.RegisterCommand("/cryptoalpha", h.cmdCryptoAlpha)
}

// ---------------------------------------------------------------------------
// /alpha — Unified Alpha Engine Dashboard
// ---------------------------------------------------------------------------

// computeAlphaState gathers all alpha engine data at once.
func (h *Handler) computeAlphaState(ctx context.Context) (*alphaState, error) {
	if h.alpha == nil || h.alpha.FactorEngine == nil || h.alpha.ProfileBuilder == nil {
		return nil, fmt.Errorf("alpha engine not configured")
	}

	profiles, err := h.alpha.ProfileBuilder.BuildProfiles(ctx)
	if err != nil || len(profiles) == 0 {
		return nil, fmt.Errorf("could not build asset profiles: %s", alphaErr(err))
	}

	ranking := h.alpha.FactorEngine.Rank(profiles)

	// Build strategy input
	tProb, tFrom, tTo := h.alpha.ProfileBuilder.GetTransitionProb(ctx)
	in := strategy.Input{
		Ranking:        ranking,
		MacroRegime:    h.alpha.ProfileBuilder.GetMacroRegime(ctx),
		COTBias:        h.alpha.ProfileBuilder.GetCOTBias(ctx),
		VolRegime:      h.alpha.ProfileBuilder.GetVolRegime(ctx),
		CarryBps:       h.alpha.ProfileBuilder.GetCarryBps(ctx),
		TransitionProb: tProb,
		TransitionFrom: tFrom,
		TransitionTo:   tTo,
	}

	var playbook *strategy.PlaybookResult
	if h.alpha.StrategyEngine != nil {
		playbook = h.alpha.StrategyEngine.Generate(in)
	}

	// Crypto microstructure (optional)
	cryptoSyms := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"}
	var crypto map[string]*microstructure.Signal
	if h.alpha.MicroEngine != nil {
		crypto, _ = h.alpha.MicroEngine.AnalyzeMultiple(ctx, "linear", cryptoSyms)
	}

	return &alphaState{
		ranking:    ranking,
		playbook:   playbook,
		crypto:     crypto,
		cryptoSyms: cryptoSyms,
		computedAt: time.Now(),
	}, nil
}

func (h *Handler) cmdAlpha(ctx context.Context, chatID string, _ int64, _ string) error {
	if h.alpha == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Alpha Engine not configured.")
		return err
	}

	loadID, _ := h.bot.SendLoading(ctx, chatID, "⚡ Menghitung Alpha Engine... ⏳")

	state, err := h.computeAlphaState(ctx)
	if err != nil {
		if loadID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadID)
		}
		h.sendUserError(ctx, chatID, err, "alpha")
		return nil
	}

	if loadID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadID)
	}

	h.alphaCache.set(chatID, state)

	summary := formatAlphaSummary(state)
	kb := h.kb.AlphaMenu()
	kb = AppendFeedbackRow(kb, h.kb, "fb:alpha:summary", h.feedbackEnabled())
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, summary, kb)
	return err
}

// handleAlphaCallback handles "alpha:" prefixed callbacks for the unified dashboard.
func (h *Handler) handleAlphaCallback(ctx context.Context, chatID string, msgID int, _ int64, data string) error {
	action := strings.TrimPrefix(data, "alpha:")

	// Get or recompute state
	state := h.alphaCache.get(chatID)
	if state == nil {
		// State expired — recompute
		var err error
		state, err = h.computeAlphaState(ctx)
		if err != nil {
			h.editUserError(ctx, chatID, msgID, err, "alpha")
			return nil
		}
		h.alphaCache.set(chatID, state)
	}

	switch {
	case action == "back":
		// Back to summary — use cached state (no recompute)
		summary := formatAlphaSummary(state)
		kb := h.kb.AlphaMenu()
		return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, summary, kb)

	case action == "refresh":
		// Force recompute
		newState, err := h.computeAlphaState(ctx)
		if err != nil {
			h.editUserError(ctx, chatID, msgID, err, "alpha")
			return nil
		}
		h.alphaCache.set(chatID, newState)
		summary := formatAlphaSummary(newState)
		kb := h.kb.AlphaMenu()
		return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, summary, kb)

	case action == "factors":
		txt := alphaExplainHeader("📊 Factor Ranking",
			"Peringkat berdasarkan momentum harga, kualitas tren, carry, dan volatilitas. Skor positif = bullish, negatif = bearish.")
		txt += formatFactorRanking(state.ranking)
		kb := h.kb.AlphaDetailMenu()
		return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, txt, kb)

	case action == "playbook":
		txt := alphaExplainHeader("🎯 Strategy Playbook",
			"Rekomendasi trading berdasarkan analisis multifaktor. Conviction menunjukkan tingkat keyakinan.")
		if state.playbook != nil {
			txt += formatPlaybook(state.playbook)
		} else {
			txt += "⚠️ No playbook data."
		}
		kb := h.kb.AlphaDetailMenu()
		return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, txt, kb)

	case action == "heat":
		txt := alphaExplainHeader("🌡️ Portfolio Heat",
			"Mengukur total eksposur portfolio. COLD = aman untuk tambah posisi, OVERHEAT = kurangi posisi.")
		if state.playbook != nil {
			txt += formatHeat(state.playbook.Heat)
		} else {
			txt += "⚠️ No heat data."
		}
		kb := h.kb.AlphaDetailMenu()
		return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, txt, kb)

	case action == "rankx":
		txt := alphaExplainHeader("📈 RankX Leaderboard",
			"Ranking ringkas — atas = kandidat long, bawah = kandidat short.")
		txt += formatRankX(state.ranking)
		kb := h.kb.AlphaDetailMenu()
		return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, txt, kb)

	case action == "transition":
		txt := alphaExplainHeader("🔄 Regime & Transisi",
			"Monitor perubahan fase ekonomi makro. Transisi aktif = kurangi ukuran posisi.")
		if state.playbook != nil {
			txt += formatTransition(state.playbook.Transition, state.playbook.MacroRegime)
		} else {
			// Fallback: compute directly
			macroRegime := ""
			if h.alpha.ProfileBuilder != nil {
				macroRegime = h.alpha.ProfileBuilder.GetMacroRegime(ctx)
			}
			tProb, tFrom, tTo := h.alpha.ProfileBuilder.GetTransitionProb(ctx)
			tw := strategy.TransitionWarning{
				IsActive:    tProb > 0.50,
				FromRegime:  tFrom,
				ToRegime:    tTo,
				Probability: tProb,
				DetectedAt:  time.Now(),
			}
			txt += formatTransition(tw, macroRegime)
		}
		kb := h.kb.AlphaDetailMenu()
		return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, txt, kb)

	case action == "crypto":
		txt := alphaExplainHeader("⚡ Crypto Microstructure Alpha",
			"Analisis microstructure dari orderbook dan funding rate. Konfirmasi = sinyal searah dengan flow.")
		if len(state.crypto) > 0 {
			txt += formatCryptoAlpha(state.crypto, state.cryptoSyms, nil)
		} else {
			txt += "⚠️ No microstructure data available."
		}
		kb := h.kb.AlphaCryptoDetailMenu()
		return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, txt, kb)

	default:
		// Handle alpha:crypto:BTC etc.
		if strings.HasPrefix(action, "crypto:") {
			sym := strings.TrimPrefix(action, "crypto:")
			sym = strings.ToUpper(sym)
			if !strings.HasSuffix(sym, "USDT") {
				sym += "USDT"
			}
			txt := alphaExplainHeader("⚡ Crypto: "+strings.TrimSuffix(sym, "USDT"),
				"Analisis microstructure dari orderbook dan funding rate. Konfirmasi = sinyal searah dengan flow.")
			// Try from cache first
			if sig, ok := state.crypto[sym]; ok {
				txt += formatCryptoAlpha(map[string]*microstructure.Signal{sym: sig}, []string{sym}, nil)
			} else if h.alpha.MicroEngine != nil {
				// Fetch single symbol
				results, _ := h.alpha.MicroEngine.AnalyzeMultiple(ctx, "linear", []string{sym})
				txt += formatCryptoAlpha(results, []string{sym}, nil)
			} else {
				txt += "⚠️ No data for " + sym
			}
			kb := h.kb.AlphaCryptoDetailMenu()
			return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, txt, kb)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// formatAlphaSummary — beginner-friendly decision summary in Indonesian
// ---------------------------------------------------------------------------

func formatAlphaSummary(state *alphaState) string {
	var sb strings.Builder

	sb.WriteString("<b>⚡ Alpha Engine Dashboard</b>\n")
	sb.WriteString(fmt.Sprintf("📅 <i>%s UTC</i>\n\n", state.computedAt.UTC().Format("02 Jan 2006 15:04")))

	// Regime & stability assessment
	if state.playbook != nil && state.playbook.MacroRegime != "" {
		regime := state.playbook.MacroRegime
		tw := state.playbook.Transition
		stabilityText := "stabil"
		probPct := tw.Probability * 100
		if tw.IsActive {
			stabilityText = fmt.Sprintf("⚠️ transisi aktif ke %s", html.EscapeString(tw.ToRegime))
		} else if tw.Probability > 0.30 {
			stabilityText = "mulai goyah"
		}
		sb.WriteString(fmt.Sprintf("🧭 <b>KEPUTUSAN UTAMA:</b>\nRegime saat ini: <b>%s</b> (%s, probabilitas transisi %.0f%%)\n\n",
			html.EscapeString(regime), stabilityText, probPct))
	}

	// Top recommendations
	if state.playbook != nil && len(state.playbook.Playbook) > 0 {
		longs := state.playbook.TopLong(3)
		shorts := state.playbook.TopShort(3)
		combined := append(longs, shorts...)

		// Sort by conviction descending
		sort.Slice(combined, func(i, j int) bool {
			return combined[i].Conviction > combined[j].Conviction
		})

		// Take top 3 overall
		if len(combined) > 3 {
			combined = combined[:3]
		}

		if len(combined) > 0 {
			sb.WriteString("✅ <b>Rekomendasi:</b>\n")
			for _, e := range combined {
				dir := "LONG"
				if e.Direction == strategy.DirectionShort {
					dir = "SHORT"
				}

				// Build reason string
				reasons := buildReasonIndonesian(e)
				convEmoji := alphaConvEmoji(e.ConvLevel)

				sb.WriteString(fmt.Sprintf("• %s %s — %s (conviction %s %s)\n",
					dir, html.EscapeString(e.Currency),
					reasons,
					string(e.ConvLevel), convEmoji))
			}
			sb.WriteString("\n")
		}
	}

	// Warnings
	var warnings []string

	// Portfolio heat
	if state.playbook != nil {
		heat := state.playbook.Heat
		heatEmoji := alphaHeatEmoji(heat.HeatLevel)
		var heatAdvice string
		switch heat.HeatLevel {
		case strategy.HeatCold:
			heatAdvice = "aman untuk tambah posisi baru"
		case strategy.HeatWarm:
			heatAdvice = "masih aman tapi jangan terlalu agresif"
		case strategy.HeatHot:
			heatAdvice = "hati-hati, kurangi agresivitas"
		case strategy.HeatOverheat:
			heatAdvice = "KURANGI POSISI segera!"
		default:
			heatAdvice = "evaluasi eksposur"
		}
		warnings = append(warnings, fmt.Sprintf("Portfolio heat: %s %s — %s",
			string(heat.HeatLevel), heatEmoji, heatAdvice))

		// Transition warning
		if state.playbook.Transition.IsActive {
			warnings = append(warnings, fmt.Sprintf("Regime transition aktif: %s → %s (%.0f%%) — kurangi ukuran posisi",
				html.EscapeString(state.playbook.Transition.FromRegime),
				html.EscapeString(state.playbook.Transition.ToRegime),
				state.playbook.Transition.Probability*100))
		}
	}

	// Notable crypto signals
	if len(state.crypto) > 0 {
		for _, sym := range state.cryptoSyms {
			sig, ok := state.crypto[sym]
			if !ok {
				continue
			}
			displaySym := strings.TrimSuffix(sym, "USDT")
			if sig.Bias != microstructure.BiasNeutral {
				note := fmt.Sprintf("%s microstructure: %s", displaySym, strings.ToLower(string(sig.Bias)))
				if sig.FundingRate > 0.01 {
					note += " tapi funding rate tinggi"
				}
				if sig.ConfirmEntry {
					note += " ✅"
				}
				warnings = append(warnings, note)
			}
		}
	}

	if len(warnings) > 0 {
		sb.WriteString("⚠️ <b>Waspadai:</b>\n")
		for _, w := range warnings {
			sb.WriteString(fmt.Sprintf("• %s\n", w))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Detail lengkap 👇")
	return sb.String()
}

// buildReasonIndonesian generates a simple Indonesian explanation of why a trade is recommended.
func buildReasonIndonesian(e strategy.PlaybookEntry) string {
	var parts []string

	// Factor score interpretation
	if e.FactorScore > 0.30 {
		parts = append(parts, "momentum kuat")
	} else if e.FactorScore > 0.10 {
		parts = append(parts, "momentum positif")
	} else if e.FactorScore < -0.30 {
		parts = append(parts, "momentum lemah")
	} else if e.FactorScore < -0.10 {
		parts = append(parts, "momentum negatif")
	}

	// COT bias
	switch e.COTBias {
	case "BULLISH":
		parts = append(parts, "COT bullish")
	case "BEARISH":
		parts = append(parts, "COT bearish")
	}

	// Carry
	if e.RateDiffBps > 50 {
		parts = append(parts, "carry positif")
	} else if e.RateDiffBps < -50 {
		parts = append(parts, "carry negatif")
	}

	// Regime fit
	if e.RegimeFit == "ALIGNED" {
		parts = append(parts, "regime mendukung")
	} else if e.RegimeFit == "AGAINST_REGIME" {
		parts = append(parts, "melawan regime")
	}

	if len(parts) == 0 {
		parts = append(parts, "sinyal multifaktor")
	}

	return strings.Join(parts, " + ")
}

// alphaExplainHeader adds an Indonesian explanation header to detail views.
func alphaExplainHeader(title, explanation string) string {
	return fmt.Sprintf("<b>%s</b>\n<i>ℹ️ %s</i>\n\n", title, explanation)
}

// ---------------------------------------------------------------------------
// /factors — cross-sectional factor ranking
// ---------------------------------------------------------------------------

func (h *Handler) cmdXFactors(ctx context.Context, chatID string, _ int64, _ string) error {
	if h.alpha == nil || h.alpha.FactorEngine == nil || h.alpha.ProfileBuilder == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Factor Engine not configured.")
		return err
	}

	profiles, err := h.alpha.ProfileBuilder.BuildProfiles(ctx)
	if err != nil || len(profiles) == 0 {
		h.sendUserError(ctx, chatID, err, "alpha")
		return nil
	}

	result := h.alpha.FactorEngine.Rank(profiles)
	_, err = h.bot.SendHTML(ctx, chatID, formatFactorRanking(result))
	return err
}

// ---------------------------------------------------------------------------
// /playbook — strategy playbook
// ---------------------------------------------------------------------------

func (h *Handler) cmdPlaybook(ctx context.Context, chatID string, _ int64, _ string) error {
	if h.alpha == nil || h.alpha.StrategyEngine == nil || h.alpha.ProfileBuilder == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Strategy Engine not configured.")
		return err
	}

	profiles, err := h.alpha.ProfileBuilder.BuildProfiles(ctx)
	if err != nil || len(profiles) == 0 {
		h.sendUserError(ctx, chatID, err, "alpha")
		return nil
	}

	ranking := h.alpha.FactorEngine.Rank(profiles)
	tProb, tFrom, tTo := h.alpha.ProfileBuilder.GetTransitionProb(ctx)

	in := strategy.Input{
		Ranking:        ranking,
		MacroRegime:    h.alpha.ProfileBuilder.GetMacroRegime(ctx),
		COTBias:        h.alpha.ProfileBuilder.GetCOTBias(ctx),
		VolRegime:      h.alpha.ProfileBuilder.GetVolRegime(ctx),
		CarryBps:       h.alpha.ProfileBuilder.GetCarryBps(ctx),
		TransitionProb: tProb,
		TransitionFrom: tFrom,
		TransitionTo:   tTo,
	}
	result := h.alpha.StrategyEngine.Generate(in)
	_, err = h.bot.SendHTML(ctx, chatID, formatPlaybook(result))
	return err
}

// ---------------------------------------------------------------------------
// /heat — portfolio heat
// ---------------------------------------------------------------------------

func (h *Handler) cmdHeat(ctx context.Context, chatID string, _ int64, _ string) error {
	if h.alpha == nil || h.alpha.StrategyEngine == nil || h.alpha.ProfileBuilder == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Strategy Engine not configured.")
		return err
	}

	profiles, err := h.alpha.ProfileBuilder.BuildProfiles(ctx)
	if err != nil || len(profiles) == 0 {
		h.sendUserError(ctx, chatID, err, "alpha")
		return nil
	}
	ranking := h.alpha.FactorEngine.Rank(profiles)
	tProb, tFrom, tTo := h.alpha.ProfileBuilder.GetTransitionProb(ctx)
	result := h.alpha.StrategyEngine.Generate(strategy.Input{
		Ranking:        ranking,
		MacroRegime:    h.alpha.ProfileBuilder.GetMacroRegime(ctx),
		COTBias:        h.alpha.ProfileBuilder.GetCOTBias(ctx),
		VolRegime:      h.alpha.ProfileBuilder.GetVolRegime(ctx),
		CarryBps:       h.alpha.ProfileBuilder.GetCarryBps(ctx),
		TransitionProb: tProb,
		TransitionFrom: tFrom,
		TransitionTo:   tTo,
	})
	_, err = h.bot.SendHTML(ctx, chatID, formatHeat(result.Heat))
	return err
}

// ---------------------------------------------------------------------------
// /rankx — compact rank leaderboard
// ---------------------------------------------------------------------------

func (h *Handler) cmdRankX(ctx context.Context, chatID string, _ int64, _ string) error {
	if h.alpha == nil || h.alpha.FactorEngine == nil || h.alpha.ProfileBuilder == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Factor Engine not configured.")
		return err
	}

	profiles, err := h.alpha.ProfileBuilder.BuildProfiles(ctx)
	if err != nil || len(profiles) == 0 {
		h.sendUserError(ctx, chatID, err, "alpha")
		return nil
	}
	result := h.alpha.FactorEngine.Rank(profiles)
	_, err = h.bot.SendHTML(ctx, chatID, formatRankX(result))
	return err
}

// ---------------------------------------------------------------------------
// /transition — regime transition warning
// ---------------------------------------------------------------------------

func (h *Handler) cmdTransition(ctx context.Context, chatID string, _ int64, _ string) error {
	if h.alpha == nil || h.alpha.ProfileBuilder == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Strategy Engine not configured.")
		return err
	}

	macroRegime := h.alpha.ProfileBuilder.GetMacroRegime(ctx)
	tProb, tFrom, tTo := h.alpha.ProfileBuilder.GetTransitionProb(ctx)

	// If we have a strategy engine, compute full playbook to get AffectedAssets
	if h.alpha.StrategyEngine != nil && h.alpha.FactorEngine != nil {
		profiles, err := h.alpha.ProfileBuilder.BuildProfiles(ctx)
		if err == nil && len(profiles) > 0 {
			ranking := h.alpha.FactorEngine.Rank(profiles)
			result := h.alpha.StrategyEngine.Generate(strategy.Input{
				Ranking:        ranking,
				MacroRegime:    macroRegime,
				COTBias:        h.alpha.ProfileBuilder.GetCOTBias(ctx),
				VolRegime:      h.alpha.ProfileBuilder.GetVolRegime(ctx),
				CarryBps:       h.alpha.ProfileBuilder.GetCarryBps(ctx),
				TransitionProb: tProb,
				TransitionFrom: tFrom,
				TransitionTo:   tTo,
			})
			_, _ = h.bot.SendHTML(ctx, chatID, formatTransition(result.Transition, macroRegime))
			return nil
		}
	}

	// Fallback: minimal transition info
	tw := strategy.TransitionWarning{
		IsActive:    tProb > 0.50,
		FromRegime:  tFrom,
		ToRegime:    tTo,
		Probability: tProb,
		DetectedAt:  time.Now(),
	}
	_, err := h.bot.SendHTML(ctx, chatID, formatTransition(tw, macroRegime))
	return err
}

// ---------------------------------------------------------------------------
// /cryptoalpha [SYMBOL] — Bybit microstructure
// ---------------------------------------------------------------------------

func (h *Handler) cmdCryptoAlpha(ctx context.Context, chatID string, _ int64, args string) error {
	if h.alpha == nil || h.alpha.MicroEngine == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Microstructure Engine not configured (Bybit API required).")
		return err
	}

	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"}
	if arg := strings.TrimSpace(args); arg != "" {
		custom := strings.ToUpper(arg)
		if !strings.HasSuffix(custom, "USDT") {
			custom += "USDT"
		}
		symbols = []string{custom}
	}

	results, _ := h.alpha.MicroEngine.AnalyzeMultiple(ctx, "linear", symbols)
	tvl := defillama.GetCachedOrFetch(ctx)
	exVol := cryptocompare.GetCachedOrFetch(ctx)
	out := formatCryptoAlpha(results, symbols, tvl)
	out += cryptocompare.FormatExchangeVolumeSection(exVol)
	_, err := h.bot.SendHTML(ctx, chatID, out)
	return err
}

// ---------------------------------------------------------------------------
// Formatters
// ---------------------------------------------------------------------------

func formatFactorRanking(result *factors.RankingResult) string {
	if result == nil || len(result.Assets) == 0 {
		return "⚠️ Tidak ada data faktor."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>📊 Factor Ranking</b> — %d aset\n", result.AssetCount))
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n\n", fmtutil.FormatDateTimeUTC(result.ComputedAt)))

	for _, a := range result.Assets {
		emoji := alphaSignalEmoji(string(a.Signal))
		bar := alphaScoreBar(a.CompositeScore)
		sb.WriteString(fmt.Sprintf("%s <b>#%d %s</b> %s (%.2f)\n",
			emoji, a.Rank, html.EscapeString(a.Currency), bar, a.CompositeScore))
		sb.WriteString(fmt.Sprintf("   Mom:%.2f TQ:%.2f CA:%.2f LV:%.2f\n",
			a.Scores.Momentum, a.Scores.TrendQuality,
			a.Scores.CarryAdjusted, a.Scores.LowVol))
		// Indonesian interpretation
		interpret := factorInterpretIndonesian(a)
		if interpret != "" {
			sb.WriteString(fmt.Sprintf("   → <i>%s</i>\n", interpret))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// factorInterpretIndonesian gives a brief Indonesian explanation for one ranked asset.
func factorInterpretIndonesian(a factors.RankedAsset) string {
	switch a.Signal {
	case factors.SignalStrongLong:
		return "Sinyal beli kuat — momentum dan tren sangat positif"
	case factors.SignalLong:
		return "Sinyal beli — momentum cukup positif"
	case factors.SignalStrongShort:
		return "Sinyal jual kuat — momentum dan tren sangat negatif"
	case factors.SignalShort:
		return "Sinyal jual — momentum cukup negatif"
	default:
		return "Netral — belum ada arah yang jelas"
	}
}

func formatPlaybook(result *strategy.PlaybookResult) string {
	if result == nil {
		return "⚠️ Tidak ada data playbook."
	}
	var sb strings.Builder
	sb.WriteString("<b>🎯 Strategy Playbook</b>\n")
	if result.MacroRegime != "" {
		regimeDesc := regimeIndonesian(result.MacroRegime)
		sb.WriteString(fmt.Sprintf("Regime: <b>%s</b> — <i>%s</i>\n",
			html.EscapeString(result.MacroRegime), regimeDesc))
	}
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n\n", fmtutil.FormatDateTimeUTC(result.ComputedAt)))

	if result.Transition.IsActive {
		sb.WriteString(fmt.Sprintf("⚠️ <b>TRANSISI:</b> %s → %s (%.0f%% prob)\n",
			html.EscapeString(result.Transition.FromRegime),
			html.EscapeString(result.Transition.ToRegime),
			result.Transition.Probability*100))
		sb.WriteString("<i>Kurangi ukuran posisi saat transisi aktif</i>\n\n")
	}

	longs := result.TopLong(5)
	shorts := result.TopShort(5)

	if len(longs) > 0 {
		sb.WriteString("<b>🟢 IDE LONG (Beli):</b>\n")
		for _, e := range longs {
			convBar := alphaConvBar(e.Conviction)
			fit := ""
			if e.RegimeFit == "ALIGNED" {
				fit = " ✓ regime"
			} else if e.RegimeFit == "AGAINST_REGIME" {
				fit = " ✗ anti-regime"
			}
			carry := ""
			if e.RateDiffBps != 0 {
				carry = fmt.Sprintf(" carry:%+.0fbps", e.RateDiffBps)
			}
			reason := buildReasonIndonesian(e)
			sb.WriteString(fmt.Sprintf("  %s <b>%s</b>%s%s %s\n",
				alphaConvEmoji(e.ConvLevel), html.EscapeString(e.Currency), fit, carry, convBar))
			sb.WriteString(fmt.Sprintf("     <i>%s</i>\n", reason))
		}
		sb.WriteString("\n")
	}

	if len(shorts) > 0 {
		sb.WriteString("<b>🔴 IDE SHORT (Jual):</b>\n")
		for _, e := range shorts {
			convBar := alphaConvBar(e.Conviction)
			carry := ""
			if e.RateDiffBps != 0 {
				carry = fmt.Sprintf(" carry:%+.0fbps", e.RateDiffBps)
			}
			reason := buildReasonIndonesian(e)
			sb.WriteString(fmt.Sprintf("  %s <b>%s</b>%s %s\n",
				alphaConvEmoji(e.ConvLevel), html.EscapeString(e.Currency), carry, convBar))
			sb.WriteString(fmt.Sprintf("     <i>%s</i>\n", reason))
		}
		sb.WriteString("\n")
	}

	heat := result.Heat
	heatAdvice := heatAdviceIndonesian(heat.HeatLevel)
	sb.WriteString(fmt.Sprintf("Heat: <b>%s</b> %s | Long %.1f Short %.1f Net %+.1f\n",
		heat.HeatLevel, alphaHeatEmoji(heat.HeatLevel),
		heat.LongExposure, heat.ShortExposure, heat.NetExposure))
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", heatAdvice))
	return sb.String()
}

// regimeIndonesian returns Indonesian description of a macro regime.
func regimeIndonesian(regime string) string {
	switch regime {
	case "EXPANSION":
		return "ekonomi tumbuh, risk-on"
	case "SLOWDOWN":
		return "ekonomi melambat, hati-hati"
	case "RECESSION":
		return "kontraksi ekonomi, risk-off"
	case "RECOVERY":
		return "ekonomi pulih, awal risk-on"
	case "GOLDILOCKS":
		return "pertumbuhan moderat, inflasi terkendali"
	case "NEUTRAL":
		return "tidak ada tren makro dominan"
	default:
		return "fase ekonomi saat ini"
	}
}

// heatAdviceIndonesian returns actionable Indonesian advice based on heat level.
func heatAdviceIndonesian(h strategy.HeatLevel) string {
	switch h {
	case strategy.HeatCold:
		return "Eksposur rendah — aman untuk tambah posisi baru"
	case strategy.HeatWarm:
		return "Eksposur sedang — masih aman, jangan terlalu agresif"
	case strategy.HeatHot:
		return "Eksposur tinggi — kurangi agresivitas, pertimbangkan take profit"
	case strategy.HeatOverheat:
		return "⚠️ OVERHEAT — segera kurangi posisi!"
	default:
		return "Evaluasi eksposur portfolio"
	}
}

func formatHeat(heat strategy.PortfolioHeat) string {
	emoji := alphaHeatEmoji(heat.HeatLevel)
	advice := heatAdviceIndonesian(heat.HeatLevel)
	return fmt.Sprintf(`<b>🌡️ Portfolio Heat</b>

Level: %s <b>%s</b>
Posisi Aktif: %d
Eksposur Long:  %.2f
Eksposur Short: %.2f
Eksposur Net:   %+.2f
Total: %.0f%%

<i>%s</i>

<i>%s UTC</i>`,
		emoji, heat.HeatLevel,
		heat.ActiveTrades,
		heat.LongExposure,
		heat.ShortExposure,
		heat.NetExposure,
		heat.TotalExposure*100,
		advice,
		fmtutil.FormatDateTimeUTC(heat.UpdatedAt))
}

// formatRiskParity renders a risk-parity sizing analysis for Telegram HTML.
func formatRiskParity(rp *strategy.RiskParityResult) string {
	if rp == nil {
		return ""
	}

	adviceEmoji := "⚖️"
	switch rp.Recommendation {
	case strategy.SizingScaleDown:
		adviceEmoji = "🔻"
	case strategy.SizingScaleUp:
		adviceEmoji = "🔺"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>⚖️ Risk Parity Sizing</b>\n\n"+
		"%s Rekomendasi: <b>%s</b>\n"+
		"Heat Total: <b>%.1f%%</b> / %.1f%% maks\n"+
		"Kelly: %.1f%% (½K: %.1f%%)\n",
		adviceEmoji, rp.Recommendation,
		rp.TotalHeatPct, rp.MaxHeatPct,
		rp.KellyFraction*100, rp.HalfKelly*100))

	if len(rp.HeatBreakdown) > 0 {
		sb.WriteString("\n<b>Heat per Posisi:</b>\n")
		for _, h := range rp.HeatBreakdown {
			bar := heatBar(h.RiskPct, rp.MaxHeatPct/float64(len(rp.HeatBreakdown)))
			sb.WriteString(fmt.Sprintf("  %s: $%.0f (%.1f%%) %s\n", h.Symbol, h.RiskAmt, h.RiskPct, bar))
		}
	}

	if len(rp.AdjustedPositions) > 0 {
		sb.WriteString("\n<b>Sizing Adjustment:</b>\n")
		for _, a := range rp.AdjustedPositions {
			arrow := "→"
			if a.ScaleFactor < 0.95 {
				arrow = "↓"
			} else if a.ScaleFactor > 1.05 {
				arrow = "↑"
			}
			sb.WriteString(fmt.Sprintf("  %s: %.0f %s %.0f (×%.2f)\n",
				a.Symbol, a.OriginalSize, arrow, a.RecommendedSize, a.ScaleFactor))
		}
	}

	return sb.String()
}

// heatBar renders a small inline bar for heat percentage.
func heatBar(pct, thresholdPerPos float64) string {
	if thresholdPerPos <= 0 {
		return ""
	}
	ratio := pct / thresholdPerPos
	switch {
	case ratio >= 1.5:
		return "🔴"
	case ratio >= 1.0:
		return "🟡"
	default:
		return "🟢"
	}
}

func formatRankX(result *factors.RankingResult) string {
	if result == nil || len(result.Assets) == 0 {
		return "⚠️ Tidak ada data ranking."
	}

	all := result.Assets
	topN := 5
	if topN > len(all) {
		topN = len(all)
	}
	top := all[:topN]

	// Bottom: take from end, but skip any that are already in top
	topSet := make(map[string]bool, topN)
	for _, a := range top {
		topSet[a.Currency] = true
	}
	bottom := make([]factors.RankedAsset, 0, 5)
	for i := len(all) - 1; i >= 0 && len(bottom) < 5; i-- {
		if !topSet[all[i].Currency] {
			bottom = append(bottom, all[i])
		}
	}

	var sb strings.Builder
	sb.WriteString("<b>📈 RankX Leaderboard</b>\n")
	sb.WriteString("<i>Atas = kandidat long (beli), bawah = kandidat short (jual)</i>\n\n")
	sb.WriteString("<b>🟢 Kandidat Long:</b>\n")
	for i, a := range top {
		sb.WriteString(fmt.Sprintf("  %d. <b>%s</b> %.2f %s\n",
			i+1, html.EscapeString(a.Currency), a.CompositeScore, alphaSignalEmoji(string(a.Signal))))
	}
	if len(bottom) > 0 {
		sb.WriteString("\n<b>🔴 Kandidat Short:</b>\n")
		for i, a := range bottom {
			sb.WriteString(fmt.Sprintf("  %d. <b>%s</b> %.2f %s\n",
				i+1, html.EscapeString(a.Currency), a.CompositeScore, alphaSignalEmoji(string(a.Signal))))
		}
	}
	sb.WriteString(fmt.Sprintf("\n<i>%s</i>", fmtutil.FormatDateTimeUTC(result.ComputedAt)))
	return sb.String()
}

func formatTransition(tw strategy.TransitionWarning, currentRegime string) string {
	regimeDesc := regimeIndonesian(currentRegime)

	if !tw.IsActive && tw.Probability < 0.30 {
		return fmt.Sprintf(`<b>🔄 Monitor Transisi Regime</b>

Regime Saat Ini: <b>%s</b>
<i>%s</i>

Probabilitas Transisi: <b>%.0f%%</b>
Status: ✅ <i>Stabil — tidak ada transisi terdeteksi</i>

<i>Artinya: kondisi ekonomi makro tidak menunjukkan perubahan signifikan. Lanjutkan strategi sesuai regime saat ini.</i>

<i>%s</i>`,
			html.EscapeString(currentRegime),
			regimeDesc,
			tw.Probability*100,
			fmtutil.FormatDateTimeUTC(time.Now()))
	}

	emoji := "⚠️"
	statusWord := "Peringatan"
	if tw.IsActive {
		emoji = "🚨"
		statusWord = "AKTIF"
	}

	affected := "N/A"
	if len(tw.AffectedAssets) > 0 {
		affected = html.EscapeString(strings.Join(tw.AffectedAssets, ", "))
	}

	fromDesc := regimeIndonesian(tw.FromRegime)
	toDesc := regimeIndonesian(tw.ToRegime)

	return fmt.Sprintf(`<b>%s Transisi Regime — %s</b>

Regime Saat Ini: <b>%s</b> — <i>%s</i>
Transisi: <b>%s → %s</b>
<i>Dari "%s" ke "%s"</i>

Probabilitas: <b>%.0f%%</b>
Aset Terdampak: %s

<i>Saran: Kurangi ukuran posisi dan hindari entry baru yang berlawanan dengan regime yang akan datang.</i>`,
		emoji, statusWord,
		html.EscapeString(currentRegime), regimeDesc,
		html.EscapeString(tw.FromRegime),
		html.EscapeString(tw.ToRegime),
		fromDesc, toDesc,
		tw.Probability*100,
		affected)
}

func formatCryptoAlpha(results map[string]*microstructure.Signal, symbols []string, tvl *defillama.TVLSummary) string {
	if len(results) == 0 {
		return "⚠️ Tidak ada data microstructure."
	}

	sorted := make([]string, len(symbols))
	copy(sorted, symbols)
	sort.Strings(sorted)

	var sb strings.Builder
	sb.WriteString("<b>⚡ Crypto Microstructure Alpha</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n\n", fmtutil.FormatDateTimeUTC(time.Now())))

	// DeFi TVL context from DeFiLlama (graceful skip if unavailable)
	if tvl != nil && tvl.Available {
		trendEmoji := "➡️"
		switch tvl.Trend {
		case "EXPANDING":
			trendEmoji = "📈"
		case "CONTRACTING":
			trendEmoji = "📉"
		}
		sb.WriteString(fmt.Sprintf("🏗️ DeFi TVL: %s (%+.1f%% 7d) — %s %s\n\n",
			defillama.FormatTVLBillions(tvl.Current), tvl.Change7D, tvl.Trend, trendEmoji))
	}

	for _, sym := range sorted {
		sig, ok := results[sym]
		if !ok {
			continue
		}
		biasEmoji := alphaMicroEmoji(sig.Bias)
		confirmTag := ""
		if sig.ConfirmEntry {
			confirmTag = " ✅ KONFIRMASI"
		}
		displaySym := strings.TrimSuffix(sym, "USDT")
		sb.WriteString(fmt.Sprintf("%s <b>%s</b>%s\n", biasEmoji, displaySym, confirmTag))
		sb.WriteString(fmt.Sprintf("  OB Imbalance: %+.2f | Taker Buy: %.0f%%\n",
			sig.BidAskImbalance, sig.TakerBuyRatio*100))
		if sig.OIChange != 0 {
			sb.WriteString(fmt.Sprintf("  OI Change: %+.1f%% | LS Ratio: %.2f\n",
				sig.OIChange, sig.LongShortRatio))
		}
		if sig.FundingStats != nil {
			fs := sig.FundingStats
			sb.WriteString(fmt.Sprintf("  Funding: %+.4f%% (7d avg: %+.4f%%)\n",
				fs.Current*100, fs.Avg7D*100))
			sb.WriteString(fmt.Sprintf("  30d: avg %+.4f%% | min %+.4f%% | max %+.4f%%\n",
				fs.Avg30D*100, fs.Min30D*100, fs.Max30D*100))
			sb.WriteString(fmt.Sprintf("  Regime: %s (pctl: %.0f%%)\n",
				fs.Regime, fs.Percentile))
		} else if sig.FundingRate != 0 {
			sb.WriteString(fmt.Sprintf("  Funding: %+.4f%%\n", sig.FundingRate*100))
		}
		sb.WriteString(fmt.Sprintf("  Bias: <b>%s</b> (kekuatan %.0f%%)\n",
			sig.Bias, sig.Strength*100))
		// Interpretation
		interp := cryptoInterpretIndonesian(sig)
		sb.WriteString(fmt.Sprintf("  → <i>%s</i>\n\n", interp))
	}
	return sb.String()
}

// cryptoInterpretIndonesian gives Indonesian interpretation for a crypto signal.
func cryptoInterpretIndonesian(sig *microstructure.Signal) string {
	var parts []string
	switch sig.Bias {
	case microstructure.BiasBullish:
		parts = append(parts, "tekanan beli dominan")
	case microstructure.BiasBearish:
		parts = append(parts, "tekanan jual dominan")
	case microstructure.BiasConflict:
		parts = append(parts, "sinyal bertentangan, tunggu konfirmasi")
	default:
		parts = append(parts, "tidak ada tekanan dominan")
	}
	if sig.FundingStats != nil {
		switch sig.FundingStats.Regime {
		case "POSITIVE_BIAS":
			parts = append(parts, "funding positif berkepanjangan (crowded long)")
		case "NEGATIVE_BIAS":
			parts = append(parts, "funding negatif berkepanjangan (squeeze risk)")
		}
		if sig.FundingStats.Percentile > 90 {
			parts = append(parts, "funding di pctl 90+% (extreme)")
		} else if sig.FundingStats.Percentile < 10 {
			parts = append(parts, "funding di pctl <10% (extreme rendah)")
		}
	} else if sig.FundingRate > 0.01 {
		parts = append(parts, "funding rate tinggi (hati-hati long)")
	} else if sig.FundingRate < -0.01 {
		parts = append(parts, "funding rate negatif (hati-hati short)")
	}
	if sig.ConfirmEntry {
		parts = append(parts, "entry terkonfirmasi ✅")
	}
	return strings.Join(parts, ", ")
}

// ---------------------------------------------------------------------------
// Local helpers (prefixed with alpha to avoid package-level collisions)
// ---------------------------------------------------------------------------

func alphaSignalEmoji(sig string) string {
	switch sig {
	case "STRONG_LONG":
		return "🟢🟢"
	case "LONG":
		return "🟢 Bullish"
	case "STRONG_SHORT":
		return "🔴🔴"
	case "SHORT":
		return "🔴 Bearish"
	default:
		return "⚪ Neutral"
	}
}

func alphaScoreBar(score float64) string {
	bars := int((score+1)/2*10 + 0.5)
	if bars < 0 {
		bars = 0
	}
	if bars > 10 {
		bars = 10
	}
	return strings.Repeat("█", bars) + strings.Repeat("░", 10-bars)
}

func alphaConvBar(c float64) string {
	bars := int(c*5 + 0.5)
	if bars > 5 {
		bars = 5
	}
	if bars < 0 {
		bars = 0
	}
	return strings.Repeat("▪", bars) + strings.Repeat("·", 5-bars)
}

func alphaConvEmoji(l strategy.ConvictionLevel) string {
	switch l {
	case strategy.ConvictionHigh:
		return "🔥"
	case strategy.ConvictionMedium:
		return "📌"
	case strategy.ConvictionLow:
		return "💡"
	default:
		return "⛔"
	}
}

func alphaHeatEmoji(h strategy.HeatLevel) string {
	switch h {
	case strategy.HeatCold:
		return "🔵"
	case strategy.HeatWarm:
		return "🟡"
	case strategy.HeatHot:
		return "🟠"
	case strategy.HeatOverheat:
		return "🔴 Bearish"
	default:
		return "⚪ Neutral"
	}
}

func alphaMicroEmoji(b microstructure.Bias) string {
	switch b {
	case microstructure.BiasBullish:
		return "🟢 Bullish"
	case microstructure.BiasBearish:
		return "🔴 Bearish"
	case microstructure.BiasConflict:
		return "🟡"
	default:
		return "⚪ Neutral"
	}
}

func alphaErr(err error) string {
	if err == nil {
		return "unknown error"
	}
	return html.EscapeString(err.Error())
}
