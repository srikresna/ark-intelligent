package telegram

// /start, /help, /status — Onboarding & Core UI

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

// ---------------------------------------------------------------------------

func (h *Handler) cmdStart(ctx context.Context, chatID string, userID int64, args string) error {
	// Persist chatID so the scheduler can push alerts to this user.
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	if prefs.ChatID != chatID {
		prefs.ChatID = chatID
		_ = h.prefsRepo.Set(ctx, userID, prefs)
	}

	// Parse deep link parameters (t.me/botname?start=PARAM)
	args = strings.TrimSpace(args)
	if args != "" {
		h.handleDeepLinkArgs(ctx, chatID, userID, args, &prefs)
	}

	// If user already has experience level set, show welcome back with quick actions
	// (or execute cached deep link command intent).
	if prefs.ExperienceLevel != "" {
		// Check for pending deep link command intent.
		if intent := h.deepLinks.Pop(userID); intent != nil {
			log.Info().
				Int64("user", userID).
				Str("cmd", intent.Command).
				Str("args", intent.Args).
				Msg("deep link: auto-executing cached command intent")
			return h.executeDeepLinkCommand(ctx, chatID, userID, intent.Command, intent.Args)
		}

		// TASK-001-EXT: Welcome back with quick-access keyboard
		roleConfig := GetRoleConfig(prefs.ExperienceLevel)
		welcomeBack := fmt.Sprintf(`👋 <b>Selamat datang kembali!</b>

Level: %s

<b>Starter Kit:</b>
%s

Pilih command atau ketik /help untuk melihat semua command.`,
			roleConfig.Name,
			formatStarterKit(roleConfig.StarterKit),
		)

		_, err := h.bot.SendWithKeyboard(ctx, chatID, welcomeBack, h.kb.StarterKitMenu(prefs.ExperienceLevel))
		return err
	}

	// New user → interactive onboarding with role selector.
	welcome := `🦅 <b>Selamat datang di ARK Intelligence!</b>
<i>Institutional Flow &amp; Macro Analytics</i>

Sebelum mulai, pilih level pengalaman trading kamu:

🌱 <b>Pemula</b> — Baru mulai trading, ingin belajar dasar
📈 <b>Intermediate</b> — Sudah trading aktif, ingin tools analisis
🏛 <b>Pro</b> — Trader berpengalaman, butuh data institusional`

	_, err := h.bot.SendWithKeyboard(ctx, chatID, welcome, h.kb.OnboardingRoleMenu())
	return err
}

// cmdOnboarding allows users to restart the onboarding flow voluntarily (TASK-001-EXT).
func (h *Handler) cmdOnboarding(ctx context.Context, chatID string, userID int64, args string) error {
	// Reset onboarding state
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	prefs.ExperienceLevel = "" // Reset role to trigger re-selection
	prefs.OnboardingStep = 0
	_ = h.prefsRepo.Set(ctx, userID, prefs)

	welcome := `🔄 <b>Restart Onboarding</b>

Pilih level pengalaman trading kamu:

🌱 <b>Pemula</b> — Baru mulai trading, ingin belajar dasar
📈 <b>Intermediate</b> — Sudah trading aktif, ingin tools analisis
🏛 <b>Pro</b> — Trader berpengalaman, butuh data institusional`

	_, err := h.bot.SendWithKeyboard(ctx, chatID, welcome, h.kb.OnboardingRoleMenu())
	return err
}

