package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// -- CONFIG -------------------------------------------------------------------

var (
	BOT_TOKEN   = os.Getenv("BOT_TOKEN")
	CHAT_ID     = os.Getenv("CHAT_ID")
	PREFS_FILE  = getEnvDefault("PREFS_FILE", "/app/data/prefs.json")
	WIB         *time.Location
)

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// -- DEFAULT ALERT CONFIG -----------------------------------------------------

var (
	DEFAULT_ALERT_MINUTES = []int{30, 15, 5}
	DEFAULT_ALERT_IMPACTS = []string{"High"}
	ALL_ALERT_MINUTES     = []int{60, 30, 15, 5, 1}
	ALL_IMPACTS           = []string{"High", "Medium", "Low"}
)

// -- DATA MODELS --------------------------------------------------------------

type FFEvent struct {
	Title    string `json:"title"`
	Country  string `json:"country"`
	Date     string `json:"date"`
	Impact   string `json:"impact"`
	Forecast string `json:"forecast"`
	Previous string `json:"previous"`
	Actual   string `json:"actual"`
}

type EventState struct {
	AlertedMinutes map[int]bool
	ActualSent     bool
}

// -- USER PREFERENCES ---------------------------------------------------------

type UserPrefs struct {
	AlertMinutes []int    `json:"alert_minutes"` // e.g. [30, 15, 5]
	AlertImpacts []string `json:"alert_impacts"` // e.g. ["High", "Medium"]
	AlertsOn     bool     `json:"alerts_on"`
}

func defaultPrefs() UserPrefs {
	return UserPrefs{
		AlertMinutes: append([]int{}, DEFAULT_ALERT_MINUTES...),
		AlertImpacts: append([]string{}, DEFAULT_ALERT_IMPACTS...),
		AlertsOn:     true,
	}
}

// -- PREFERENCES STORE (file-based JSON) --------------------------------------

type PrefsStore struct {
	mu    sync.RWMutex
	prefs map[string]UserPrefs // key: user_id as string
	path  string
}

func NewPrefsStore(path string) *PrefsStore {
	ps := &PrefsStore{
		prefs: make(map[string]UserPrefs),
		path:  path,
	}
	ps.load()
	return ps
}

func (ps *PrefsStore) load() {
	data, err := os.ReadFile(ps.path)
	if err != nil {
		log.Printf("[PREFS] No existing prefs file: %v", err)
		return
	}
	var m map[string]UserPrefs
	if err := json.Unmarshal(data, &m); err != nil {
		log.Printf("[PREFS] Parse error: %v", err)
		return
	}
	ps.prefs = m
	log.Printf("[PREFS] Loaded %d user preferences", len(m))
}

func (ps *PrefsStore) save() {
	data, err := json.MarshalIndent(ps.prefs, "", "  ")
	if err != nil {
		log.Printf("[PREFS] Marshal error: %v", err)
		return
	}
	if err := os.WriteFile(ps.path, data, 0644); err != nil {
		log.Printf("[PREFS] Write error: %v", err)
	}
}

func (ps *PrefsStore) Get(userID int64) UserPrefs {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	key := strconv.FormatInt(userID, 10)
	if p, ok := ps.prefs[key]; ok {
		return p
	}
	return defaultPrefs()
}

func (ps *PrefsStore) Set(userID int64, p UserPrefs) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	key := strconv.FormatInt(userID, 10)
	ps.prefs[key] = p
	ps.save()
}

func (ps *PrefsStore) ToggleMinute(userID int64, min int) UserPrefs {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	key := strconv.FormatInt(userID, 10)
	p, ok := ps.prefs[key]
	if !ok {
		p = defaultPrefs()
	}
	if containsInt(p.AlertMinutes, min) {
		p.AlertMinutes = removeInt(p.AlertMinutes, min)
	} else {
		p.AlertMinutes = append(p.AlertMinutes, min)
		sort.Sort(sort.Reverse(sort.IntSlice(p.AlertMinutes)))
	}
	ps.prefs[key] = p
	ps.save()
	return p
}

func (ps *PrefsStore) ToggleImpact(userID int64, impact string) UserPrefs {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	key := strconv.FormatInt(userID, 10)
	p, ok := ps.prefs[key]
	if !ok {
		p = defaultPrefs()
	}
	if containsStr(p.AlertImpacts, impact) {
		p.AlertImpacts = removeStr(p.AlertImpacts, impact)
	} else {
		p.AlertImpacts = append(p.AlertImpacts, impact)
	}
	ps.prefs[key] = p
	ps.save()
	return p
}

