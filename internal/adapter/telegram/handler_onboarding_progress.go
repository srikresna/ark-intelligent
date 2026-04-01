package telegram

// Onboarding completion tracking — guided 4-step progression (TASK-204).
//
// Steps:
//   0 → /start sent (role selector shown)
//   1 → role chosen (via cbOnboard)
//   2 → first analysis command executed
//   3 → /settings opened
//   4 → second distinct feature explored → complete
//
// A single-line progress hint is sent after each command until the user
// reaches step 4 or explicitly dismisses via the "Skip" button.

import (
	"context"
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// onboardingCommands is the set of commands that count as "analysis features"
// for onboarding step advancement (steps 2 and 4).
var onboardingCommands = map[string]bool{
	"/cot": true, "/outlook": true, "/calendar": true, "/rank": true,
	"/macro": true, "/bias": true, "/backtest": true, "/sentiment": true,
	"/seasonal": true, "/price": true, "/levels": true, "/intermarket": true,
	"/cta": true, "/quant": true, "/vp": true, "/alpha": true,
	"/gex": true, "/ict": true, "/wyckoff": true, "/smc": true,
	"/elliott": true, "/impact": true, "/history": true, "/orderflow": true,
	"/treasury": true, "/ecb": true, "/snb": true, "/eurostat": true,
	"/signal": true, "/onchain": true,
	// shortcuts
	"/c": true, "/cal": true, "/out": true, "/m": true, "/b": true,
	"/q": true, "/bt": true, "/r": true, "/s": true, "/p": true,
	"/l": true, "/h": true, "/ce": true, "/ca": true, "/qe": true,
	"/bta": true, "/of": true,
}

// onboardingFeatureGroup maps commands to feature groups so that step 4
// requires a *different* feature than the one used at step 2.
var onboardingFeatureGroup = map[string]string{
	"/cot": "cot", "/c": "cot", "/ce": "cot",
	"/outlook": "outlook", "/out": "outlook", "/of": "outlook",
	"/calendar": "calendar", "/cal": "calendar",
	"/rank": "rank", "/r": "rank",
	"/macro": "macro", "/m": "macro",
	"/bias": "bias", "/b": "bias",
	"/backtest": "backtest", "/bt": "backtest", "/bta": "backtest",
	"/sentiment": "sentiment", "/s": "sentiment",
	"/seasonal": "seasonal",
	"/price": "price", "/p": "price",
	"/levels": "levels", "/l": "levels",
	"/intermarket": "intermarket",
	"/cta": "cta", "/ca": "cta",
	"/quant": "quant", "/q": "quant", "/qe": "quant",
	"/vp": "vp",
	"/alpha": "alpha",
	"/gex": "gex",
	"/ict": "ict",
	"/wyckoff": "wyckoff",
	"/smc": "smc",
	"/elliott": "elliott",
	"/impact": "impact",
	"/history": "history", "/h": "history",
	"/orderflow": "orderflow",
	"/treasury": "treasury",
	"/ecb": "ecb",
	"/snb": "snb",
	"/eurostat": "eurostat",
	"/signal": "signal",
	"/onchain": "onchain",
	"/settings": "settings",
}

// registerOnboardingProgress wires the post-command hook and skip callback.
func (h *Handler) registerOnboardingProgress() {
	h.bot.SetPostCommandHook(h.onboardingPostCommand)
	h.bot.RegisterCallback("onboard_prog:", h.cbOnboardProgress)
}

// onboardingPostCommand is called after every successful command.
// It advances the onboarding step and sends a progress hint if incomplete.
func (h *Handler) onboardingPostCommand(ctx context.Context, chatID string, userID int64, cmd string) {
	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		return
	}

	// Already complete or dismissed — nothing to do.
	if prefs.OnboardingStep >= 4 || prefs.OnboardingDismissed {
		return
	}

	// Existing users who have ExperienceLevel set but OnboardingStep==0
	// are pre-existing users — mark them complete so they don't see hints.
	if prefs.ExperienceLevel != "" && prefs.OnboardingStep == 0 {
		// If they haven't run any analysis commands yet (new onboarding),
		// set them to step 1 (role chosen). But for truly old users,
		// we check: if they already had ExperienceLevel before this feature
		// was deployed, they should be marked complete.
		// Heuristic: if OnboardingStep == 0 and OnboardingDismissed == false
		// and ExperienceLevel != "", this is a legacy user → auto-complete.
		prefs.OnboardingStep = 4
		_ = h.prefsRepo.Set(ctx, userID, prefs)
		return
	}

	// Advance step based on the command that was just executed.
	advanced := false

	switch {
	case prefs.OnboardingStep == 1 && cmd == "/settings":
		// Step 1→3: settings opened (skip step 2 — they went to settings first)
		prefs.OnboardingStep = 3
		prefs.OnboardingFirstFeature = ""
		advanced = true

	case prefs.OnboardingStep == 1 && onboardingCommands[cmd]:
		// Step 1→2: first analysis command
		prefs.OnboardingStep = 2
		prefs.OnboardingFirstFeature = onboardingFeatureGroup[cmd]
		advanced = true

	case prefs.OnboardingStep == 2 && cmd == "/settings":
		// Step 2→3: settings opened
		prefs.OnboardingStep = 3
		advanced = true

	case prefs.OnboardingStep == 2 && onboardingCommands[cmd]:
		// Check if it's a different feature group → step 2→4
		group := onboardingFeatureGroup[cmd]
		if group != prefs.OnboardingFirstFeature && group != "" {
			prefs.OnboardingStep = 4
			advanced = true
		}

	case prefs.OnboardingStep == 3 && onboardingCommands[cmd]:
		// Step 3→4: explore a feature after settings
		prefs.OnboardingStep = 4
		advanced = true
	}

	if advanced {
		_ = h.prefsRepo.Set(ctx, userID, prefs)
	}

	// Send progress hint if not yet complete and not dismissed.
	if prefs.OnboardingStep > 0 && prefs.OnboardingStep < 4 && !prefs.OnboardingDismissed {
		h.sendOnboardingHint(ctx, chatID, prefs)
	}
}