// handleDeepLinkArgs parses and processes deep link start parameters.
// Supported formats:
//   - ref_<userID>  → store referrer in user profile
//   - cmd_<command>_<symbol> → cache intent for post-onboarding execution
//   - anything else → log and ignore (backward compatible)
func (h *Handler) handleDeepLinkArgs(ctx context.Context, chatID string, userID int64, args string, prefs *domain.UserPrefs) {
	log.Info().
		Int64("user", userID).
		Str("args", args).
		Msg("deep link: processing start parameter")

	switch {
	case strings.HasPrefix(args, "ref_"):
		// Referral tracking: ref_<referrerUserID>
		refStr := strings.TrimPrefix(args, "ref_")
		referrerID, err := strconv.ParseInt(refStr, 10, 64)
		if err != nil || referrerID <= 0 || referrerID == userID {
			log.Warn().Str("ref", refStr).Msg("deep link: invalid or self-referral, ignoring")
			return
		}
		// Only record first referral (don't overwrite)
		if prefs.ReferrerID == 0 {
			prefs.ReferrerID = referrerID
			prefs.ReferredAt = timeutil.NowWIB().Format("2006-01-02 15:04")
			_ = h.prefsRepo.Set(ctx, userID, *prefs)
			log.Info().
				Int64("user", userID).
				Int64("referrer", referrerID).
				Msg("deep link: referral recorded")
		}

	case strings.HasPrefix(args, "cmd_"):
		// Command pre-fill: cmd_<command>[_<symbol>]
		// Examples: cmd_cot_EUR, cmd_outlook, cmd_cta_XAU
		parts := strings.SplitN(strings.TrimPrefix(args, "cmd_"), "_", 2)
		command := parts[0]
		cmdArgs := ""
		if len(parts) > 1 {
			cmdArgs = parts[1]
		}

		if command == "" {
			log.Warn().Str("args", args).Msg("deep link: empty command, ignoring")
			return
		}

		// Cache the intent — will be executed after onboarding completes
		h.deepLinks.Set(userID, command, cmdArgs)
		log.Info().
			Int64("user", userID).
			Str("cmd", command).
			Str("cmdArgs", cmdArgs).
			Msg("deep link: command intent cached (awaiting onboarding)")

	default:
		// Unknown format — log for analytics and ignore gracefully
		log.Info().
			Int64("user", userID).
			Str("args", args).
			Msg("deep link: unrecognized parameter, ignoring")
	}
}

// executeDeepLinkCommand routes a deep link command intent to the appropriate handler.
func (h *Handler) executeDeepLinkCommand(ctx context.Context, chatID string, userID int64, command, args string) error {
	switch command {
	case "cot":
		return h.cmdCOT(ctx, chatID, userID, args)
	case "outlook":
		return h.cmdOutlook(ctx, chatID, userID, args)
	case "cta":
		return h.cmdCTA(ctx, chatID, userID, args)
	case "quant":
		return h.cmdQuant(ctx, chatID, userID, args)
	case "vp":
		return h.cmdVP(ctx, chatID, userID, args)
	case "alpha":
		return h.cmdAlpha(ctx, chatID, 0, args)
	case "gex":
		return h.cmdGEX(ctx, chatID, userID, args)
	case "macro":
		return h.cmdMacro(ctx, chatID, userID, args)
	case "bias":
		return h.cmdBias(ctx, chatID, userID, args)
	case "price":
		return h.cmdPrice(ctx, chatID, userID, args)
	case "calendar":
		return h.cmdCalendar(ctx, chatID, userID, args)
	case "sentiment":
		return h.cmdSentiment(ctx, chatID, userID, args)
	case "seasonal":
		return h.cmdSeasonal(ctx, chatID, userID, args)
	case "rank":
		return h.cmdRank(ctx, chatID, userID, args)
	case "levels":
		return h.cmdLevels(ctx, chatID, userID, args)
	case "backtest":
		return h.cmdBacktest(ctx, chatID, userID, args)
	case "impact":
		return h.cmdImpact(ctx, chatID, 0, args)
	case "intermarket":
		return h.cmdIntermarket(ctx, chatID, 0, args)
	case "wyckoff":
		return h.cmdWyckoff(ctx, chatID, userID, args)
	case "smc":
		return h.cmdSMC(ctx, chatID, userID, args)
	case "elliott":
		return h.cmdElliott(ctx, chatID, userID, args)
	default:
		log.Warn().Str("cmd", command).Msg("deep link: unknown command, showing help")
		return h.sendHelp(ctx, chatID, userID)
	}
}

