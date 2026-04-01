package telegram

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/pkg/format"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

type cotGroup struct {
	Header    string
	Emoji     string
	Codes     []string // contract codes in preferred order
}

var cotGroups = []cotGroup{
	{
		Header: "FOREX MAJORS",
		Emoji:  "🌍",
		Codes:  []string{"098662", "099741", "096742", "097741", "232741", "112741", "090741", "092741"},
	},
	{
		Header: "EQUITY INDICES",
		Emoji:  "📈",
		Codes:  []string{"13874A", "209742", "124601", "239742"},
	},
	{
		Header: "COMMODITIES",
		Emoji:  "🏅",
		Codes:  []string{"088691", "084691", "085692", "067651", "022651", "111659"},
	},
	{
		Header: "BONDS",
		Emoji:  "📊",
		Codes:  []string{"042601", "044601", "043602", "020601"},
	},
	{
		Header: "CRYPTO",
		Emoji:  "₿",
		Codes:  []string{"133741", "146021"},
	},
}

type rankEntry struct {
	Currency string
	Score    float64
	COTIndex float64
}

type convictionRankEntry struct {
	Currency        string
	Score           float64
	COTIndex        float64
	Conviction      cot.ConvictionScore
	ThinMarketAlert bool
	ThinMarketDesc  string
}

// cotIdxLabel returns a short label for a COT Index value.
func cotIdxLabel(idx float64) string {
	switch {
	case idx >= 80:
		return "X.Long"
	case idx >= 60:
		return "Bullish"
	case idx >= 40:
		return "Neutral"
	case idx >= 20:
		return "Bearish"
	default:
		return "X.Short"
	}
}

// convictionMiniBar renders a small conviction bar: e.g. "▓▓▓░░ 62"
func convictionMiniBar(score float64, dir string) string {
	filled := int(score / 20) // 0-5 blocks
	if filled > 5 {
		filled = 5
	}
	bar := strings.Repeat("▓", filled) + strings.Repeat("░", 5-filled)
	icon := "⚪"
	switch {
	case score >= 65 && dir == "LONG":
		icon = "🟢"
	case score >= 65 && dir == "SHORT":
		icon = "🔴"
	case score >= 55:
		icon = "🟡"
	}
	return fmt.Sprintf("%s[%s]%.0f", icon, bar, score)
}

// FormatCOTOverview formats a grouped, sorted summary of all COT analyses.
// convictions may be nil — conviction column is hidden gracefully.
func (f *Formatter) FormatCOTOverview(analyses []domain.COTAnalysis, convictions []cot.ConvictionScore) string {
	var b strings.Builder

	// Build lookup maps
	byCode := make(map[string]domain.COTAnalysis, len(analyses))
	for _, a := range analyses {
		byCode[a.Contract.Code] = a
	}
	convMap := make(map[string]cot.ConvictionScore, len(convictions))
	for _, cs := range convictions {
		convMap[cs.Currency] = cs
	}
	shown := make(map[string]bool)

	b.WriteString("📋 <b>COT POSITIONING OVERVIEW</b>\n")
	if len(analyses) > 0 {
		b.WriteString(fmt.Sprintf("<i>Report: %s</i>\n", analyses[0].ReportDate.Format("Jan 2, 2006")))
	}
	hasConv := len(convictions) > 0
	if hasConv {
		b.WriteString("<i>Conv = Conviction Score (COT+FRED+Price)</i>\n")
	}
	b.WriteString("\n")

	for _, grp := range cotGroups {
		// Collect analyses for this group
		var grpAnalyses []domain.COTAnalysis
		for _, code := range grp.Codes {
			if a, ok := byCode[code]; ok {
				grpAnalyses = append(grpAnalyses, a)
				shown[code] = true
			}
		}
		if len(grpAnalyses) == 0 {
			continue
		}

		// Sort within group by COT Index descending (strongest conviction first)
		sort.Slice(grpAnalyses, func(i, j int) bool {
			// Use conviction score if available, otherwise COT Index
			ci, ciOk := convMap[grpAnalyses[i].Contract.Currency]
			cj, cjOk := convMap[grpAnalyses[j].Contract.Currency]
			if ciOk && cjOk {
				return ci.Score > cj.Score
			}
			return grpAnalyses[i].COTIndex > grpAnalyses[j].COTIndex
		})

		b.WriteString(fmt.Sprintf("%s <b>%s</b>\n", grp.Emoji, grp.Header))

		for _, a := range grpAnalyses {
			bias := "NEUTRAL"
			biasIcon := "⚪"
			if a.NetPosition > 0 {
				bias = "LONG"
				biasIcon = "🟢"
			} else if a.NetPosition < 0 {
				bias = "SHORT"
				biasIcon = "🔴"
			}

			idxLbl := cotIdxLabel(a.COTIndex)

			// Line 1: name + bias
			b.WriteString(fmt.Sprintf("%s <b>%s</b> %s\n", biasIcon, a.Contract.Name, bias))

			// Line 2: Net | Idx | Conv (if available)
			if cs, ok := convMap[a.Contract.Currency]; ok {
				b.WriteString(fmt.Sprintf("<code>  Net:%-10s Idx:%.0f%% (%s)</code>\n",
					format.FormatInt(int64(a.NetPosition)), a.COTIndex, idxLbl))
				b.WriteString(fmt.Sprintf("<code>  Chg:%-10s Mom:%-10s Conv:%s</code>\n",
					fmtutil.FmtNumSigned(a.NetChange, 0),
					f.momentumLabel(a.MomentumDir),
					convictionMiniBar(cs.Score, cs.Direction)))
			} else {
				b.WriteString(fmt.Sprintf("<code>  Net:%-10s Idx:%.0f%% (%s)</code>\n",
					format.FormatInt(int64(a.NetPosition)), a.COTIndex, idxLbl))
				b.WriteString(fmt.Sprintf("<code>  Chg:%-10s Mom:%s</code>\n",
					fmtutil.FmtNumSigned(a.NetChange, 0),
					f.momentumLabel(a.MomentumDir)))
			}
			b.WriteString("\n")
		}
	}

	// Catch-all: any analyses not in a group (future contracts)
	var ungrouped []domain.COTAnalysis
	for _, a := range analyses {
		if !shown[a.Contract.Code] {
			ungrouped = append(ungrouped, a)
		}
	}
	if len(ungrouped) > 0 {
		b.WriteString("📌 <b>OTHER</b>\n")
		for _, a := range ungrouped {
			bias := "NEUTRAL"
			if a.NetPosition > 0 {
				bias = "LONG"
			} else if a.NetPosition < 0 {
				bias = "SHORT"
			}
			b.WriteString(fmt.Sprintf("<b>%s</b> %s\n", a.Contract.Name, bias))
			b.WriteString(fmt.Sprintf("<code>  Net: %s | Idx: %.0f%%</code>\n\n",
				format.FormatInt(int64(a.NetPosition)), a.COTIndex))
		}
	}

	b.WriteString("<i>Tap a currency for detailed breakdown</i>\n")
	b.WriteString("<i>Tip: </i><code>/cot USD</code> | <code>/cot raw EUR</code> | <code>/cot GBP</code>")
	return b.String()
}

// FormatCOTDetail formats detailed COT analysis for one contract.
// Signature unchanged for backward compatibility.
func (f *Formatter) FormatCOTDetail(a domain.COTAnalysis) string {
	return f.FormatCOTDetailWithCode(a, "")
}

