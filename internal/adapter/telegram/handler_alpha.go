package telegram

// handler_alpha.go — Handlers for new factor/strategy/microstructure commands:
//   /factors     — cross-sectional factor ranking
//   /playbook    — strategy playbook (top long/short + macro context)
//   /heat        — portfolio exposure heat
//   /rankx       — compact rank leaderboard
//   /transition  — regime transition warning
//   /cryptoalpha — Bybit microstructure confirmation for top crypto signals

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/factors"
	"github.com/arkcode369/ark-intelligent/internal/service/microstructure"
	"github.com/arkcode369/ark-intelligent/internal/service/strategy"
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

// WithAlpha injects AlphaServices into the handler and registers alpha commands.
func (h *Handler) WithAlpha(a *AlphaServices) *Handler {
	h.alpha = a
	if a != nil {
		h.registerAlphaCommands()
	}
	return h
}

// registerAlphaCommands wires the new commands into the bot.
func (h *Handler) registerAlphaCommands() {
	h.bot.RegisterCommand("/xfactors", h.cmdXFactors)   // /xfactors = cross-sectional factor ranking
	h.bot.RegisterCommand("/playbook", h.cmdPlaybook)
	h.bot.RegisterCommand("/heat", h.cmdHeat)
	h.bot.RegisterCommand("/rankx", h.cmdRankX)
	h.bot.RegisterCommand("/transition", h.cmdTransition)
	h.bot.RegisterCommand("/cryptoalpha", h.cmdCryptoAlpha)
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
		_, err2 := h.bot.SendHTML(ctx, chatID, "❌ Could not build asset profiles: "+alphaErr(err))
		return err2
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
		_, err2 := h.bot.SendHTML(ctx, chatID, "❌ Could not build profiles: "+alphaErr(err))
		return err2
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
		_, err2 := h.bot.SendHTML(ctx, chatID, "❌ Could not build profiles: "+alphaErr(err))
		return err2
	}
	ranking := h.alpha.FactorEngine.Rank(profiles)
	result := h.alpha.StrategyEngine.Generate(strategy.Input{Ranking: ranking})
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
		_, err2 := h.bot.SendHTML(ctx, chatID, "❌ Could not build profiles: "+alphaErr(err))
		return err2
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

	tProb, tFrom, tTo := h.alpha.ProfileBuilder.GetTransitionProb(ctx)
	macroRegime := h.alpha.ProfileBuilder.GetMacroRegime(ctx)
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
	_, err := h.bot.SendHTML(ctx, chatID, formatCryptoAlpha(results, symbols))
	return err
}

// ---------------------------------------------------------------------------
// Formatters
// ---------------------------------------------------------------------------