func (ps *PrefsStore) ToggleAlerts(userID int64) UserPrefs {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	key := strconv.FormatInt(userID, 10)
	p, ok := ps.prefs[key]
	if !ok {
		p = defaultPrefs()
	}
	p.AlertsOn = !p.AlertsOn
	ps.prefs[key] = p
	ps.save()
	return p
}

// AllActive returns all user IDs with alerts enabled + their prefs
func (ps *PrefsStore) AllActive() map[int64]UserPrefs {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	result := make(map[int64]UserPrefs)
	for k, p := range ps.prefs {
		if p.AlertsOn {
			uid, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				continue
			}
			result[uid] = p
		}
	}
	return result
}

// -- HELPERS ------------------------------------------------------------------

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func removeInt(s []int, v int) []int {
	var r []int
	for _, x := range s {
		if x != v {
			r = append(r, x)
		}
	}
	return r
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if strings.EqualFold(x, v) {
			return true
		}
	}
	return false
}

func removeStr(s []string, v string) []string {
	var r []string
	for _, x := range s {
		if !strings.EqualFold(x, v) {
			r = append(r, x)
		}
	}
	return r
}

// -- TELEGRAM STRUCTS ---------------------------------------------------------

type TGUpdate struct {
	UpdateID      int         `json:"update_id"`
	Message       *TGMessage  `json:"message"`
	CallbackQuery *TGCallback `json:"callback_query"`
}

type TGMessage struct {
	MessageID int    `json:"message_id"`
	Chat      TGChat `json:"chat"`
	From      *TGUser `json:"from"`
	Text      string `json:"text"`
}

type TGChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type TGCallback struct {
	ID      string     `json:"id"`
	From    TGUser     `json:"from"`
	Message *TGMessage `json:"message"`
	Data    string     `json:"data"`
}

type TGUser struct {
	ID int64 `json:"id"`
}

type TGResponse struct {
	Ok     bool       `json:"ok"`
	Result []TGUpdate `json:"result"`
}

