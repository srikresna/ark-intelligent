package telegram

import (
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Tutorial System — TASK-001-EXT
// ---------------------------------------------------------------------------

// TutorialStep represents a single step in the interactive tutorial.
type TutorialStep struct {
	Title   string
	Content string
}

// RoleConfig defines the complete onboarding experience for each role.
type RoleConfig struct {
	Name          string
	Description   string
	StarterKit    []string // command names
	TutorialSteps []TutorialStep
}

// RoleConfigs maps each experience level to its configuration.
var RoleConfigs = map[string]RoleConfig{
	"beginner": {
		Name:        "🌱 Trader Pemula",
		Description: "Saya baru memulai trading dan ingin belajar dasar-dasarnya",
		StarterKit:  []string{"/help", "/start", "/settings", "/changelog"},
		TutorialSteps: []TutorialStep{
			{
				Title:   "📚 Langkah 1: Perintah Dasar",
				Content: "<b>/help</b> menampilkan semua perintah yang tersedia dengan penjelasan singkat.\n\n<b>/start</b> menampilkan menu utama dan starter kit.\n\nCoba ketik <code>/help</code> kapan saja untuk melihat referensi.",
			},
			{
				Title:   "⚙️ Langkah 2: Pengaturan",
				Content: "<b>/settings</b> mengatur preferensi model AI dan bahasa.\n\n<b>/changelog</b> melihat update terbaru dan fitur baru.\n\nAtur preferensi di awal untuk pengalaman optimal.",
			},
			{
				Title:   "🚀 Langkah 3: Mulai Trading",
				Content: "Selamat! Kamu sudah siap menggunakan ARK Intelligence.\n\nGunakan starter kit di bawah untuk mulai analisis pertama kamu.\n\n💡 <i>Tip: Gunakan /help kapan saja untuk melihat referensi perintah.</i>",
			},
		},
	},
	"intermediate": {
		Name:        "📊 Trader Intermediate",
		Description: "Saya sudah punya pengalaman dan ingin tools analisis",
		StarterKit:  []string{"/cot", "/macro", "/signal", "/help"},
		TutorialSteps: []TutorialStep{
			{
				Title:   "📈 Langkah 1: COT Analysis",
				Content: "<b>/cot</b> menampilkan Commitment of Traders data — posisi big player di pasar.\n\nGunakan untuk melihat sentiment institusional terhadap mata uang tertentu.\n\n💡 <i>Tip: Coba <code>/cot EUR</code> untuk analisis Euro.</i>",
			},
			{
				Title:   "🌍 Langkah 2: Macro Intel",
				Content: "<b>/macro</b> menampilkan kalender ekonomi dan data FRED.\n\n<b>/signal</b> memberikan sinyal trading berdasarkan COT + CTA + Quant.\n\nKombinasi data makro dan teknikal untuk analisis lengkap.",
			},
			{
				Title:   "🎯 Langkah 3: Eksekusi Trading",
				Content: "Selamat! Kamu sudah menguasai tools utama ARK Intelligence.\n\nEksplorasi fitur lain seperti /quant, /cta, dan /outlook untuk analisis lebih dalam.\n\n💡 <i>Tip: Gunakan /backtest untuk menguji strategi trading.</i>",
			},
		},
	},
	"pro": {
		Name:        "🎯 Trader Pro",
		Description: "Saya butuh data institusional dan analisis kuantitatif",
		StarterKit:  []string{"/cot", "/macro", "/quant", "/impact", "/accuracy", "/backtest"},
		TutorialSteps: []TutorialStep{
			{
				Title:   "🔬 Langkah 1: Quant Analysis",
				Content: "<b>/quant</b> menghasilkan laporan kuantitatif lengkap dengan statistik, GARCH, dan regime detection.\n\n<b>/impact</b> melihat dampak event ekonomi historis.\n\nData-driven analysis untuk keputusan objektif.",
			},
			{
				Title:   "📊 Langkah 2: Backtest & Akurasi",
				Content: "<b>/backtest</b> menguji strategi trading dengan data historis.\n\n<b>/accuracy</b> melihat statistik akurasi sinyal sistem.\n\nGunakan untuk validasi edge trading sebelum live execution.",
			},
			{
				Title:   "⚡ Langkah 3: Alpha Engine",
				Content: "Selamat! Kamu sudah siap menggunakan fitur pro ARK Intelligence.\n\nEksplorasi /alpha, /gex, /elliott, dan /wyckoff untuk edge kompetitif.\n\n💡 <i>Tip: Setup /setalert untuk notifikasi real-time pada event penting.</i>",
			},
		},
	},
}

// GetRoleConfig returns the RoleConfig for the given experience level.
// Defaults to beginner if not found.
func GetRoleConfig(level string) RoleConfig {
	if cfg, ok := RoleConfigs[level]; ok {
		return cfg
	}
	return RoleConfigs["beginner"]
}

// TutorialWelcomeKeyboard shows the initial prompt to start tutorial.
func (kb *KeyboardBuilder) TutorialWelcomeKeyboard(role string) ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📖 Mulai Tutorial", CallbackData: "tutorial:start:" + role},
				{Text: "⏭️ Lewati", CallbackData: "tutorial:skip"},
			},
		},
	}
}

// TutorialStepKeyboard builds navigation for a tutorial step.
// step is 0-indexed, totalSteps is the total number of steps.
func (kb *KeyboardBuilder) TutorialStepKeyboard(role string, step, totalSteps int) ports.InlineKeyboard {
	var buttons []ports.InlineButton

	// Add Back button if not on first step
	if step > 0 {
		buttons = append(buttons, ports.InlineButton{
			Text:         "◀ Kembali",
			CallbackData: fmt.Sprintf("tutorial:step:%s:%d", role, step-1),
		})
	}

	// Add progress indicator (not clickable)
	buttons = append(buttons, ports.InlineButton{
		Text:         fmt.Sprintf("%d/%d", step+1, totalSteps),
		CallbackData: "tutorial:nop", // no-op
	})

	// Add Next or Done button
	if step < totalSteps-1 {
		buttons = append(buttons, ports.InlineButton{
			Text:         "Lanjut ▶",
			CallbackData: fmt.Sprintf("tutorial:step:%s:%d", role, step+1),
		})
	} else {
		buttons = append(buttons, ports.InlineButton{
			Text:         "✅ Selesai",
			CallbackData: "tutorial:done:" + role,
		})
	}

	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{buttons},
	}
}

// ---------------------------------------------------------------------------
// Onboarding — Role Selector + Starter Kits (Existing)
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