// cbOnboard handles the onboarding flow callbacks (role selection + tutorial).
func (h *Handler) cbOnboard(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "onboard:")

	// "showhelp" → show full help menu
	if action == "showhelp" {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		return h.sendHelp(ctx, chatID, userID)
	}

	// Role selection: beginner / intermediate / pro
	level := action
	if level != "beginner" && level != "intermediate" && level != "pro" {
		return nil
	}

	// Persist experience level
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	prefs.ExperienceLevel = level
	prefs.OnboardingStep = 1 // Step 1: role chosen
	_ = h.prefsRepo.Set(ctx, userID, prefs)

	// Delete the role selector message
	_ = h.bot.DeleteMessage(ctx, chatID, msgID)

	// TASK-001-EXT: Show tutorial welcome prompt with starter kit
	roleConfig := GetRoleConfig(level)
	welcomeMsg := fmt.Sprintf(`✅ <b>Level: %s</b>

%s

<b>Starter Kit:</b>
%s

Mau lihat tutorial penggunaan?`,
		roleConfig.Name,
		roleConfig.Description,
		formatStarterKit(roleConfig.StarterKit),
	)

	_, err := h.bot.SendWithKeyboard(ctx, chatID, welcomeMsg, h.kb.TutorialWelcomeKeyboard(level))
	return err
}

// formatStarterKit formats the starter kit commands as a bullet list.
func formatStarterKit(commands []string) string {
	var result string
	for _, cmd := range commands {
		result += fmt.Sprintf("• <code>%s</code>\n", cmd)
	}
	return result
}

// cbTutorial handles tutorial navigation callbacks (TASK-001-EXT).
func (h *Handler) cbTutorial(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "tutorial:")
	parts := strings.Split(action, ":")
	if len(parts) < 1 {
		return nil
	}

	switch parts[0] {
	case "start":
		// Start tutorial from step 0
		if len(parts) < 2 {
			return nil
		}
		role := parts[1]
		return h.showTutorialStep(ctx, chatID, msgID, userID, role, 0)

	case "step":
		// Navigate to specific step
		if len(parts) < 3 {
			return nil
		}
		role := parts[1]
		step, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil
		}
		return h.showTutorialStep(ctx, chatID, msgID, userID, role, step)

	case "skip", "done":
		// End tutorial, show starter kit
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		prefs, _ := h.prefsRepo.Get(ctx, userID)
		return h.showStarterKit(ctx, chatID, userID, prefs.ExperienceLevel)
	}

	return nil
}

// showTutorialStep displays a specific tutorial step with navigation.
func (h *Handler) showTutorialStep(ctx context.Context, chatID string, msgID int, userID int64, role string, step int) error {
	roleConfig := GetRoleConfig(role)
	if step < 0 || step >= len(roleConfig.TutorialSteps) {
		return h.showStarterKit(ctx, chatID, userID, role)
	}

	tutorialStep := roleConfig.TutorialSteps[step]
	content := fmt.Sprintf("<b>%s</b>\n\n%s", tutorialStep.Title, tutorialStep.Content)

	kb := h.kb.TutorialStepKeyboard(role, step, len(roleConfig.TutorialSteps))
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, content, kb)
}