type TGSendResponse struct {
	Ok          bool   `json:"ok"`
	Description string `json:"description"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// -- COUNTRY FLAGS ------------------------------------------------------------

var countryFlags = map[string]string{
	"USD": "\U0001F1FA\U0001F1F8", "EUR": "\U0001F1EA\U0001F1FA",
	"GBP": "\U0001F1EC\U0001F1E7", "JPY": "\U0001F1EF\U0001F1F5",
	"AUD": "\U0001F1E6\U0001F1FA", "NZD": "\U0001F1F3\U0001F1FF",
	"CAD": "\U0001F1E8\U0001F1E6", "CHF": "\U0001F1E8\U0001F1ED",
	"CNY": "\U0001F1E8\U0001F1F3", "ALL": "\U0001F30D",
}

func flag(country string) string {
	if f, ok := countryFlags[strings.ToUpper(country)]; ok {
		return f
	}
	return "\U0001F30D"
}

func impactIcon(impact string) string {
	switch strings.ToLower(impact) {
	case "high":
		return "\U0001F534"
	case "medium":
		return "\U0001F7E0"
	case "low":
		return "\U0001F7E1"
	case "holiday":
		return "\U0001F3D6"
	default:
		return "\u26AA"
	}
}

// -- BOT CORE -----------------------------------------------------------------

type Bot struct {
	token      string
	chatID     string
	client     *http.Client
	events     []FFEvent
	eventState map[string]*EventState
	mu         sync.RWMutex
	lastFetch  time.Time
	offset     int
	prefs      *PrefsStore
}

func NewBot(token, chatID string) *Bot {
	return &Bot{
		token:      token,
		chatID:     chatID,
		client:     &http.Client{Timeout: 15 * time.Second},
		eventState: make(map[string]*EventState),
		prefs:      NewPrefsStore(PREFS_FILE),
	}
}

func (b *Bot) apiURL(method string) string {
	return fmt.Sprintf("https://api.telegram.org/bot%s/%s", b.token, method)
}

func (b *Bot) postJSON(method string, payload map[string]interface{}) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", b.apiURL(method), strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var result TGSendResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Ok {
		return fmt.Errorf("tg %s: %s", method, result.Description)
	}
	return nil
}

func (b *Bot) sendMessage(chatID, text string) error {
	if chatID == "" {
		chatID = b.chatID
	}
	return b.postJSON("sendMessage", map[string]interface{}{
		"chat_id": chatID, "text": text, "parse_mode": "HTML",
		"disable_web_page_preview": true,
	})
}

func (b *Bot) sendWithKB(chatID, text string, kb InlineKeyboardMarkup) error {
	if chatID == "" {
		chatID = b.chatID
	}
	return b.postJSON("sendMessage", map[string]interface{}{
		"chat_id": chatID, "text": text, "parse_mode": "HTML",
		"reply_markup": kb, "disable_web_page_preview": true,
	})
}

func (b *Bot) editMsg(chatID string, msgID int, text string, kb *InlineKeyboardMarkup) error {
	p := map[string]interface{}{
		"chat_id": chatID, "message_id": msgID, "text": text,
		"parse_mode": "HTML", "disable_web_page_preview": true,
	}
	if kb != nil {
		p["reply_markup"] = kb
	}
	return b.postJSON("editMessageText", p)
}

func (b *Bot) answerCB(cbID, text string) {
	p := map[string]interface{}{"callback_query_id": cbID}
	if text != "" {
		p["text"] = text
	}
	b.postJSON("answerCallbackQuery", p)
}

func (b *Bot) getUpdates() ([]TGUpdate, error) {
	url := fmt.Sprintf("%s?offset=%d&timeout=1&allowed_updates=[\"message\",\"callback_query\"]",
		b.apiURL("getUpdates"), b.offset)
	resp, err := b.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result TGResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Ok {
		return nil, fmt.Errorf("getUpdates failed")
	}
	return result.Result, nil
}

// -- DATA FETCHER -------------------------------------------------------------


func eventKey(e FFEvent) string { return e.Title + "|" + e.Date }

func parseEventTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05-0700", s)
	}
	return t, err
}

// -- INLINE KEYBOARDS (RAMPED MENU) -------------------------------------------

// Main menu: just 2 buttons
func kbMain() InlineKeyboardMarkup {
	return InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{{Text: "\U0001F4C5 Calendar", CallbackData: "menu_cal"}},
		{{Text: "\u2699\uFE0F Settings", CallbackData: "menu_settings"}},
	}}
}

// Calendar sub-menu: all calendar views in one place
func kbCalendarMenu() InlineKeyboardMarkup {
	return InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{{Text: "\U0001F4C6 Today", CallbackData: "cal_today"}, {Text: "\U0001F4CB This Week", CallbackData: "cal_week"}},
		{{Text: "\U0001F534 High Impact", CallbackData: "cal_high"}, {Text: "\u23ED Next Events", CallbackData: "cal_next"}},
		{{Text: "\U0001F504 Refresh", CallbackData: "cal_refresh"}},
		{{Text: "\u2B05 Back", CallbackData: "start"}},
	}}
}

// Shown after calendar view results
func kbCalendarBack() InlineKeyboardMarkup {
	return InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{{Text: "\u2B05 Calendar", CallbackData: "menu_cal"}, {Text: "\U0001F3E0 Menu", CallbackData: "start"}},
	}}
}

// Settings sub-menu
func kbSettingsMenu() InlineKeyboardMarkup {
	return InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{{Text: "\u23F0 Alert Timing", CallbackData: "set_timing"}},
		{{Text: "\U0001F4CA Impact Filter", CallbackData: "set_impact"}},
		{{Text: "\U0001F514 Toggle Alerts", CallbackData: "set_toggle"}},
		{{Text: "\U0001F4CB Status", CallbackData: "set_status"}},
		{{Text: "\u2B05 Back", CallbackData: "start"}},
	}}
}

// Alert timing toggles — dynamic based on user prefs
func kbTimingToggles(p UserPrefs) InlineKeyboardMarkup {
	var rows [][]InlineKeyboardButton
	var row []InlineKeyboardButton
	for _, m := range ALL_ALERT_MINUTES {
		label := fmt.Sprintf("%dm", m)
		if containsInt(p.AlertMinutes, m) {
			label = "\u2705 " + label
		} else {
			label = "\u274C " + label
		}
		row = append(row, InlineKeyboardButton{Text: label, CallbackData: fmt.Sprintf("tog_min_%d", m)})
		if len(row) == 3 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	rows = append(rows, []InlineKeyboardButton{{Text: "\u2B05 Settings", CallbackData: "menu_settings"}})
	return InlineKeyboardMarkup{InlineKeyboard: rows}
}

// Impact filter toggles — dynamic based on user prefs
func kbImpactToggles(p UserPrefs) InlineKeyboardMarkup {
	var row []InlineKeyboardButton
	for _, imp := range ALL_IMPACTS {
		label := impactIcon(imp) + " " + imp
		if containsStr(p.AlertImpacts, imp) {
			label = "\u2705 " + label
		} else {
			label = "\u274C " + label
		}
		row = append(row, InlineKeyboardButton{Text: label, CallbackData: "tog_imp_" + strings.ToLower(imp)})
	}
	return InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		row,
		{{Text: "\u2B05 Settings", CallbackData: "menu_settings"}},
	}}
}

func kbBack() InlineKeyboardMarkup {
	return InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{{Text: "\U0001F3E0 Menu", CallbackData: "start"}},
	}}
}

// -- FORMAT HELPERS ------------------------------------------------------------

func fmtEventLine(e FFEvent) string {
	t, err := parseEventTime(e.Date)
	if err != nil {
		return ""
	}
	wt := t.In(WIB)
	line := fmt.Sprintf("%s <b>%s</b> %s %s", impactIcon(e.Impact), wt.Format("15:04"), flag(e.Country), e.Title)
	var dp []string
	if e.Forecast != "" {
		dp = append(dp, "F:"+e.Forecast)
	}
	if e.Previous != "" {
		dp = append(dp, "P:"+e.Previous)
	}
	if e.Actual != "" {
		dp = append(dp, "<b>A:"+e.Actual+"</b>")
	}
	if len(dp) > 0 {
		line += "\n    " + strings.Join(dp, " | ")
	}
	return line
}

func fmtToday(events []FFEvent) string {
	now := time.Now().In(WIB)
	dayStr := now.Format("2006-01-02")
	var dayEvents []FFEvent
	for _, e := range events {
		t, err := parseEventTime(e.Date)
		if err != nil {
			continue
		}
		if t.In(WIB).Format("2006-01-02") == dayStr {
			dayEvents = append(dayEvents, e)
		}
	}
	if len(dayEvents) == 0 {
		return fmt.Sprintf("<b>%s</b>\n\nNo events today.", now.Format("Mon, 02 Jan 2006"))
	}
	sort.Slice(dayEvents, func(i, j int) bool { return dayEvents[i].Date < dayEvents[j].Date })
	high := 0
	for _, e := range dayEvents {
		if strings.EqualFold(e.Impact, "high") {
			high++
		}
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>\U0001F4C6 %s</b>\n", now.Format("Mon, 02 Jan 2006")))
	sb.WriteString(fmt.Sprintf("%d events \u2022 %d high impact\n\n", len(dayEvents), high))
	for _, e := range dayEvents {
		if line := fmtEventLine(e); line != "" {
			sb.WriteString(line + "\n")
		}
	}
	sb.WriteString(fmt.Sprintf("\n<i>Updated %s WIB</i>", time.Now().In(WIB).Format("15:04")))
	return sb.String()
}

func fmtWeek(events []FFEvent) string {
	dayMap := make(map[string][]FFEvent)
	var days []string
	for _, e := range events {
		t, err := parseEventTime(e.Date)
		if err != nil {
			continue
		}
		ds := t.In(WIB).Format("2006-01-02")
		if _, ok := dayMap[ds]; !ok {
			days = append(days, ds)
		}
		dayMap[ds] = append(dayMap[ds], e)
	}
	sort.Strings(days)
	var sb strings.Builder
	sb.WriteString("<b>\U0001F4CB Weekly Calendar</b>\n\n")
	for _, ds := range days {
		dev := dayMap[ds]
		sort.Slice(dev, func(i, j int) bool { return dev[i].Date < dev[j].Date })
		dt, _ := time.Parse("2006-01-02", ds)
		h := 0
		for _, e := range dev {
			if strings.EqualFold(e.Impact, "high") {
				h++
			}
		}
		sb.WriteString(fmt.Sprintf("<b>%s</b> (%d", dt.Format("Mon 02 Jan"), len(dev)))
		if h > 0 {
			sb.WriteString(fmt.Sprintf(", %d HIGH", h))
		}
		sb.WriteString(")\n")
		for _, e := range dev {
			if line := fmtEventLine(e); line != "" {
				sb.WriteString(line + "\n")
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("<i>Source: MQL5 Economic Calendar</i>")
	return sb.String()
}

func fmtHigh(events []FFEvent) string {
	var hi []FFEvent
	for _, e := range events {
		if strings.EqualFold(e.Impact, "high") {
			hi = append(hi, e)
		}
	}
	if len(hi) == 0 {
		return "<b>\U0001F534 High Impact</b>\n\nNo high-impact events this week."
	}
	sort.Slice(hi, func(i, j int) bool { return hi[i].Date < hi[j].Date })
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>\U0001F534 High Impact</b> (%d events)\n\n", len(hi)))
	curDay := ""
	for _, e := range hi {
		t, err := parseEventTime(e.Date)
		if err != nil {
			continue
		}
		ds := t.In(WIB).Format("Mon 02 Jan")
		if ds != curDay {
			if curDay != "" {
				sb.WriteString("\n")
			}
			sb.WriteString(fmt.Sprintf("<b>%s</b>\n", ds))
			curDay = ds
		}
		wt := t.In(WIB)
		sb.WriteString(fmt.Sprintf("<b>%s</b> %s %s\n", wt.Format("15:04"), flag(e.Country), e.Title))
		var dp []string
		if e.Forecast != "" {
			dp = append(dp, "F:"+e.Forecast)
		}
		if e.Previous != "" {
			dp = append(dp, "P:"+e.Previous)
		}
		if e.Actual != "" {
			dp = append(dp, "<b>A:"+e.Actual+"</b>")
		}
		if len(dp) > 0 {
			sb.WriteString("    " + strings.Join(dp, " | ") + "\n")
		}
	}
	sb.WriteString("\n<i>Source: MQL5 Economic Calendar</i>")
	return sb.String()
}

func fmtNext(events []FFEvent, count int) string {
	now := time.Now().In(WIB)
	var up []FFEvent
	for _, e := range events {
		t, err := parseEventTime(e.Date)
		if err != nil {
			continue
		}
		if t.In(WIB).After(now) {
			up = append(up, e)
		}
	}
	sort.Slice(up, func(i, j int) bool { return up[i].Date < up[j].Date })
	if len(up) == 0 {
		return "<b>\u23ED Next Events</b>\n\nNo upcoming events this week."
	}
	if count > len(up) {
		count = len(up)
	}
	up = up[:count]
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>\u23ED Next %d Events</b>\n\n", count))
	for _, e := range up {
		t, _ := parseEventTime(e.Date)
		wt := t.In(WIB)
		mins := int(time.Until(wt).Minutes())
		cd := ""
		if mins < 60 {
			cd = fmt.Sprintf("%dm", mins)
		} else if mins < 1440 {
			cd = fmt.Sprintf("%dh%dm", mins/60, mins%60)
		} else {
			cd = fmt.Sprintf("%dd%dh", mins/1440, (mins%1440)/60)
		}
		sb.WriteString(fmt.Sprintf("%s <b>%s</b> %s %s <i>(%s)</i>\n",
			impactIcon(e.Impact), wt.Format("15:04"), flag(e.Country), e.Title, cd))
		var dp []string
		if e.Forecast != "" {
			dp = append(dp, "F:"+e.Forecast)
		}
		if e.Previous != "" {
			dp = append(dp, "P:"+e.Previous)
		}
		if len(dp) > 0 {
			sb.WriteString("    " + strings.Join(dp, " | ") + "\n")
		}
	}
	sb.WriteString(fmt.Sprintf("\n<i>Updated %s WIB</i>", time.Now().In(WIB).Format("15:04")))
	return sb.String()
}

func fmtStart() string {
	return "<b>FF Economic Calendar</b>\n\n" +
		"Real-time Forex Factory events\nwith personalized auto-alerts.\n\n" +
		"Pick an option below \u2B07"
}

func fmtCalendarMenu() string {
	return "<b>\U0001F4C5 Calendar</b>\n\nChoose a view:"
}

func fmtSettingsMenu() string {
	return "<b>\u2699\uFE0F Settings</b>\n\nConfigure your personal alert preferences:"
}

func fmtTimingPage(p UserPrefs) string {
	var active []string
	for _, m := range p.AlertMinutes {
		active = append(active, fmt.Sprintf("%dm", m))
	}
	if len(active) == 0 {
		return "<b>\u23F0 Alert Timing</b>\n\nNo timing selected.\nTap to toggle:"
	}
	return fmt.Sprintf("<b>\u23F0 Alert Timing</b>\n\nActive: %s\nTap to toggle:", strings.Join(active, ", "))
}

func fmtImpactPage(p UserPrefs) string {
	var active []string
	for _, imp := range p.AlertImpacts {
		active = append(active, impactIcon(imp)+" "+imp)
	}
	if len(active) == 0 {
		return "<b>\U0001F4CA Impact Filter</b>\n\nNo impact levels selected.\nTap to toggle:"
	}
	return fmt.Sprintf("<b>\U0001F4CA Impact Filter</b>\n\nActive: %s\nTap to toggle:", strings.Join(active, ", "))
}

func (b *Bot) fmtStatus(userID int64) string {
	p := b.prefs.Get(userID)
	now := time.Now()
	up := 0
	for _, e := range b.events {
		t, err := parseEventTime(e.Date)
		if err != nil {
			continue
		}
		if t.After(now) {
			up++
		}
	}
	b.mu.RLock()
	lf := b.lastFetch.In(WIB).Format("15:04:05")
	b.mu.RUnlock()

	st := "\u2705 ON"
	if !p.AlertsOn {
		st = "\u274C OFF"
	}

	var mins []string
	for _, m := range p.AlertMinutes {
		mins = append(mins, fmt.Sprintf("%dm", m))
	}
	if len(mins) == 0 {
		mins = []string{"none"}
	}

	var imps []string
	for _, imp := range p.AlertImpacts {
		imps = append(imps, impactIcon(imp)+" "+imp)
	}
	if len(imps) == 0 {
		imps = []string{"none"}
	}

	return fmt.Sprintf(
		"<b>\U0001F4CB Bot Status</b>\n\n"+
			"Alerts: %s\n"+
			"Timing: %s\n"+
			"Impact: %s\n\n"+
			"Events: %d loaded, %d upcoming\n"+
			"Refresh: %s WIB\n"+
			"Interval: 2min active / 5min idle",
		st, strings.Join(mins, ", "), strings.Join(imps, ", "),
		len(b.events), up, lf)
}

// -- ALERT ENGINE (PER-USER) --------------------------------------------------

func (b *Bot) checkAlerts() {
	if b.chatID == "" {
		return
	}

	// Get all active users
	activeUsers := b.prefs.AllActive()

	// Also support legacy mode: if CHAT_ID is set but no users registered yet,
	// alert to CHAT_ID with default prefs
	if len(activeUsers) == 0 {
		// No user has interacted yet — use legacy CHAT_ID behavior
		b.checkAlertsForChat(b.chatID, defaultPrefs())
		return
	}

	// Per-user alerts
	for userID, prefs := range activeUsers {
		chatIDStr := strconv.FormatInt(userID, 10)
		b.checkAlertsForChat(chatIDStr, prefs)
	}

	// Also always alert to the configured CHAT_ID (group) with default prefs
	// if it's a group chat (negative ID)
	if strings.HasPrefix(b.chatID, "-") {
		b.checkAlertsForChat(b.chatID, defaultPrefs())
	}
}

func (b *Bot) checkAlertsForChat(chatID string, prefs UserPrefs) {
	b.mu.RLock()
	events := b.events
	b.mu.RUnlock()
	now := time.Now()

	for _, e := range events {
		// Impact filter: skip if this event's impact is not in user's preferences
		if !containsStr(prefs.AlertImpacts, e.Impact) && e.Actual == "" {
			continue
		}

		t, err := parseEventTime(e.Date)
		if err != nil {
			continue
		}
		key := eventKey(e)
		b.mu.RLock()
		state, ok := b.eventState[key]
		b.mu.RUnlock()
		if !ok {
			continue
		}

		diff := time.Until(t)
		minsLeft := int(diff.Minutes())

		// Pre-event alerts — only for minutes the user has enabled
		for _, am := range prefs.AlertMinutes {
			if minsLeft <= am && minsLeft > (am-2) && !state.AlertedMinutes[am] {
				wt := t.In(WIB).Format("15:04")
				urgency := "\U0001F514" // bell - 30min+
				if am <= 5 {
					urgency = "\U0001F6A8" // siren - 5min or less
				} else if am <= 15 {
					urgency = "\u26A0\uFE0F" // warning - 15min
				}

				msg := fmt.Sprintf(
					"%s <b>%dm before release</b>\n\n"+
						"%s %s %s\n"+
						"Time: %s WIB",
					urgency, minsLeft,
					impactIcon(e.Impact), flag(e.Country), e.Title, wt)

				if e.Forecast != "" {
					msg += fmt.Sprintf("\nFcst: %s", e.Forecast)
				}
				if e.Previous != "" {
					msg += fmt.Sprintf("\nPrev: %s", e.Previous)
				}

				if err := b.sendMessage(chatID, msg); err != nil {
					log.Printf("[ALERT] %v", err)
				} else {
					log.Printf("[ALERT] %dm: %s %s -> %s", am, e.Country, e.Title, chatID)
				}
				b.mu.Lock()
				state.AlertedMinutes[am] = true
				b.mu.Unlock()
			}
		}

		// Result alert — send to all who have this impact in their prefs
		if e.Actual != "" && !state.ActualSent && now.After(t) {
			if !containsStr(prefs.AlertImpacts, e.Impact) {
				continue
			}
			wt := t.In(WIB).Format("15:04")
			verdict, vIcon := "", ""
			if e.Forecast != "" {
				av := parseNumber(e.Actual)
				fv := parseNumber(e.Forecast)
				if av > fv {
					verdict, vIcon = "BEAT", "\u2705"
				} else if av < fv {
					verdict, vIcon = "MISS", "\u274C"
				} else {
					verdict, vIcon = "IN LINE", "\u2796"
				}
			}

			msg := fmt.Sprintf(
				"\U0001F4CA <b>RESULT</b>\n\n"+
					"%s %s %s\n"+
					"Time: %s WIB\n\n"+
					"Actual:   <b>%s</b>\n"+
					"Forecast: %s\n"+
					"Previous: %s",
				impactIcon(e.Impact), flag(e.Country), e.Title,
				wt, e.Actual, e.Forecast, e.Previous)

			if verdict != "" {
				msg += fmt.Sprintf("\n\n%s <b>%s</b>", vIcon, verdict)
			}

			if err := b.sendMessage(chatID, msg); err != nil {
				log.Printf("[RESULT] %v", err)
			} else {
				log.Printf("[RESULT] %s %s = %s -> %s", e.Country, e.Title, e.Actual, chatID)
			}
			b.mu.Lock()
			state.ActualSent = true
			b.mu.Unlock()
		}
	}
}

// -- COMMAND HANDLER ----------------------------------------------------------

func (b *Bot) handleCommand(chatID int64, userID int64, text string) {
	cid := fmt.Sprintf("%d", chatID)

	switch {
	case text == "/start" || text == "/help":
		b.sendWithKB(cid, fmtStart(), kbMain())

	case text == "/calendar":
		b.sendWithKB(cid, fmtCalendarMenu(), kbCalendarMenu())

	case text == "/today":
		b.mu.RLock()
		events := b.events
		b.mu.RUnlock()
		msg := fmtToday(events)
		b.safeSend(cid, msg, kbCalendarBack())

	case text == "/high":
		b.mu.RLock()
		events := b.events
		b.mu.RUnlock()
		b.sendWithKB(cid, fmtHigh(events), kbCalendarBack())

	case text == "/next":
		b.mu.RLock()
		events := b.events
		b.mu.RUnlock()
		b.sendWithKB(cid, fmtNext(events, 10), kbCalendarBack())

	case text == "/settings":
		b.sendWithKB(cid, fmtSettingsMenu(), kbSettingsMenu())


	case text == "/chatid":
		b.sendWithKB(cid, fmt.Sprintf("Chat ID: <code>%d</code>", chatID), kbBack())
	}
}

// -- CALLBACK HANDLER ---------------------------------------------------------

func (b *Bot) handleCallback(cb *TGCallback) {
	if cb.Message == nil {
		b.answerCB(cb.ID, "")
		return
	}
	cid := fmt.Sprintf("%d", cb.Message.Chat.ID)
	mid := cb.Message.MessageID
	userID := cb.From.ID

	b.mu.RLock()
	events := b.events
	b.mu.RUnlock()

	var text string
	var kb *InlineKeyboardMarkup

	switch {
	// -- Main menu
	case cb.Data == "start":
		text = fmtStart()
		k := kbMain()
		kb = &k

	// -- Calendar sub-menu
	case cb.Data == "menu_cal":
		text = fmtCalendarMenu()
		k := kbCalendarMenu()
		kb = &k

	case cb.Data == "cal_today":
		text = fmtToday(events)
		k := kbCalendarBack()
		kb = &k

	case cb.Data == "cal_week":
		text = fmtWeek(events)
		k := kbCalendarBack()
		kb = &k

	case cb.Data == "cal_high":
		text = fmtHigh(events)
		k := kbCalendarBack()
		kb = &k

	case cb.Data == "cal_next":
		text = fmtNext(events, 10)
		k := kbCalendarBack()
		kb = &k

	case cb.Data == "cal_refresh":
		b.answerCB(cb.ID, "Refresh disabled")
		return

	// -- Settings sub-menu
	case cb.Data == "menu_settings":
		text = fmtSettingsMenu()
		k := kbSettingsMenu()
		kb = &k

	case cb.Data == "set_timing":
		p := b.prefs.Get(userID)
		text = fmtTimingPage(p)
		k := kbTimingToggles(p)
		kb = &k

	case cb.Data == "set_impact":
		p := b.prefs.Get(userID)
		text = fmtImpactPage(p)
		k := kbImpactToggles(p)
		kb = &k

	case cb.Data == "set_toggle":
		p := b.prefs.ToggleAlerts(userID)
		status := "\u2705 Alerts ON"
		if !p.AlertsOn {
			status = "\u274C Alerts OFF"
		}
		b.answerCB(cb.ID, status)
		text = b.fmtStatus(userID)
		k := kbSettingsMenu()
		kb = &k
		b.editMsg(cid, mid, text, kb)
		return

	case cb.Data == "set_status":
		text = b.fmtStatus(userID)
		k := kbSettingsMenu()
		kb = &k

	// -- Timing toggles (tog_min_60, tog_min_30, etc.)
	case strings.HasPrefix(cb.Data, "tog_min_"):
		minStr := strings.TrimPrefix(cb.Data, "tog_min_")
		minVal, err := strconv.Atoi(minStr)
		if err != nil {
			b.answerCB(cb.ID, "Invalid")
			return
		}
		p := b.prefs.ToggleMinute(userID, minVal)
		b.answerCB(cb.ID, fmt.Sprintf("%dm toggled", minVal))
		text = fmtTimingPage(p)
		k := kbTimingToggles(p)
		kb = &k
		b.editMsg(cid, mid, text, kb)
		return

	// -- Impact toggles (tog_imp_high, tog_imp_medium, tog_imp_low)
	case strings.HasPrefix(cb.Data, "tog_imp_"):
		impact := strings.TrimPrefix(cb.Data, "tog_imp_")
		// Capitalize first letter
		impact = strings.ToUpper(impact[:1]) + impact[1:]
		p := b.prefs.ToggleImpact(userID, impact)
		b.answerCB(cb.ID, impact+" toggled")
		text = fmtImpactPage(p)
		k := kbImpactToggles(p)
		kb = &k
		b.editMsg(cid, mid, text, kb)
		return

	default:
		b.answerCB(cb.ID, "")
		return
	}

	// Handle long messages — send new instead of edit
	if len(text) > 4000 {
		b.answerCB(cb.ID, "")
		chunks := splitMessage(text, 4000)
		for i, c := range chunks {
			if i == len(chunks)-1 && kb != nil {
				b.sendWithKB(cid, c, *kb)
			} else {
				b.sendMessage(cid, c)
			}
			time.Sleep(100 * time.Millisecond)
		}
		return
	}

	b.answerCB(cb.ID, "")
	b.editMsg(cid, mid, text, kb)
}

// -- SAFE SEND (auto-split long messages) ------------------------------------

func (b *Bot) safeSend(chatID, text string, kb InlineKeyboardMarkup) {
	if len(text) > 4000 {
		chunks := splitMessage(text, 4000)
		for i, c := range chunks {
			if i == len(chunks)-1 {
				b.sendWithKB(chatID, c, kb)
			} else {
				b.sendMessage(chatID, c)
			}
			time.Sleep(100 * time.Millisecond)
		}
	} else {
		b.sendWithKB(chatID, text, kb)
	}
}

// -- MAIN LOOP ----------------------------------------------------------------

func (b *Bot) run() {
	log.Println("[BOT] Starting ARK Community Intelligent v1...")


	// Alert check every 30s
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			b.checkAlerts()
		}
	}()

	// Poll commands + callbacks
	log.Println("[BOT] Polling...")
	for {
		updates, err := b.getUpdates()
		if err != nil {
			log.Printf("[POLL] %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, u := range updates {
			if u.UpdateID >= b.offset {
				b.offset = u.UpdateID + 1
			}
			if u.CallbackQuery != nil {
				go b.handleCallback(u.CallbackQuery)
				continue
			}
			if u.Message != nil && strings.HasPrefix(u.Message.Text, "/") {
				cmd := u.Message.Text
				if idx := strings.Index(cmd, "@"); idx != -1 {
					cmd = cmd[:idx]
				}
				var uid int64
				if u.Message.From != nil {
					uid = u.Message.From.ID
				}
				go b.handleCommand(u.Message.Chat.ID, uid, cmd)
			}
		}
	}
}

// -- HELPERS ------------------------------------------------------------------

func parseNumber(s string) float64 {
	s = strings.TrimSpace(s)
	for _, r := range []string{"%", "K", "M", "B", "T", ","} {
		s = strings.Replace(s, r, "", -1)
	}
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

func splitMessage(text string, maxLen int) []string {
	var chunks []string
	lines := strings.Split(text, "\n")
	cur := ""
	for _, line := range lines {
		if len(cur)+len(line)+1 > maxLen {
			if cur != "" {
				chunks = append(chunks, cur)
			}
			cur = line
		} else {
			if cur != "" {
				cur += "\n"
			}
			cur += line
		}
	}
	if cur != "" {
		chunks = append(chunks, cur)
	}
	return chunks
}

// -- ENTRYPOINT ---------------------------------------------------------------

func main() {
	var err error
	WIB, err = time.LoadLocation("Asia/Jakarta")
	if err != nil {
		WIB = time.FixedZone("WIB", 7*60*60)
	}
	if BOT_TOKEN == "" {
		log.Fatal("[FATAL] BOT_TOKEN required")
	}
	if CHAT_ID == "" {
		log.Println("[WARN] CHAT_ID not set -- alerts go to individual users only")
	}

	// Ensure prefs directory exists
	if dir := prefsDir(PREFS_FILE); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	bot := NewBot(BOT_TOKEN, CHAT_ID)
	bot.run()
}

func prefsDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return ""
}