func formatFactorRanking(result *factors.RankingResult) string {
	if result == nil || len(result.Assets) == 0 {
		return "⚠️ No factor data available."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>📊 Factor Ranking</b> — %d assets\n", result.AssetCount))
	sb.WriteString(fmt.Sprintf("<i>%s UTC</i>\n\n", result.ComputedAt.UTC().Format("02 Jan 15:04")))

	for _, a := range result.Assets {
		emoji := alphaSignalEmoji(string(a.Signal))
		bar := alphaScoreBar(a.CompositeScore)
		sb.WriteString(fmt.Sprintf("%s <b>#%d %s</b> %s (%.2f)\n",
			emoji, a.Rank, html.EscapeString(a.Currency), bar, a.CompositeScore))
		sb.WriteString(fmt.Sprintf("   Mom:%.2f TQ:%.2f CA:%.2f LV:%.2f\n\n",
			a.Scores.Momentum, a.Scores.TrendQuality,
			a.Scores.CarryAdjusted, a.Scores.LowVol))
	}
	return sb.String()
}

func formatPlaybook(result *strategy.PlaybookResult) string {
	if result == nil {
		return "⚠️ No playbook data."
	}
	var sb strings.Builder
	sb.WriteString("<b>🎯 Strategy Playbook</b>\n")
	if result.MacroRegime != "" {
		sb.WriteString(fmt.Sprintf("Regime: <b>%s</b>\n", html.EscapeString(result.MacroRegime)))
	}
	sb.WriteString(fmt.Sprintf("<i>%s UTC</i>\n\n", result.ComputedAt.UTC().Format("02 Jan 15:04")))

	if result.Transition.IsActive {
		sb.WriteString(fmt.Sprintf("⚠️ <b>TRANSITION:</b> %s → %s (%.0f%% prob)\n\n",
			html.EscapeString(result.Transition.FromRegime),
			html.EscapeString(result.Transition.ToRegime),
			result.Transition.Probability*100))
	}

	longs := result.TopLong(5)
	shorts := result.TopShort(5)

	if len(longs) > 0 {
		sb.WriteString("<b>🟢 LONG Ideas:</b>\n")
		for _, e := range longs {
			convBar := alphaConvBar(e.Conviction)
			fit := ""
			if e.RegimeFit == "ALIGNED" {
				fit = " ✓"
			} else if e.RegimeFit == "AGAINST_REGIME" {
				fit = " ✗"
			}
			carry := ""
			if e.RateDiffBps != 0 {
				carry = fmt.Sprintf(" carry:%+.0fbps", e.RateDiffBps)
			}
			sb.WriteString(fmt.Sprintf("  %s <b>%s</b>%s%s %s\n",
				alphaConvEmoji(e.ConvLevel), html.EscapeString(e.Currency), fit, carry, convBar))
		}
		sb.WriteString("\n")
	}

	if len(shorts) > 0 {
		sb.WriteString("<b>🔴 SHORT Ideas:</b>\n")
		for _, e := range shorts {
			convBar := alphaConvBar(e.Conviction)
			carry := ""
			if e.RateDiffBps != 0 {
				carry = fmt.Sprintf(" carry:%+.0fbps", e.RateDiffBps)
			}
			sb.WriteString(fmt.Sprintf("  %s <b>%s</b>%s %s\n",
				alphaConvEmoji(e.ConvLevel), html.EscapeString(e.Currency), carry, convBar))
		}
		sb.WriteString("\n")
	}

	heat := result.Heat
	sb.WriteString(fmt.Sprintf("Heat: <b>%s</b> %s | Long %.1f Short %.1f Net %+.1f\n",
		heat.HeatLevel, alphaHeatEmoji(heat.HeatLevel),
		heat.LongExposure, heat.ShortExposure, heat.NetExposure))
	return sb.String()
}

func formatHeat(heat strategy.PortfolioHeat) string {
	emoji := alphaHeatEmoji(heat.HeatLevel)
	return fmt.Sprintf(`<b>🌡️ Portfolio Heat</b>

Level: %s <b>%s</b>
Active Trades: %d
Long Exposure:  %.2f
Short Exposure: %.2f
Net Exposure:   %+.2f
Total: %.0f%%

<i>%s UTC</i>`,
		emoji, heat.HeatLevel,
		heat.ActiveTrades,
		heat.LongExposure,
		heat.ShortExposure,
		heat.NetExposure,
		heat.TotalExposure*100,
		heat.UpdatedAt.UTC().Format("02 Jan 15:04"))
}

func formatRankX(result *factors.RankingResult) string {
	if result == nil || len(result.Assets) == 0 {
		return "⚠️ No ranking data."
	}

	all := result.Assets
	top := all
	if len(top) > 5 {
		top = all[:5]
	}
	bottom := make([]factors.RankedAsset, 0, 5)
	for i := len(all) - 1; i >= 0 && len(bottom) < 5; i-- {
		bottom = append(bottom, all[i])
	}

	var sb strings.Builder
	sb.WriteString("<b>📈 RankX Leaderboard</b>\n\n")
	sb.WriteString("<b>🟢 Longs:</b>\n")
	for i, a := range top {
		sb.WriteString(fmt.Sprintf("  %d. <b>%s</b> %.2f %s\n",
			i+1, html.EscapeString(a.Currency), a.CompositeScore, alphaSignalEmoji(string(a.Signal))))
	}
	sb.WriteString("\n<b>🔴 Shorts:</b>\n")
	for i, a := range bottom {
		sb.WriteString(fmt.Sprintf("  %d. <b>%s</b> %.2f %s\n",
			i+1, html.EscapeString(a.Currency), a.CompositeScore, alphaSignalEmoji(string(a.Signal))))
	}
	sb.WriteString(fmt.Sprintf("\n<i>%s UTC</i>", result.ComputedAt.UTC().Format("02 Jan 15:04")))
	return sb.String()
}

func formatTransition(tw strategy.TransitionWarning, currentRegime string) string {
	if !tw.IsActive && tw.Probability < 0.30 {
		return fmt.Sprintf(`<b>🔄 Regime Transition Monitor</b>

Current Regime: <b>%s</b>
Transition Probability: <b>%.0f%%</b>
Status: ✅ <i>Stable — no transition detected</i>

<i>%s UTC</i>`,
			html.EscapeString(currentRegime),
			tw.Probability*100,
			time.Now().UTC().Format("02 Jan 15:04"))
	}

	emoji := "⚠️"
	if tw.IsActive {
		emoji = "🚨"
	}
	affected := "N/A"
	if len(tw.AffectedAssets) > 0 {
		affected = html.EscapeString(strings.Join(tw.AffectedAssets, ", "))
	}

	return fmt.Sprintf(`<b>%s Regime Transition Alert</b>

Current Regime: <b>%s</b>
Transition: <b>%s → %s</b>
Probability: <b>%.0f%%</b>
Affected Assets: %s

<i>%s</i>

Reduce position sizes and avoid new entries against the incoming regime.`,
		emoji,
		html.EscapeString(currentRegime),
		html.EscapeString(tw.FromRegime),
		html.EscapeString(tw.ToRegime),
		tw.Probability*100,
		affected,
		html.EscapeString(tw.Note),
	)
}

func formatCryptoAlpha(results map[string]*microstructure.Signal, symbols []string) string {
	if len(results) == 0 {
		return "⚠️ No microstructure data available."
	}

	sorted := make([]string, len(symbols))
	copy(sorted, symbols)
	sort.Strings(sorted)

	var sb strings.Builder
	sb.WriteString("<b>⚡ Crypto Microstructure Alpha</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s UTC</i>\n\n", time.Now().UTC().Format("02 Jan 15:04")))

	for _, sym := range sorted {
		sig, ok := results[sym]
		if !ok {
			continue
		}
		biasEmoji := alphaMicroEmoji(sig.Bias)
		confirmTag := ""
		if sig.ConfirmEntry {
			confirmTag = " ✅ CONFIRM"
		}
		displaySym := strings.TrimSuffix(sym, "USDT")
		sb.WriteString(fmt.Sprintf("%s <b>%s</b>%s\n", biasEmoji, displaySym, confirmTag))
		sb.WriteString(fmt.Sprintf("  OB Imbalance: %+.2f | Taker Buy: %.0f%%\n",
			sig.BidAskImbalance, sig.TakerBuyRatio*100))
		if sig.OIChange != 0 {
			sb.WriteString(fmt.Sprintf("  OI Change: %+.1f%% | LS Ratio: %.2f\n",
				sig.OIChange, sig.LongShortRatio))
		}
		if sig.FundingRate != 0 {
			sb.WriteString(fmt.Sprintf("  Funding: %+.4f%%\n", sig.FundingRate*100))
		}
		sb.WriteString(fmt.Sprintf("  Bias: <b>%s</b> (strength %.0f%%)\n\n",
			sig.Bias, sig.Strength*100))
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Local helpers (prefixed with alpha to avoid package-level collisions)
// ---------------------------------------------------------------------------

func alphaSignalEmoji(sig string) string {
	switch sig {
	case "STRONG_LONG":
		return "🟢🟢"
	case "LONG":
		return "🟢"
	case "STRONG_SHORT":
		return "🔴🔴"
	case "SHORT":
		return "🔴"
	default:
		return "⚪"
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
		return "🔴"
	default:
		return "⚪"
	}
}

func alphaMicroEmoji(b microstructure.Bias) string {
	switch b {
	case microstructure.BiasBullish:
		return "🟢"
	case microstructure.BiasBearish:
		return "🔴"
	case microstructure.BiasConflict:
		return "🟡"
	default:
		return "⚪"
	}
}

func alphaErr(err error) string {
	if err == nil {
		return "unknown error"
	}
	return html.EscapeString(err.Error())
}
