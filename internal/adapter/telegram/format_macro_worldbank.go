package telegram

import (
	"fmt"
	"strings"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// FormatWorldBankFundamentals formats the World Bank global macro fundamentals section.
// Suitable for appending to /macro global view.
func (f *Formatter) FormatWorldBankFundamentals(wb *fred.WorldBankData) string {
	if wb == nil || !wb.Available || len(wb.Countries) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n🌍 <b>GLOBAL FUNDAMENTALS</b> <i>(World Bank, latest annual)</i>\n")

	// Display in consistent currency order
	order := []string{"USD", "EUR", "GBP", "JPY", "AUD", "NZD", "CAD", "CHF"}

	for _, currency := range order {
		cm, ok := wb.Countries[currency]
		if !ok {
			continue
		}

		var parts []string

		if cm.GDPGrowthYoY != 0 {
			parts = append(parts, fmt.Sprintf("GDP %+.1f%%", cm.GDPGrowthYoY))
		}
		if cm.CurrentAccount != 0 {
			parts = append(parts, fmt.Sprintf("CA %+.1f%% GDP", cm.CurrentAccount))
		}
		if cm.InflationCPI != 0 {
			parts = append(parts, fmt.Sprintf("CPI %+.1f%%", cm.InflationCPI))
		}

		if len(parts) == 0 {
			continue
		}

		yearStr := ""
		if cm.Year > 0 {
			yearStr = fmt.Sprintf(" (%d)", cm.Year)
		}

		b.WriteString(fmt.Sprintf("<b>%s</b>%s: %s\n", currency, yearStr, strings.Join(parts, " | ")))
	}

	b.WriteString(fmt.Sprintf("<i>Source: World Bank API • %s</i>\n",
		fmtutil.FormatDateTimeWIB(wb.FetchedAt)))

	return b.String()
}