// showStarterKit displays the starter kit for the user's role.
func (h *Handler) showStarterKit(ctx context.Context, chatID string, userID int64, level string) error {
	var tutorial string
	switch level {
	case "beginner":
		tutorial = `✅ <b>Level: Pemula</b>

<b>🎓 3 Langkah Memulai:</b>

<b>1️⃣ Cek COT Data</b>
Ketik <code>/cot EUR</code> — lihat posisi big player di Euro

<b>2️⃣ Cek Kalender</b>
Ketik <code>/calendar</code> — jadwal rilis data ekonomi

<b>3️⃣ Cek Harga</b>
Ketik <code>/price EUR</code> — harga terkini + perubahan

Ini menu kamu — klik untuk mulai:`
	case "intermediate":
		tutorial = `✅ <b>Level: Intermediate</b>

<b>🎓 3 Langkah Memulai:</b>

<b>1️⃣ CTA Dashboard</b>
Ketik <code>/cta EUR</code> — analisis teknikal lengkap dengan chart

<b>2️⃣ AI Outlook</b>
Ketik <code>/outlook</code> — analisis gabungan AI (data + sentiment + web)

<b>3️⃣ Macro Regime</b>
Ketik <code>/macro</code> — kondisi makro ekonomi global + dampak ke trading

Ini menu kamu — klik untuk mulai:`
	case "pro":
		tutorial = `✅ <b>Level: Pro / Institutional</b>

<b>🎓 3 Langkah Memulai:</b>

<b>1️⃣ Alpha Engine</b>
Ketik <code>/alpha</code> — factor ranking + playbook + risk dashboard

<b>2️⃣ Volume Profile</b>
Ketik <code>/vp EUR</code> — 10 mode VP termasuk AMT institutional-grade

<b>3️⃣ Quant Analysis</b>
Ketik <code>/quant EUR</code> — 12 model econometric (GARCH, regime, PCA, dll)

Ini menu kamu — klik untuk mulai:`
	default:
		tutorial = `✅ <b>Selamat datang!</b>

Pilih command dari menu di bawah:`
	}

	_, err := h.bot.SendWithKeyboard(ctx, chatID, tutorial, h.kb.StarterKitMenu(level))
	if err != nil {
		return err
	}

	// After onboarding completes, check for pending deep link command intent.
	if intent := h.deepLinks.Pop(userID); intent != nil {
		log.Info().
			Int64("user", userID).
			Str("cmd", intent.Command).
			Str("args", intent.Args).
			Msg("deep link: auto-executing post-onboarding command intent")
		return h.executeDeepLinkCommand(ctx, chatID, userID, intent.Command, intent.Args)
	}

	return nil
}

func (h *Handler) cmdHelp(ctx context.Context, chatID string, userID int64, args string) error {
	// Support /help <category> to directly expand a sub-category
	category := strings.ToLower(strings.TrimSpace(args))
	if category != "" {
		switch category {
		case "market", "research", "ai", "signals", "settings", "admin", "changelog", "shortcuts":
			return h.sendHelpSubCategory(ctx, chatID, userID, category, 0)
		}
	}
	return h.sendHelp(ctx, chatID, userID)
}

// sendHelp sends the interactive category-based help menu.
func (h *Handler) sendHelp(ctx context.Context, chatID string, userID int64) error {
	// Determine user role
	isAdmin := h.bot.isOwner(userID)
	if !isAdmin && h.middleware != nil {
		role := h.middleware.GetUserRole(ctx, userID)
		isAdmin = domain.RoleHierarchy(role) >= domain.RoleHierarchy(domain.RoleAdmin)
	}

	header := `🦅 <b>ARK Intelligence Terminal</b>
<i>Institutional Flow &amp; Macro Analytics</i>

<i>Pilih kategori untuk melihat commands tersedia:</i>`

	// Fetch user pins for personalized keyboard (TASK-078)
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	pins := prefs.PinnedCommands

	var kb ports.InlineKeyboard
	if isAdmin {
		kb = h.kb.HelpCategoryMenuWithAdmin(pins...)
	} else {
		kb = h.kb.HelpCategoryMenu(pins...)
	}

	_, err := h.bot.SendWithKeyboard(ctx, chatID, header, kb)
	return err
}