// sendOnboardingHint sends a single-line onboarding progress indicator.
func (h *Handler) sendOnboardingHint(ctx context.Context, chatID string, prefs domain.UserPrefs) {
	step := prefs.OnboardingStep

	// Build progress bar
	checks := [4]string{"⬜", "⬜", "⬜", "⬜"}
	labels := [4]string{"Role", "First cmd", "Settings", "Explore"}
	for i := 0; i < 4; i++ {
		if i < step {
			checks[i] = "✅"
		}
	}
	progress := fmt.Sprintf("%s%s %s%s %s%s %s%s",
		checks[0], labels[0],
		checks[1], labels[1],
		checks[2], labels[2],
		checks[3], labels[3],
	)

	// Contextual hint based on current step
	var hint string
	switch step {
	case 1:
		hint = "→ Coba jalankan command pertamamu! Ketik <code>/cot EUR</code> atau <code>/price EUR</code>"
	case 2:
		hint = "→ Buka <code>/settings</code> untuk kustomisasi alerts"
	case 3:
		hint = "→ Eksplorasi fitur lain! Coba <code>/macro</code> atau <code>/calendar</code>"
	}

	text := fmt.Sprintf("💡 <b>Onboarding (%d/4):</b> %s\n   %s", step, progress, hint)

	kb := ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "✖ Lewati Onboarding", CallbackData: "onboard_prog:skip"},
			},
		},
	}

	_, _ = h.bot.SendWithKeyboard(ctx, chatID, text, kb)
}

// cbOnboardProgress handles the "skip" button for onboarding progress.
func (h *Handler) cbOnboardProgress(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// Only action is "skip"
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	prefs.OnboardingDismissed = true
	_ = h.prefsRepo.Set(ctx, userID, prefs)

	// Delete the hint message
	_ = h.bot.DeleteMessage(ctx, chatID, msgID)

	// Toast confirmation
	return nil
}