// FormatCOTDetailWithCode formats detailed COT analysis and appends quick-copy commands.
func (f *Formatter) FormatCOTDetailWithCode(a domain.COTAnalysis, displayCode string) string {
	var b strings.Builder

	rt := a.Contract.ReportType
	smartMoneyLabel := "Speculator"
	hedgerLabel := "Hedger"
	if rt == "TFF" {
		smartMoneyLabel = "Lev Funds"
		hedgerLabel = "Dealers"
	} else if rt == "DISAGGREGATED" {
		smartMoneyLabel = "Managed Money"
		hedgerLabel = "Prod/Swap"
	}

	b.WriteString(fmt.Sprintf("<b>COT Analysis: %s</b>\n", a.Contract.Name))
	b.WriteString(fmt.Sprintf("<i>Report: %s (%s)</i>\n\n", a.ReportDate.Format("Jan 2, 2006"), rt))

	// Alerts section — all warnings first
	if a.AssetMgrAlert {
		b.WriteString(fmt.Sprintf("⚠️ <b>Asset Manager Structural Shift!</b> (Z-Score: %.2f)\n", a.AssetMgrZScore))
	}
	if a.ThinMarketAlert {
		b.WriteString(fmt.Sprintf("🚨 <b>THIN MARKET:</b> %s\n", a.ThinMarketDesc))
	}
	if a.SmartDumbDivergence {
		if rt == "DISAGGREGATED" {
			// Untuk komoditas, divergence antara Managed Money dan Prod/Swap adalah NORMAL
			// karena produsen selalu hedge (net short) sementara spekulan beli
			b.WriteString("🔀 <b>Divergence:</b> Spekulan vs produsen posisi berlawanan\n")
			b.WriteString("<i>  ℹ️ Untuk komoditas, ini NORMAL — produsen biasanya selalu hedge net short</i>\n")
		} else {
			b.WriteString("🔀 <b>Divergence:</b> Smart money vs commercials moving opposite\n")
		}
	}
	if a.CommExtremeBull {
		b.WriteString("🟢 <b>Commercial COT Extreme LONG</b> (contrarian bullish signal)\n")
	}
	if a.CommExtremeBear {
		b.WriteString("🔴 <b>Commercial COT Extreme SHORT</b> (contrarian bearish signal)\n")
	}
	if a.CategoryDivergence {
		b.WriteString(fmt.Sprintf("⚡ <b>Category Divergence:</b> %s\n", a.CategoryDivergenceDesc))
	}
	if a.AssetMgrAlert || a.ThinMarketAlert || a.SmartDumbDivergence || a.CommExtremeBull || a.CommExtremeBear || a.CategoryDivergence {
		b.WriteString("\n")
	}

	// Category Z-Score Breakdown (show if any alert or divergence exists)
	hasZAlert := a.DealerAlert || a.LevFundAlert || a.ManagedMoneyAlert || a.SwapDealerAlert || a.CategoryDivergence
	if hasZAlert {
		b.WriteString("📊 <b>Category Z-Scores (WoW Change vs 52W):</b>\n")
		zScoreEmoji := func(z float64, alert bool) string {
			if !alert {
				return "  "
			}
			if z > 0 {
				return "🟢"
			}
			return "🔴"
		}
		if rt == "TFF" {
			b.WriteString(fmt.Sprintf("<code>  Dealer:       %+.2fσ %s</code>\n", a.DealerZScore, zScoreEmoji(a.DealerZScore, a.DealerAlert)))
			b.WriteString(fmt.Sprintf("<code>  LevFund:      %+.2fσ %s</code>\n", a.LevFundZScore, zScoreEmoji(a.LevFundZScore, a.LevFundAlert)))
			b.WriteString(fmt.Sprintf("<code>  AssetMgr:     %+.2fσ %s</code>\n", a.AssetMgrZScore, zScoreEmoji(a.AssetMgrZScore, a.AssetMgrAlert)))
			b.WriteString(fmt.Sprintf("<code>  ManagedMoney: %+.2fσ %s</code>\n", a.ManagedMoneyZScore, zScoreEmoji(a.ManagedMoneyZScore, a.ManagedMoneyAlert)))
		} else {
			// DISAGGREGATED: SwapDealer and ManagedMoney are primary
			b.WriteString(fmt.Sprintf("<code>  ManagedMoney: %+.2fσ %s</code>\n", a.ManagedMoneyZScore, zScoreEmoji(a.ManagedMoneyZScore, a.ManagedMoneyAlert)))
			b.WriteString(fmt.Sprintf("<code>  SwapDealer:   %+.2fσ %s</code>\n", a.SwapDealerZScore, zScoreEmoji(a.SwapDealerZScore, a.SwapDealerAlert)))
			b.WriteString(fmt.Sprintf("<code>  LevFund:      %+.2fσ %s</code>\n", a.LevFundZScore, zScoreEmoji(a.LevFundZScore, a.LevFundAlert)))
		}
		b.WriteString(fmt.Sprintf("<i>  Alert threshold: |z| ≥ 2.0σ  |  max |z|: %.2f</i>\n",
			math.Max(math.Max(math.Abs(a.DealerZScore), math.Abs(a.LevFundZScore)),
				math.Max(math.Abs(a.ManagedMoneyZScore), math.Abs(a.SwapDealerZScore)))))
		b.WriteString("\n")
	}

	// Positioning
	b.WriteString(fmt.Sprintf("<b>%s (Smart Money):</b>\n", smartMoneyLabel))
	b.WriteString(fmt.Sprintf("<code>  Net Position:   %s</code>\n", format.FormatNetPosition(int64(a.NetPosition))))
	b.WriteString(fmt.Sprintf("<code>  Net Change:     %s</code>\n", fmtutil.FmtNumSigned(a.NetChange, 0)))
	if a.LongShortRatio >= 999 {
		b.WriteString("<code>  L/S Ratio:      ∞ (no shorts reported)</code>\n")
	} else if a.LongShortRatio == 0 {
		b.WriteString("<code>  L/S Ratio:      N/A (no positions)</code>\n")
	} else {
		b.WriteString(fmt.Sprintf("<code>  L/S Ratio:      %.2f</code>\n", a.LongShortRatio))
	}
	b.WriteString(fmt.Sprintf("<code>  Net as %% OI:    %.1f%%</code>\n", a.PctOfOI))

	b.WriteString(fmt.Sprintf("\n<b>%s:</b>\n", hedgerLabel))
	b.WriteString(fmt.Sprintf("<code>  Net Position:   %s</code>\n", fmtutil.FmtNumSigned(a.CommercialNet, 0)))
	b.WriteString(fmt.Sprintf("<code>  Comm %% OI:      %.1f%%</code>\n", a.CommPctOfOI))
	b.WriteString(fmt.Sprintf("<code>  COT Index:      %.1f%%</code>\n", a.COTIndexComm))
	b.WriteString(fmt.Sprintf("<code>  Signal:         %s</code>\n", commercialSignalLabel(a.CommercialSignal, rt)))

	// COT Index
	b.WriteString(fmt.Sprintf("\n<b>COT Index (%s):</b>\n", smartMoneyLabel))
	b.WriteString(fmt.Sprintf("<code>  52-Week:        %.1f%%</code>\n", a.COTIndex))
	b.WriteString(f.formatProgressBar(a.COTIndex, 20))

	// Momentum (4W + 8W)
	b.WriteString("\n<b>Momentum:</b>\n")
	b.WriteString(fmt.Sprintf("<code>  4W:             %s</code>\n", fmtutil.FmtNumSigned(a.SpecMomentum4W, 0)))
	if a.SpecMomentum8W != 0 {
		trendFilter := "✅ aligned"
		if (a.SpecMomentum4W > 0) != (a.SpecMomentum8W > 0) {
			trendFilter = "⚠️ opposing"
		}
		b.WriteString(fmt.Sprintf("<code>  8W:             %s (%s)</code>\n", fmtutil.FmtNumSigned(a.SpecMomentum8W, 0), trendFilter))
	}
	if a.ConsecutiveWeeks > 0 {
		b.WriteString(fmt.Sprintf("<code>  Streak:         %d weeks same dir</code>\n", a.ConsecutiveWeeks))
	}

	// Open Interest
	b.WriteString("\n<b>Open Interest:</b>\n")
	b.WriteString(fmt.Sprintf("<code>  OI Change:      %s (%s)</code>\n", fmtutil.FmtNumSigned(a.OpenInterestChg, 0), a.OITrend))
	if a.SpreadPctOfOI > 0 {
		b.WriteString(fmt.Sprintf("<code>  Spread Pos:     %.1f%% of OI</code>\n", a.SpreadPctOfOI))
	}

	// Trader concentration
	if a.TotalTraders > 0 {
		b.WriteString(fmt.Sprintf("\n<b>Trader Depth (%s):</b>\n", a.TraderConcentration))
		if rt == "TFF" {
			if a.LevFundLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  Lev Fund Long:  %d traders</code>\n", a.LevFundLongTraders))
			}
			if a.LevFundShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  Lev Fund Short: %d traders</code>\n", a.LevFundShortTraders))
			}
			if a.DealerShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  Dealer Short:   %d traders</code>\n", a.DealerShortTraders))
			}
			if a.AssetMgrLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  AssetMgr Long:  %d traders</code>\n", a.AssetMgrLongTraders))
			}
		} else {
			if a.MMoneyLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  MM Long:        %d traders</code>\n", a.MMoneyLongTraders))
			}
			if a.MMoneyShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  MM Short:       %d traders</code>\n", a.MMoneyShortTraders))
			}
		}
		b.WriteString(fmt.Sprintf("<code>  Total:          %d traders</code>\n", a.TotalTraders))
	}

	// Scalper Intel
	b.WriteString("\n<b>Scalper Intel:</b>\n")
	b.WriteString(fmt.Sprintf("<code>  ST Bias:        %s</code>\n", a.ShortTermBias))
	b.WriteString(fmt.Sprintf("<code>  Crowding:       %.0f/100</code>\n", a.CrowdingIndex))
	b.WriteString(fmt.Sprintf("<code>  Divergence:     %v</code>\n", a.DivergenceFlag))

	// Smart Money vs Commercial Signal Confluence
	b.WriteString("\n<b>Signal Confluence:</b>\n")
	b.WriteString(fmt.Sprintf("<code>  Smart Money:    %s</code>\n", a.SpeculatorSignal))
	b.WriteString(fmt.Sprintf("<code>  %s:   %s</code>\n", hedgerLabel, commercialSignalLabel(a.CommercialSignal, rt)))
	b.WriteString(signalConfluenceInterpretation(a.SpeculatorSignal, a.CommercialSignal, rt))

	// Quick copy commands — prefer currency code (e.g. GOLD, EUR) over contract code
	if displayCode != "" {
		// Map known contract codes back to friendly currency shortcuts
		friendlyCode := contractCodeToFriendly(displayCode)
		b.WriteString(fmt.Sprintf("\n<i>Quick commands:</i>\n<code>/cot %s</code> | <code>/cot raw %s</code>", friendlyCode, friendlyCode))
	}

	return b.String()
}