// sendHelpSubCategory sends or edits the help sub-category message for a given category.
func (h *Handler) sendHelpSubCategory(ctx context.Context, chatID string, userID int64, category string, editMsgID int) error {
	var text string

	switch category {
	case "market":
		text = `📊 <b>Market &amp; Data Commands</b>

/cot — COT institutional positioning · <code>/cot EUR</code>
/rank — Currency strength ranking
/bias — Directional bias summary · <code>/bias EUR</code>
/calendar — Economic calendar · <code>/calendar week</code>
/price — Daily OHLC price context · <code>/price EUR</code>
/levels — Support/resistance levels · <code>/levels EUR</code>
/history — COT history comparison · <code>/history EUR</code>
/ecb — ECB monetary policy dashboard · <code>/ecb</code>
/leading — OECD leading indicators (G7+ CLI) · <code>/leading</code>
/intermarket — Cross-asset correlation signals
/defi — DeFi health dashboard (TVL, DEX, stablecoins)
/onchain — On-chain exchange flows (BTC, ETH)`

	case "research":
		text = `🔬 <b>Research &amp; Alpha Commands</b>

/alpha — Dashboard lengkap (factor + playbook + risk)
/cta — Classical TA dashboard · <code>/cta EUR</code> · <code>/cta EUR 4h</code>
/ctabt — Backtest Classical TA · <code>/ctabt EUR</code> · <code>/ctabt EUR 4h</code>
/quant — Econometric analysis · <code>/quant EUR</code> · <code>/quant XAU 4h</code>
/vp — Volume Profile institutional · <code>/vp EUR</code> · <code>/vp XAU 4h</code>
/ict — ICT/SMC Smart Money Concepts · <code>/ict EURUSD</code> · <code>/ict XAUUSD H4</code>
/gex — Gamma Exposure (crypto options) · <code>/gex BTC</code> · <code>/gex ETH</code>
/backtest — Backtest dashboard (17 sub-views)
/accuracy — Win rate summary
/report — Weekly signal performance
/wyckoff — Wyckoff phase analysis · <code>/wyckoff EURUSD</code>
/smc — SMC structure (BOS/CHoCH) · <code>/smc EURUSD</code>
/elliott — Elliott Wave analysis · <code>/elliott EURUSD</code>`

	case "ai":
		text = `🧠 <b>AI &amp; Outlook Commands</b>

/outlook — Unified AI analysis (all data + web search)
/macro — FRED macro regime + asset performance
/impact — Event impact database · <code>/impact NFP</code>
/sentiment — Sentiment surveys (CNN F&amp;G, AAII, P/C)
/seasonal — Seasonal patterns · <code>/seasonal EUR</code>`

	case "signals":
		text = `⚡ <b>Signals &amp; Alerts</b>

/bias — Directional bias signals · <code>/bias EUR</code>
/cot — COT positioning + conviction score · <code>/cot EUR</code>
/rank — Currency strength ranking

<b>Alert Settings:</b>
Use /settings to configure:
• COT release alerts
• News event alerts (High/Med/All impact)
• Currency filter for alerts
• Alert timing (60/15/5, 15/5/1, 5/1 min)`

	case "settings":
		text = `⚙️ <b>Settings &amp; Preferences</b>

/settings — Preferences dashboard (alerts, language, model)
/membership — Tier info + upgrade · <code>/membership</code>
/clear — Clear AI chat history

<b>Available settings:</b>
• Language: Indonesian / English
• AI Provider: Claude / Gemini
• Claude Model: Opus / Sonnet / Haiku
• COT &amp; AI report alerts on/off
• Currency filter for alerts
• Alert timing presets

<b>⚡ Shortcuts:</b>
<code>/c</code> cot · <code>/q</code> quant · <code>/b</code> bias · <code>/bt</code> backtest
<code>/ce</code> cot · <code>/ca</code> cta · <code>/qe</code> quant (with currency arg)
<code>/bta</code> backtest all · <code>/of</code> outlook fred`

	case "admin":
		// Only show admin section to admins
		isAdmin := h.bot.isOwner(userID)
		if !isAdmin && h.middleware != nil {
			role := h.middleware.GetUserRole(ctx, userID)
			isAdmin = domain.RoleHierarchy(role) >= domain.RoleHierarchy(domain.RoleAdmin)
		}
		if !isAdmin {
			text = "⛔ Admin commands hanya tersedia untuk Admin+"
		} else {
			text = `🔐 <b>Admin Commands</b>

/users — List all registered users with roles
/setrole — Change user role · <code>/setrole &lt;userID&gt; &lt;role&gt;</code>
/ban — Ban a user · <code>/ban &lt;userID&gt;</code>
/unban — Unban a user · <code>/unban &lt;userID&gt;</code>

<b>Roles:</b> owner · admin · member · free · banned`
		}

	case "changelog":
		if h.changelog == "" {
			text = "📋 <b>Changelog</b>\n\n<i>Changelog tidak tersedia.</i>"
		} else {
			// Show a reasonable portion of the changelog
			cl := h.changelog
			if len(cl) > 3500 {
				cl = cl[:3500] + "\n\n<i>... (lihat selengkapnya di /settings → View Changelog)</i>"
			}
			text = "🆕 <b>What's New</b>\n\n" + cl
		}

	case "shortcuts":
		text = "⚡ <b>Quick Shortcuts</b>\n\n" +
			"<i>Aliases for faster typing on mobile:</i>\n\n" +
			"<code>/c</code> → /cot · <code>/cal</code> → /calendar · <code>/out</code> → /outlook\n" +
			"<code>/m</code> → /macro · <code>/b</code> → /bias · <code>/q</code> → /quant\n" +
			"<code>/bt</code> → /backtest · <code>/r</code> → /rank · <code>/s</code> → /sentiment\n" +
			"<code>/p</code> → /price · <code>/l</code> → /levels · <code>/h</code> → /history\n\n" +
			"<i>Tip: All shortcuts accept the same arguments as their full command.</i>"

	default:
		return h.sendHelp(ctx, chatID, userID)
	}

	kb := h.kb.HelpSubMenu()

	if editMsgID > 0 {
		return h.bot.EditWithKeyboard(ctx, chatID, editMsgID, text, kb)
	}
	_, err := h.bot.SendWithKeyboard(ctx, chatID, text, kb)
	return err
}

