package telegram

import (
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Onboarding — Role Selector + Starter Kits
// ---------------------------------------------------------------------------

// OnboardingRoleMenu builds the experience-level selector for new users.
func (kb *KeyboardBuilder) OnboardingRoleMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "🌱 Pemula", CallbackData: "onboard:beginner"},
			},
			{
				{Text: "📈 Intermediate", CallbackData: "onboard:intermediate"},
			},
			{
				{Text: "🏛 Pro / Institutional", CallbackData: "onboard:pro"},
			},
		},
	}
}

// StarterKitMenu returns a role-appropriate starter keyboard.
func (kb *KeyboardBuilder) StarterKitMenu(level string) ports.InlineKeyboard {
	switch level {
	case "beginner":
		return ports.InlineKeyboard{
			Rows: [][]ports.InlineButton{
				{
					{Text: "📊 COT (Posisi Big Player)", CallbackData: "nav:cot"},
					{Text: "📅 Kalender Ekonomi", CallbackData: "cmd:calendar"},
				},
				{
					{Text: "💹 Cek Harga", CallbackData: "cmd:price"},
					{Text: "📈 Ranking Mata Uang", CallbackData: "cmd:rank"},
				},
				{
					{Text: "📖 Lihat Semua Command", CallbackData: "onboard:showhelp"},
				},
			},
		}
	case "intermediate":
		return ports.InlineKeyboard{
			Rows: [][]ports.InlineButton{
				{
					{Text: "📊 COT Analysis", CallbackData: "nav:cot"},
					{Text: "🦅 AI Outlook", CallbackData: "out:unified"},
				},
				{
					{Text: "📉 CTA Dashboard", CallbackData: "cmd:cta"},
					{Text: "🔬 Quant Analysis", CallbackData: "cmd:quant"},
				},
				{
					{Text: "🏦 Macro Regime", CallbackData: "cmd:macro"},
					{Text: "📊 Bias", CallbackData: "cmd:bias"},
				},
				{
					{Text: "📖 Lihat Semua Command", CallbackData: "onboard:showhelp"},
				},
			},
		}
	default: // pro
		return ports.InlineKeyboard{
			Rows: [][]ports.InlineButton{
				{
					{Text: "⚡ Alpha Engine", CallbackData: "alpha:back"},
					{Text: "🦅 AI Outlook", CallbackData: "out:unified"},
				},
				{
					{Text: "📊 Volume Profile", CallbackData: "cmd:vp"},
					{Text: "🔬 Quant", CallbackData: "cmd:quant"},
				},
				{
					{Text: "📉 CTA + Backtest", CallbackData: "cmd:cta"},
					{Text: "🏦 Macro", CallbackData: "cmd:macro"},
				},
				{
					{Text: "📖 Lihat Semua Command", CallbackData: "onboard:showhelp"},
				},
			},
		}
	}
}
