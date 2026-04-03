package telegram_test

import "testing"

// knownNavActions must stay in sync with all nav: action cases in cbNav (handler_calendar_cmd.go).
// Add new entries here AND in cbNav when adding keyboard buttons with "nav:XYZ" callbacks.
var knownNavActions = []string{
	"home",
	"cot",
}

// knownCmdActions must stay in sync with all cmd: action cases in cbQuickCommand (handler_calendar_cmd.go).
// Add new entries here AND in cbQuickCommand when adding keyboard buttons with "cmd:XYZ" callbacks.
var knownCmdActions = []string{
	"bias", "macro", "rank", "calendar", "accuracy", "sentiment",
	"seasonal", "backtest", "price", "levels", "carry", "regime",
	"corr", "intraday", "garch", "hurst", "factors", "wfopt",
	"quant", "vp", "cot", "cta", "alpha", "gex",
	"impact", "outlook", "intermarket", "playbook", "transition",
	"cryptoalpha", "session",
}

// TestNavActionsDocumented is a living checklist of known nav: callback actions.
// It fails if the list is accidentally emptied — a signal to update it.
func TestNavActionsDocumented(t *testing.T) {
	t.Logf("Known nav actions: %v", knownNavActions)
	if len(knownNavActions) == 0 {
		t.Fatal("knownNavActions must not be empty — update handler_callback_coverage_test.go")
	}
}

// TestCmdActionsDocumented is a living checklist of known cmd: callback actions.
// It fails if the list is accidentally emptied — a signal to update it.
func TestCmdActionsDocumented(t *testing.T) {
	t.Logf("Known cmd actions: %v", knownCmdActions)
	if len(knownCmdActions) == 0 {
		t.Fatal("knownCmdActions must not be empty — update handler_callback_coverage_test.go")
	}
}