// FormatCOTRaw formats raw CFTC data with plain-language explanations and calculated metrics.
func (f *Formatter) FormatCOTRaw(r domain.COTRecord) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("📊 <b>DATA MENTAH COT: %s</b>\n", r.ContractName))
	b.WriteString(fmt.Sprintf("<i>Laporan: %s | Sumber: CFTC (resmi)</i>\n", r.ReportDate.Format("Jan 2, 2006")))
	b.WriteString("<i>Data ini adalah angka posisi asli sebelum dikalkulasi</i>\n\n")

	// Open Interest
	b.WriteString("📌 <b>OPEN INTEREST (Total Kontrak Aktif)</b>\n")
	b.WriteString(fmt.Sprintf("<code>  Total: %s kontrak</code>\n", fmtutil.FmtNum(r.OpenInterest, 0)))
	b.WriteString("<i>  → Semakin besar = semakin banyak uang yang aktif di pasar ini</i>\n\n")

	// Determine report type from contract code (reliable) instead of contract name (fragile).
	// Lookup DefaultCOTContracts to find the ReportType for this contract.
	isDisagg := false
	for _, c := range domain.DefaultCOTContracts {
		if c.Code == r.ContractCode {
			isDisagg = c.ReportType == "DISAGGREGATED"
			break
		}
	}
	// Fallback: if contract code not found, infer from field presence
	// (DISAGGREGATED records populate ManagedMoneyLong; TFF records populate LevFundLong)
	if r.ContractCode == "" {
		isDisagg = r.ManagedMoneyLong > 0 || r.ManagedMoneyShort > 0
	}

	if isDisagg {
		// ── DISAGGREGATED (Komoditas fisik: Gold, Oil, dll) ──
		mmLong := r.ManagedMoneyLong
		mmShort := r.ManagedMoneyShort
		mmNet := mmLong - mmShort
		var mmRatio float64
		if mmShort > 0 {
			mmRatio = mmLong / mmShort
		}
		mmNetIcon := "🟢"
		mmNetDesc := "NET BELI"
		if mmNet < 0 {
			mmNetIcon = "🔴"
			mmNetDesc = "NET JUAL"
		}

		b.WriteString("🧠 <b>MANAGED MONEY — Hedge Fund / Spekulan Besar</b>\n")
		b.WriteString("<i>  Siapa ini? Dana investasi besar yang mencari profit dari pergerakan harga</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Long (beli) : %s kontrak</code>\n", fmtutil.FmtNum(mmLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short (jual): %s kontrak</code>\n", fmtutil.FmtNum(mmShort, 0)))
		b.WriteString(fmt.Sprintf("<code>  Net         : %s%s (%s)</code>\n",
			func() string {
				if mmNet >= 0 {
					return "+"
				}
				return ""
			}(),
			fmtutil.FmtNum(mmNet, 0), mmNetDesc))
		if mmShort > 0 || mmLong > 0 {
			if mmRatio >= 1 {
				b.WriteString(fmt.Sprintf("<code>  Rasio L/S   : %.2fx lebih banyak BELI dari jual</code>\n", mmRatio))
			} else if mmShort > 0 && mmLong > 0 {
				b.WriteString(fmt.Sprintf("<code>  Rasio L/S   : %.2fx lebih banyak JUAL dari beli</code>\n", mmShort/mmLong))
			} else if mmLong == 0 && mmShort > 0 {
				b.WriteString("<code>  Rasio L/S   : seluruhnya posisi JUAL</code>\n")
			} else if mmShort == 0 && mmLong > 0 {
				b.WriteString("<code>  Rasio L/S   : seluruhnya posisi BELI</code>\n")
			}
		}
		b.WriteString(fmt.Sprintf("<i>  → Spekulan sedang %s %s — mereka %s harga naik</i>\n\n",
			mmNetIcon, mmNetDesc,
			func() string {
				if mmNet > 0 {
					return "EKSPEKTASI"
				}
				return "TIDAK ekspektasi"
			}()))

		// Commercials (Prod/Swap)
		commLong := r.ProdMercLong + r.SwapDealerLong
		commShort := r.ProdMercShort + r.SwapDealerShort
		commNet := commLong - commShort
		commNetDesc := "net beli"
		if commNet < 0 {
			commNetDesc = "net jual (hedge)"
		}

		b.WriteString("🏭 <b>PROD/SWAP — Produsen & Korporasi</b>\n")
		b.WriteString("<i>  Siapa ini? Perusahaan tambang, kilang minyak, bank komoditas</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Long (beli) : %s kontrak</code>\n", fmtutil.FmtNum(commLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short (jual): %s kontrak</code>\n", fmtutil.FmtNum(commShort, 0)))
		b.WriteString(fmt.Sprintf("<code>  Net         : %s%s (%s)</code>\n",
			func() string {
				if commNet >= 0 {
					return "+"
				}
				return ""
			}(),
			fmtutil.FmtNum(commNet, 0), commNetDesc))
		b.WriteString("<i>  → Produsen biasanya net SHORT untuk lindungi produksi mereka.\n")
		b.WriteString("     Ini NORMAL dan bukan sinyal bearish murni.</i>\n\n")

		// Perbandingan Specs vs Commercials
		b.WriteString("⚡ <b>BACAAN CEPAT</b>\n")
		if mmNet > 0 && commNet < 0 {
			b.WriteString("  ✅ Kondisi normal: spekulan beli, produsen hedge\n")
			if mmNet > 50000 {
				b.WriteString("  ⚠️ Spekulan sudah beli banyak — risiko pembalikan jika mereka mulai keluar\n")
			}
		} else if mmNet < 0 && commNet > 0 {
			b.WriteString("  🔀 Kondisi terbalik: spekulan jual, tapi produsen justru beli\n")
			b.WriteString("  → Ini sinyal langka — bisa jadi titik balik harga\n")
		} else if mmNet < 0 && commNet < 0 {
			b.WriteString("  🔴 Semua pihak net jual — tekanan turun signifikan\n")
		}

	} else {
		// ── TFF (Financial: Mata uang, Bonds, Indices) ──
		lfLong := r.LevFundLong
		lfShort := r.LevFundShort
		lfNet := lfLong - lfShort
		lfNetIcon := "🟢"
		lfNetDesc := "NET BELI"
		if lfNet < 0 {
			lfNetIcon = "🔴"
			lfNetDesc = "NET JUAL"
		}
		var lfRatio float64
		if lfShort > 0 {
			lfRatio = lfLong / lfShort
		}

		b.WriteString("⚡ <b>LEVERAGED FUNDS — Hedge Fund / CTA</b>\n")
		b.WriteString("<i>  Siapa ini? Dana spekulatif yang trading dengan leverage tinggi</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Long (beli) : %s kontrak</code>\n", fmtutil.FmtNum(lfLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short (jual): %s kontrak</code>\n", fmtutil.FmtNum(lfShort, 0)))
		b.WriteString(fmt.Sprintf("<code>  Net         : %s%s (%s)</code>\n",
			func() string {
				if lfNet >= 0 {
					return "+"
				}
				return ""
			}(),
			fmtutil.FmtNum(lfNet, 0), lfNetDesc))
		if lfRatio > 0 && lfShort > 0 {
			dominant := "beli"
			ratio := lfRatio
			if lfRatio < 1 {
				dominant = "jual"
				ratio = lfShort / lfLong
			}
			b.WriteString(fmt.Sprintf("<code>  Rasio L/S   : %.2fx lebih banyak %s</code>\n", ratio, dominant))
		}
		b.WriteString(fmt.Sprintf("<i>  → %s Hedge fund sedang %s — ini sinyal paling penting untuk arah harga</i>\n\n",
			lfNetIcon, lfNetDesc))

		// Asset Manager
		amLong := r.AssetMgrLong
		amShort := r.AssetMgrShort
		amNet := amLong - amShort
		amNetDesc := "net beli"
		if amNet < 0 {
			amNetDesc = "net jual"
		}

		b.WriteString("🏦 <b>ASSET MANAGER — Dana Pensiun & Reksa Dana</b>\n")
		b.WriteString("<i>  Siapa ini? Dana pensiun, reksa dana, asuransi — uang jangka panjang</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Long (beli) : %s kontrak</code>\n", fmtutil.FmtNum(amLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short (jual): %s kontrak</code>\n", fmtutil.FmtNum(amShort, 0)))
		b.WriteString(fmt.Sprintf("<code>  Net         : %s%s (%s)</code>\n",
			func() string {
				if amNet >= 0 {
					return "+"
				}
				return ""
			}(),
			fmtutil.FmtNum(amNet, 0), amNetDesc))
		b.WriteString("<i>  → Pergerakan Asset Manager lebih lambat tapi lebih sustained</i>\n\n")

		// Dealers
		dlrLong := r.DealerLong
		dlrShort := r.DealerShort
		dlrNet := dlrLong - dlrShort

		b.WriteString("🏛 <b>DEALERS — Bank Besar / Market Maker</b>\n")
		b.WriteString("<i>  Siapa ini? Bank investasi yang jadi perantara pasar</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Long (beli) : %s kontrak</code>\n", fmtutil.FmtNum(dlrLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short (jual): %s kontrak</code>\n", fmtutil.FmtNum(dlrShort, 0)))
		b.WriteString(fmt.Sprintf("<code>  Net         : %s%s</code>\n",
			func() string {
				if dlrNet >= 0 {
					return "+"
				}
				return ""
			}(),
			fmtutil.FmtNum(dlrNet, 0)))
		b.WriteString("<i>  → Dealer biasanya posisi berlawanan dengan Lev Funds (mereka sisi lain transaksi)</i>\n\n")

		// Bacaan cepat
		b.WriteString("⚡ <b>BACAAN CEPAT</b>\n")
		if lfNet > 0 && amNet > 0 {
			b.WriteString("  🟢 Hedge fund DAN asset manager sama-sama beli — sinyal naik kuat\n")
			if dlrNet < -50000 {
				b.WriteString("  ⚠️ Tapi bank/dealer net jual besar — mereka di sisi berlawanan, waspadai reversal\n")
			}
		} else if lfNet < 0 && amNet < 0 {
			b.WriteString("  🔴 Hedge fund DAN asset manager sama-sama jual — sinyal turun kuat\n")
			if dlrNet > 50000 {
				b.WriteString("  ⚠️ Tapi bank/dealer net beli besar — mereka di sisi berlawanan, waspadai reversal\n")
			}
		} else if lfNet > 0 && amNet < 0 {
			b.WriteString("  🟡 Hedge fund beli tapi asset manager jual — sinyal campur\n")
		} else if lfNet < 0 && amNet > 0 {
			b.WriteString("  🟡 Hedge fund jual tapi asset manager beli — sinyal campur\n")
		}
	}

	// Trader depth — selalu tampil dengan penjelasan
	b.WriteString("\n👥 <b>KEDALAMAN PASAR (Jumlah Trader Aktif)</b>\n")
	if isDisagg {
		totalT := r.TotalTradersDisag
		if totalT > 0 {
			b.WriteString(fmt.Sprintf("<code>  Spekulan Long : %d trader</code>\n", r.MMoneyLongTraders))
			b.WriteString(fmt.Sprintf("<code>  Spekulan Short: %d trader</code>\n", r.MMoneyShortTraders))
			b.WriteString(fmt.Sprintf("<code>  Total Aktif   : %d trader</code>\n", totalT))
			if r.MMoneyLongTraders > 0 && r.MMoneyShortTraders > 0 {
				ratio := float64(r.MMoneyLongTraders) / float64(r.MMoneyShortTraders)
				b.WriteString(fmt.Sprintf("<i>  → Rasio trader: %.1fx lebih banyak yang beli vs jual</i>\n", ratio))
			}
			depthLabel := "sedang"
			depthDesc := "likuiditas normal"
			if totalT > 300 {
				depthLabel = "DEEP (dalam)"
				depthDesc = "likuiditas bagus, mudah masuk/keluar posisi"
			} else if totalT < 100 {
				depthLabel = "TIPIS"
				depthDesc = "hati-hati — pasar tipis, slippage bisa besar"
			}
			b.WriteString(fmt.Sprintf("<i>  → Pasar %s — %s</i>\n", depthLabel, depthDesc))
		}
	} else {
		totalT := r.TotalTraders
		if totalT > 0 {
			if r.LevFundLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  Lev Fund Long : %d trader</code>\n", r.LevFundLongTraders))
			}
			if r.LevFundShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  Lev Fund Short: %d trader</code>\n", r.LevFundShortTraders))
			}
			if r.AssetMgrLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  AssetMgr Long : %d trader</code>\n", r.AssetMgrLongTraders))
			}
			if r.AssetMgrShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  AssetMgr Short: %d trader</code>\n", r.AssetMgrShortTraders))
			}
			b.WriteString(fmt.Sprintf("<code>  Total Aktif   : %d trader</code>\n", totalT))
			depthLabel := "sedang"
			depthDesc := "likuiditas normal"
			if totalT > 300 {
				depthLabel = "DEEP (dalam)"
				depthDesc = "likuiditas bagus"
			} else if totalT < 80 {
				depthLabel = "TIPIS"
				depthDesc = "hati-hati — pasar tipis"
			}
			b.WriteString(fmt.Sprintf("<i>  → Pasar %s — %s</i>\n", depthLabel, depthDesc))
		}
	}

	// Small Specs jika ada
	if r.SmallLong > 0 || r.SmallShort > 0 {
		smallNet := r.SmallLong - r.SmallShort
		b.WriteString("\n🐟 <b>SMALL SPECULATORS — Trader Retail Kecil</b>\n")
		b.WriteString(fmt.Sprintf("<code>  Long : %s | Short: %s | Net: %s%s</code>\n",
			fmtutil.FmtNum(r.SmallLong, 0),
			fmtutil.FmtNum(r.SmallShort, 0),
			func() string {
				if smallNet >= 0 {
					return "+"
				}
				return ""
			}(),
			fmtutil.FmtNum(smallNet, 0)))
		b.WriteString("<i>  → Retail trader — sering dianggap 'wrong-side' oleh institusi</i>\n")
	}

	b.WriteString("\n<i>📌 Data resmi dari CFTC, dirilis setiap Jumat untuk data Selasa sebelumnya</i>")
	return b.String()
}

// FormatRanking formats the weekly currency strength ranking based on COT sentiment scores.
// P1.3 — /rank command output.
func (f *Formatter) FormatRanking(analyses []domain.COTAnalysis, date time.Time) string {
	var b strings.Builder

	// Filter to 8 major currencies only (no commodities)
	majors := map[string]bool{"EUR": true, "GBP": true, "JPY": true, "AUD": true,
		"NZD": true, "CAD": true, "CHF": true, "USD": true}

	var entries []rankEntry
	for _, a := range analyses {
		if !majors[a.Contract.Currency] {
			continue
		}
		entries = append(entries, rankEntry{
			Currency: a.Contract.Currency,
			Score:    a.SentimentScore,
			COTIndex: a.COTIndex,
		})
	}

	// Sort by sentiment score descending (strongest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score > entries[j].Score
	})

	b.WriteString("🏆 <b>Currency Strength Ranking</b>\n")
	b.WriteString(fmt.Sprintf("<i>Week of %s | Based on COT Positioning</i>\n\n", fmtutil.FormatDateWIB(date)))

	medals := []string{"🥇", "🥈", "🥉", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣"}

	for i, e := range entries {
		medal := ""
		if i < len(medals) {
			medal = medals[i]
		}

		arrow := scoreArrow(e.Score)
		colorDot := scoreDot(e.Score)
		label := cotLabel(e.COTIndex)

		signStr := "+"
		if e.Score < 0 {
			signStr = ""
		}

		b.WriteString(fmt.Sprintf("%s %s %s: <b>%s%.0f %s</b>  <i>(%s)</i>\n",
			medal, colorDot, e.Currency, signStr, e.Score, arrow, label))
	}

	// Best pairs: top 3 spread combinations
	b.WriteString("\n📊 <b>Best Pairs:</b>\n")
	pairs := buildBestPairs(entries)
	for _, p := range pairs {
		b.WriteString(p + "\n")
	}

	b.WriteString("\n<i>Tip: </i><code>/cot GBP</code> untuk detail lengkap")
	return b.String()
}

// FormatRankingWithConviction formats the weekly currency strength ranking with unified
// conviction scores from COT + FRED regime + calendar data.
// Gap D — exposes ConvictionScore per currency in /rank output.
// Falls back gracefully to plain ranking if convictions is empty.
func (f *Formatter) FormatRankingWithConviction(
	analyses []domain.COTAnalysis,
	convictions []cot.ConvictionScore,
	regime *fred.MacroRegime,
	date time.Time,
) string {
	// If no conviction data, fall back to the plain ranking
	if len(convictions) == 0 {
		return f.FormatRanking(analyses, date)
	}

	// Build a map from currency → conviction score
	convMap := make(map[string]cot.ConvictionScore, len(convictions))
	for _, cs := range convictions {
		convMap[cs.Currency] = cs
	}

	// Filter to 8 major currencies only
	majors := map[string]bool{"EUR": true, "GBP": true, "JPY": true, "AUD": true,
		"NZD": true, "CAD": true, "CHF": true, "USD": true}

	var entries []convictionRankEntry
	for _, a := range analyses {
		if !majors[a.Contract.Currency] {
			continue
		}
		cs := convMap[a.Contract.Currency]
		entries = append(entries, convictionRankEntry{
			Currency:        a.Contract.Currency,
			Score:           a.SentimentScore,
			COTIndex:        a.COTIndex,
			Conviction:      cs,
			ThinMarketAlert: a.ThinMarketAlert,
			ThinMarketDesc:  a.ThinMarketDesc,
		})
	}

	// Sort by conviction score descending (highest conviction first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Conviction.Score > entries[j].Conviction.Score
	})

	var b strings.Builder
	b.WriteString("🏆 <b>CURRENCY STRENGTH RANKING</b>\n")
	b.WriteString(fmt.Sprintf("<i>COT + FRED Conviction — %s</i>\n", fmtutil.FormatDateWIB(date)))

	// Show regime context if available
	if regime != nil {
		b.WriteString(fmt.Sprintf("\n📊 Regime: <b>%s</b> | Risk-Off: %d/100\n", regime.Name, regime.Score))
	}
	b.WriteString("\n")

	medals := []string{"🥇", "🥈", "🥉", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣"}

	for i, e := range entries {
		medal := ""
		if i < len(medals) {
			medal = medals[i]
		}

		colorDot := scoreDot(e.Score)
		sentSign := "+"
		if e.Score < 0 {
			sentSign = ""
		}

		convScore := int(math.Round(e.Conviction.Score))
		convLabel := e.Conviction.Label
		if convLabel == "" {
			convLabel = e.Conviction.Direction
		}

		// Thin market warning flag
		thinFlag := ""
		if e.ThinMarketAlert {
			thinFlag = " ⚠️THIN"
		}

		// Data quality: show sources count
		srcLabel := ""
		if e.Conviction.SourcesAvailable > 0 && e.Conviction.SourcesAvailable < 4 {
			srcLabel = fmt.Sprintf(" (%d/4)", e.Conviction.SourcesAvailable)
		}

		b.WriteString(fmt.Sprintf("%s %s <b>%s</b>%s | Sent: %s%.0f | Conv: <b>%d/100</b>%s %s\n",
			medal, colorDot, e.Currency, thinFlag, sentSign, e.Score, convScore, srcLabel, convLabel))

		// Component breakdown for top 3 currencies
		if i < 3 && e.Conviction.Version == 3 {
			b.WriteString(fmt.Sprintf("   <i>COT:%+.0f Macro:%+.0f Price:%+.0f Cal:%+.0f</i>\n",
				e.Conviction.COTComponent, e.Conviction.MacroComponent,
				e.Conviction.PriceComponent, e.Conviction.CalendarComponent))
		}
	}

	// Best pairs based on conviction spread
	b.WriteString("\n📊 <b>Best Pairs:</b>\n")
	var plainEntries []rankEntry
	for _, e := range entries {
		plainEntries = append(plainEntries, rankEntry{
			Currency: e.Currency,
			Score:    e.Score,
			COTIndex: e.COTIndex,
		})
	}
	// Re-sort by raw sentiment for pair building
	sort.Slice(plainEntries, func(i, j int) bool {
		return plainEntries[i].Score > plainEntries[j].Score
	})
	pairs := buildBestPairs(plainEntries)
	for _, p := range pairs {
		b.WriteString(p + "\n")
	}

	// Regime advisory
	if regime != nil {
		advisory := regimeAdvisory(regime.Name)
		if advisory != "" {
			b.WriteString(fmt.Sprintf("\n⚠️ %s\n", advisory))
		}
	}

	b.WriteString("\n<i>Tip: </i><code>/cot EUR</code> untuk detail lengkap | <code>/macro</code> untuk FRED regime")
	return b.String()
}

// FormatConvictionBlock renders a detailed conviction score block for the /cot detail view.
// Uses plain language so non-finance users can immediately understand the signal.
func (f *Formatter) FormatConvictionBlock(cs cot.ConvictionScore) string {
	var b strings.Builder

	// Determine icon and plain-language verdict
	var icon string
	var verdict, explanation string
	score := cs.Score

	switch {
	case score >= 75 && cs.Direction == "LONG":
		icon = "🟢"
		verdict = "STRONG BUY SIGNAL"
		explanation = "Hampir semua indikator sepakat: harga kemungkinan besar naik."
	case score >= 65 && cs.Direction == "LONG":
		icon = "🟢"
		verdict = "BUY SIGNAL"
		explanation = "Mayoritas indikator menunjukkan potensi kenaikan harga."
	case score >= 55 && cs.Direction == "LONG":
		icon = "🟡"
		verdict = "LEMAH BUY"
		explanation = "Ada sinyal naik tapi belum cukup kuat. Lebih baik tunggu konfirmasi."
	case score >= 75 && cs.Direction == "SHORT":
		icon = "🔴"
		verdict = "STRONG SELL SIGNAL"
		explanation = "Hampir semua indikator sepakat: harga kemungkinan besar turun."
	case score >= 65 && cs.Direction == "SHORT":
		icon = "🔴"
		verdict = "SELL SIGNAL"
		explanation = "Mayoritas indikator menunjukkan potensi penurunan harga."
	case score >= 55 && cs.Direction == "SHORT":
		icon = "🟡"
		verdict = "LEMAH SELL"
		explanation = "Ada sinyal turun tapi belum kuat. Perlu konfirmasi lebih lanjut."
	default:
		icon = "⚪"
		verdict = "NETRAL / TIDAK JELAS"
		explanation = "Indikator saling bertentangan. Tidak ada sinyal yang cukup jelas saat ini."
	}

	// Build conviction bar: 10 blocks
	filled := int(score / 10)
	if filled > 10 {
		filled = 10
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)

	b.WriteString("\n🎯 <b>KESIMPULAN SINYAL</b>\n")
	b.WriteString(fmt.Sprintf("<code>[%s] %.0f/100</code>\n", bar, score))
	b.WriteString(fmt.Sprintf("%s <b>%s</b>\n", icon, verdict))
	b.WriteString(fmt.Sprintf("<i>%s</i>\n", explanation))

	// Component breakdown — plain language
	b.WriteString("\n<b>Komponen Penilaian:</b>\n")

	cotIcon := "⚪"
	cotDesc := "Netral"
	switch cs.COTBias {
	case "BULLISH":
		cotIcon = "🟢"
		cotDesc = "Institusi besar sedang beli (bullish)"
	case "BEARISH":
		cotIcon = "🔴"
		cotDesc = "Institusi besar sedang jual (bearish)"
	}
	b.WriteString(fmt.Sprintf("<code>  COT Positioning : </code>%s %s\n", cotIcon, cotDesc))

	fredIcon := "⚪"
	fredDesc := "Kondisi makro netral"
	switch cs.FREDRegime {
	case "GOLDILOCKS":
		fredIcon = "🟢"
		fredDesc = "Ekonomi AS sehat, risk-on (GOLDILOCKS)"
	case "DISINFLATIONARY":
		fredIcon = "🟢"
		fredDesc = "Inflasi mereda, kondisi positif (DISINFLATIONARY)"
	case "INFLATIONARY":
		fredIcon = "🟡"
		fredDesc = "Inflasi masih tinggi, hati-hati (INFLATIONARY)"
	case "STRESS":
		fredIcon = "🔴"
		fredDesc = "Pasar dalam tekanan/stres (STRESS)"
	case "RECESSION":
		fredIcon = "🔴"
		fredDesc = "Risiko resesi tinggi (RECESSION)"
	case "STAGFLATION":
		fredIcon = "🔴"
		fredDesc = "Stagflasi: inflasi tinggi + ekonomi lemah"
	}
	b.WriteString(fmt.Sprintf("<code>  Kondisi Ekonomi  : </code>%s %s\n", fredIcon, fredDesc))

	b.WriteString("<i>  Data: Harga (30%) + COT (25%) + Ekonomi (20%) + Kalender (15%) + Stres (10%)</i>\n")

	return b.String()
}

// cotLabel returns a human-readable label for a COT Index value (0-100).
func cotLabel(idx float64) string {
	switch {
	case idx >= 80:
		return "Extreme Long"
	case idx >= 60:
		return "Bullish"
	case idx >= 40:
		return "Neutral"
	case idx >= 20:
		return "Bearish"
	default:
		return "Extreme Short"
	}
}

// commercialSignalLabel returns a display string for the commercial signal,
// noting whether it's contrarian or structural depending on report type.
func commercialSignalLabel(signal, rt string) string {
	// For TFF (forex/indices): Dealers are the "dumb money" market maker side —
	// their signal is LESS reliable as a contrarian indicator (unlike classic commercials).
	// For DISAGGREGATED (commodities): Prod/Swap are true producers = contrarian smart.
	suffix := ""
	if rt == "TFF" {
		suffix = " (dealer)"
	} else if rt == "DISAGGREGATED" {
		suffix = " (contrarian)"
	}
	return signal + suffix
}

// signalConfluenceInterpretation returns a plain-language interpretation
// of the combined Smart Money + Commercial signal alignment.
func signalConfluenceInterpretation(specSignal, commSignal, rt string) string {
	isBullish := func(s string) bool {
		return s == "BULLISH" || s == "STRONG_BULLISH"
	}
	isBearish := func(s string) bool {
		return s == "BEARISH" || s == "STRONG_BEARISH"
	}
	isStrong := func(s string) bool {
		return s == "STRONG_BULLISH" || s == "STRONG_BEARISH"
	}

	specBull := isBullish(specSignal)
	specBear := isBearish(specSignal)
	commBull := isBullish(commSignal)
	commBear := isBearish(commSignal)
	commNeutral := !commBull && !commBear

	switch {
	// Strong agreement both sides
	case specBull && commBull && isStrong(specSignal) && isStrong(commSignal):
		return "<i>  ✅✅ KONFIRMASI KUAT: Smart money DAN hedger keduanya sangat bullish</i>\n"
	case specBear && commBear && isStrong(specSignal) && isStrong(commSignal):
		return "<i>  🔴🔴 KONFIRMASI KUAT: Smart money DAN hedger keduanya sangat bearish</i>\n"

	// Normal agreement
	case specBull && commBull:
		return "<i>  ✅ KONFIRMASI: Smart money dan hedger sama-sama bullish</i>\n"
	case specBear && commBear:
		return "<i>  🔴 KONFIRMASI: Smart money dan hedger sama-sama bearish</i>\n"

	// Classic divergence for commodities (normal)
	case specBull && commBear && rt == "DISAGGREGATED":
		return "<i>  ⚖️ Normal untuk komoditas: spekulan beli, produsen hedge jual</i>\n"
	case specBear && commBull && rt == "DISAGGREGATED":
		return "<i>  🔀 Tidak biasa: spekulan jual, tapi produsen justru akumulasi beli</i>\n"

	// Divergence for forex/indices
	case specBull && commBear:
		return "<i>  ⚠️ KONFLIK: Smart money bullish tapi dealer/hedger bearish — hati-hati</i>\n"
	case specBear && commBull:
		return "<i>  ⚠️ KONFLIK: Smart money bearish tapi dealer/hedger bullish — sinyal campur</i>\n"

	// Commercial neutral
	case specBull && commNeutral:
		return "<i>  🟡 Smart money bullish, hedger masih netral — belum full konfirmasi</i>\n"
	case specBear && commNeutral:
		return "<i>  🟡 Smart money bearish, hedger masih netral — belum full konfirmasi</i>\n"

	// Both neutral
	default:
		return "<i>  ⚪ Kedua sisi netral — tidak ada sinyal terarah saat ini</i>\n"
	}
}

// buildBestPairs generates the top 3 long/short pair recommendations.
// Long the highest-ranked currency, short the lowest-ranked.
// Direction is derived from the pair name: if the base currency (first 3 chars)
// matches the favored currency → LONG; if the base is the weak currency → SHORT.
func buildBestPairs(entries []rankEntry) []string {
	if len(entries) < 2 {
		return nil
	}

	var pairs []string
	seen := make(map[string]bool)

	// Try top-bull vs bottom-bear combinations
	for i := 0; i < len(entries) && len(pairs) < 3; i++ {
		for j := len(entries) - 1; j > i && len(pairs) < 3; j-- {
			long := entries[i]
			short := entries[j]
			spread := long.Score - short.Score

			if spread < 30 {
				continue // not enough spread to be meaningful
			}

			pairName := formatPairName(long.Currency, short.Currency)
			if seen[pairName] {
				continue
			}
			seen[pairName] = true

			direction := pairDirection(pairName, long.Currency)
			pairs = append(pairs, fmt.Sprintf("→ %s <b>%s</b> (spread +%.0f)",
				direction, pairName, math.Abs(spread)))
		}
	}

	// If no strong spreads, show best available
	if len(pairs) == 0 && len(entries) >= 2 {
		long := entries[0]
		short := entries[len(entries)-1]
		spread := long.Score - short.Score
		pairName := formatPairName(long.Currency, short.Currency)
		direction := pairDirection(pairName, long.Currency)
		pairs = append(pairs, fmt.Sprintf("→ %s <b>%s</b> (spread +%.0f)", direction, pairName, spread))
	}

	return pairs
}

// pairDirection returns "LONG" if the favored currency is the base (first) in
// the pair, or "SHORT" if the favored currency ended up as the quote (second).
// Example: favored=USD, pair=AUDUSD → base is AUD (not favored) → SHORT AUDUSD.
//          favored=EUR, pair=EURUSD → base is EUR (favored)     → LONG EURUSD.
func pairDirection(pairName, favoredCurrency string) string {
	if strings.HasPrefix(pairName, favoredCurrency) {
		return "LONG"
	}
	return "SHORT"
}

// formatPairName formats a forex pair name from two currency codes.
// Follows standard convention: USD is always the second in majors where applicable.
func formatPairName(longCur, shortCur string) string {
	// Standard major pairs where USD is quote
	usdQuote := map[string]bool{"EUR": true, "GBP": true, "AUD": true, "NZD": true}
	// Standard major pairs where USD is base
	usdBase := map[string]bool{"JPY": true, "CHF": true, "CAD": true}

	if longCur == "USD" {
		if usdBase[shortCur] {
			return "USD" + shortCur // e.g., USDJPY
		}
		return shortCur + "USD" // e.g., EURUSD (reversed — USD short)
	}
	if shortCur == "USD" {
		if usdQuote[longCur] {
			return longCur + "USD" // e.g., GBPUSD
		}
		return "USD" + longCur // e.g., USDCAD
	}
	// Cross pair: long first
	return longCur + shortCur
}

// FormatBiasHTML formats detected COT directional biases for Telegram display.
func (f *Formatter) FormatBiasHTML(signals []cot.Signal, filterCurrency string) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x8E\xAF <b>COT DIRECTIONAL BIAS</b>\n")
	if filterCurrency != "" {
		b.WriteString(fmt.Sprintf("<i>Filtered: %s</i>\n", filterCurrency))
	}
	b.WriteString("\n")

	if len(signals) == 0 {
		b.WriteString("No actionable biases detected.\n")
		b.WriteString("\n<i>Tip: Biases fire on extreme positioning, smart money moves,\ndivergences, momentum shifts, and thin markets.</i>")
		return b.String()
	}

	for i, s := range signals {
		if i >= 10 {
			b.WriteString(fmt.Sprintf("\n<i>... +%d more biases</i>", len(signals)-10))
			break
		}

		dirIcon := "\xF0\x9F\x9F\xA2"
		if s.Direction == "BEARISH" {
			dirIcon = "\xF0\x9F\x94\xB4"
		}

		strengthBar := strings.Repeat("\xE2\x96\x88", s.Strength) + strings.Repeat("\xE2\x96\x91", 5-s.Strength)

		b.WriteString(fmt.Sprintf("%s <b>%s</b> \xE2\x80\x94 %s\n", dirIcon, s.Currency, s.Type))
		b.WriteString(fmt.Sprintf("<code>  Str: [%s] %d/5 | Conf: %.0f%%</code>\n", strengthBar, s.Strength, s.Confidence))
		b.WriteString(fmt.Sprintf("<i>  %s</i>\n", s.Description))

		for _, factor := range s.Factors {
			b.WriteString(fmt.Sprintf("<code>  \xE2\x80\xA2 %s</code>\n", factor))
		}
		b.WriteString("\n")
	}

	b.WriteString("<i>Tip: </i><code>/bias EUR</code> | <code>/cot EUR</code>")
	return b.String()
}

// FormatBiasSummary formats a compact bias summary for the /cot detail view.
func (f *Formatter) FormatBiasSummary(signals []cot.Signal) string {
	if len(signals) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\xF0\x9F\x8E\xAF <b>Active Biases:</b>\n")

	for i, s := range signals {
		if i >= 3 {
			b.WriteString(fmt.Sprintf("<i>  +%d more signals available</i>\n", len(signals)-3))
			break
		}

		dirIcon := "\xF0\x9F\x9F\xA2"
		if s.Direction == "BEARISH" {
			dirIcon = "\xF0\x9F\x94\xB4"
		}

		b.WriteString(fmt.Sprintf("%s %s (%d/5, %.0f%%) \xE2\x80\x94 <i>%s</i>\n",
			dirIcon, s.Type, s.Strength, s.Confidence, s.Description))
	}

	return b.String()
}

// FormatPriceCOTDivergence formats a price-COT divergence alert in plain language.
func (f *Formatter) FormatPriceCOTDivergence(div pricesvc.PriceCOTDivergence) string {
	var b strings.Builder

	icon := "⚠️"
	severityLabel := "Perlu Perhatian"
	if div.Severity == "HIGH" {
		icon = "🚨"
		severityLabel = "PERINGATAN KERAS"
	}

	b.WriteString(fmt.Sprintf("\n%s <b>SINYAL BERTENTANGAN (%s)</b>\n", icon, severityLabel))

	// Plain language explanation based on divergence type
	if div.PriceTrend == "UP" && div.COTDirection == "BEARISH" {
		b.WriteString("<b>Situasi:</b> Harga naik, tapi institusi besar justru JUAL\n")
		b.WriteString("<i>Artinya: Kenaikan harga ini mungkin tidak didukung oleh pemain besar.\n")
		b.WriteString("Bisa jadi ini \"rally palsu\" atau harga akan berbalik turun.\n")
		b.WriteString("Hati-hati beli di sini — tunggu konfirmasi lebih lanjut.</i>\n")
		if div.Severity == "HIGH" {
			b.WriteString("🚨 <b>COT Index di zona ekstrem SHORT — sinyal reversal kuat!</b>\n")
		}
	} else if div.PriceTrend == "DOWN" && div.COTDirection == "BULLISH" {
		b.WriteString("<b>Situasi:</b> Harga turun, tapi institusi besar justru BELI\n")
		b.WriteString("<i>Artinya: Penurunan harga ini mungkin sementara.\n")
		b.WriteString("Institusi besar melihat nilai di sini dan mulai akumulasi.\n")
		b.WriteString("Ini bisa menjadi kesempatan beli — tapi tunggu harga stabilisasi dulu.</i>\n")
		if div.Severity == "HIGH" {
			b.WriteString("🚨 <b>COT Index di zona ekstrem LONG — potensi reversal naik kuat!</b>\n")
		}
	} else {
		// Generic fallback
		b.WriteString(fmt.Sprintf("<i>%s</i>\n", div.Description))
	}

	b.WriteString(fmt.Sprintf("<code>  COT Index: %.0f%% | Tren Harga: %s</code>\n", div.COTIndex, div.PriceTrend))
	return b.String()
}

// FormatPriceCOTAlignment formats a confirmation when price and COT agree (no divergence).
// This replaces the silent "no divergence" gap — user always gets a price-COT verdict.
func (f *Formatter) FormatPriceCOTAlignment(pc *domain.PriceContext, a domain.COTAnalysis) string {
	if pc == nil {
		return ""
	}

	var b strings.Builder

	cotDir := "netral"
	if a.COTIndex > 60 {
		cotDir = "bullish (beli)"
	} else if a.COTIndex < 40 {
		cotDir = "bearish (jual)"
	}

	priceTrend := "sideways"
	if pc.Trend4W == "UP" {
		priceTrend = "naik"
	} else if pc.Trend4W == "DOWN" {
		priceTrend = "turun"
	}

	b.WriteString("\n🔗 <b>KONFIRMASI HARGA vs COT</b>\n")

	cotNeutral := a.COTIndex >= 40 && a.COTIndex <= 60
	priceFlat := pc.Trend4W == "FLAT"

	switch {
	// ✅ Harga dan COT sama-sama searah
	case (pc.Trend4W == "UP" && a.COTIndex > 60) || (pc.Trend4W == "DOWN" && a.COTIndex < 40):
		b.WriteString("✅ <b>Harga dan posisi institusi SELARAS</b>\n")
		b.WriteString(fmt.Sprintf("<i>Tren harga %s, dan institusi besar juga %s.\n", priceTrend, cotDir))
		b.WriteString("Ini sinyal lebih dapat dipercaya — momentum kemungkinan berlanjut.</i>\n")

	// ⚪ Harga naik/turun tapi COT netral — sinyal lemah
	case pc.Trend4W == "UP" && cotNeutral:
		b.WriteString("🟡 <b>Harga naik tapi institusi masih netral</b>\n")
		b.WriteString("<i>Harga sedang naik, tapi posisi institusi belum memihak ke atas.\n")
		b.WriteString("Bisa jadi pergerakan ini belum dikonfirmasi — tunggu COT bergerak ke atas dulu.</i>\n")

	case pc.Trend4W == "DOWN" && cotNeutral:
		b.WriteString("🟡 <b>Harga turun tapi institusi masih netral</b>\n")
		b.WriteString("<i>Harga sedang turun, tapi posisi institusi belum memihak ke bawah.\n")
		b.WriteString("Penurunan belum dikonfirmasi oleh data COT — hati-hati dengan false breakdown.</i>\n")

	// ⚪ COT punya arah tapi harga sideways — institusi menunggu
	case priceFlat && a.COTIndex > 60:
		b.WriteString("🟡 <b>Institusi bullish tapi harga masih sideways</b>\n")
		b.WriteString("<i>Dana besar sudah akumulasi posisi beli, tapi harga belum bergerak naik.\n")
		b.WriteString("Ini bisa jadi setup sebelum breakout — pantau level resistance.</i>\n")

	case priceFlat && a.COTIndex < 40:
		b.WriteString("🟡 <b>Institusi bearish tapi harga masih sideways</b>\n")
		b.WriteString("<i>Dana besar sudah akumulasi posisi jual, tapi harga belum turun.\n")
		b.WriteString("Bisa jadi distribusi diam-diam — waspadai breakdown ke bawah.</i>\n")

	// ⚪ Semua netral — tidak ada sinyal
	default:
		b.WriteString("⚪ <b>Tidak ada sinyal jelas saat ini</b>\n")
		b.WriteString(fmt.Sprintf("<i>Tren harga %s, posisi institusi %s.\n", priceTrend, cotDir))
		b.WriteString("Sebaiknya tunggu sampai salah satu pihak menunjukkan arah yang jelas.</i>\n")
	}

	return b.String()
}

// FormatStrengthRanking formats the dual price+COT currency strength ranking.
func (f *Formatter) FormatStrengthRanking(strengths []pricesvc.CurrencyStrength) string {
	if len(strengths) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\xF0\x9F\x92\xAA <b>Price + COT Strength</b>\n")
	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-4s %6s %5s %6s\n", "CCY", "Price", "COT", "Score"))
	b.WriteString(strings.Repeat("\xE2\x94\x80", 24) + "\n")

	for _, s := range strengths {
		divFlag := " "
		if s.Divergence {
			divFlag = "!"
		}
		b.WriteString(fmt.Sprintf("%-4s %+5.1f %+4.0f %+5.1f %s\n",
			s.Currency, s.PriceScore, s.COTScore, s.CombinedScore, divFlag))
	}
	b.WriteString("</pre>")

	// Show divergence warnings
	for _, s := range strengths {
		if s.Divergence {
			b.WriteString(fmt.Sprintf("\xE2\x9A\xA0\xEF\xB8\x8F %s: %s\n", s.Currency, s.DivergenceMsg))
		}
	}

	return b.String()
}

// FormatCOTShareText generates a plain-text, copy-paste friendly version of COT analysis.
// No HTML tags — suitable for forwarding to other chats or platforms.
func (f *Formatter) FormatCOTShareText(a domain.COTAnalysis) string {
	var b strings.Builder

	currency := contractCodeToFriendly(a.Contract.Code)
	if currency == "" {
		currency = a.Contract.Name
	}

	biasEmoji := "⚪"
	biasLabel := "NEUTRAL"
	if a.NetPosition > 0 {
		biasEmoji = "🟢"
		biasLabel = "BULLISH"
	} else if a.NetPosition < 0 {
		biasEmoji = "🔴"
		biasLabel = "BEARISH"
	}

	b.WriteString(fmt.Sprintf("📊 COT Report — %s\n", currency))
	b.WriteString(fmt.Sprintf("Date: %s\n\n", a.ReportDate.Format("2 Jan 2006")))

	b.WriteString(fmt.Sprintf("Net Position: %s contracts [%s %s]\n",
		fmtutil.FmtNumSigned(a.NetPosition, 0), biasEmoji, biasLabel))
	b.WriteString(fmt.Sprintf("COT Index: %.1f%%\n", a.COTIndex))
	b.WriteString(fmt.Sprintf("Net Change (WoW): %s\n", fmtutil.FmtNumSigned(a.NetChange, 0)))

	if a.SpecMomentum4W != 0 {
		b.WriteString(fmt.Sprintf("Momentum (4W): %s\n", fmtutil.FmtNumSigned(a.SpecMomentum4W, 0)))
	}

	b.WriteString(fmt.Sprintf("Signal: %s\n", a.SpeculatorSignal))

	b.WriteString("\n⚡ ARK Intelligence Terminal")

	return b.String()
}