// cbHelp handles "help:" prefixed callbacks for the interactive help menu.
func (h *Handler) cbHelp(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "help:")

	if action == "back" {
		// Return to category menu
		isAdmin := h.bot.isOwner(userID)
		if !isAdmin && h.middleware != nil {
			role := h.middleware.GetUserRole(ctx, userID)
			isAdmin = domain.RoleHierarchy(role) >= domain.RoleHierarchy(domain.RoleAdmin)
		}

		header := `🦅 <b>ARK Intelligence Terminal</b>
<i>Institutional Flow &amp; Macro Analytics</i>

<i>Pilih kategori untuk melihat commands tersedia:</i>`

		// Fetch user pins for personalized keyboard (TASK-078)
		prefs, _ := h.prefsRepo.Get(ctx, userID)
		pins := prefs.PinnedCommands

		var kb ports.InlineKeyboard
		if isAdmin {
			kb = h.kb.HelpCategoryMenuWithAdmin(pins...)
		} else {
			kb = h.kb.HelpCategoryMenu(pins...)
		}
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, header, kb)
	}

	return h.sendHelpSubCategory(ctx, chatID, userID, action, msgID)
}

func (h *Handler) cmdStatus(ctx context.Context, chatID string, userID int64, args string) error {
	now := timeutil.NowWIB()

	// Check data freshness
	cotAnalyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)

	// AI status
	aiStatus := "Not configured"
	if h.aiAnalyzer != nil {
		if h.aiAnalyzer.IsAvailable() {
			aiStatus = "Available"
		} else {
			aiStatus = "Configured but unavailable"
		}
	}

	html := fmt.Sprintf(`<b>System Status</b>
<code>Time:       %s WIB</code>

<b>Data Sources:</b>
<code>COT:        %d contracts</code>

<b>Services:</b>
<code>AI Engine:  %s</code>

<b>Version:</b> v3.0.0`,
		now.Format("15:04:05"),
		len(cotAnalyses),
		aiStatus,
	)

	_, err := h.bot.SendHTML(ctx, chatID, html)
	return err
}
