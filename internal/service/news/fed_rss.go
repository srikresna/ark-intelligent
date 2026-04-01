package news

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var fedLog = logger.Component("fed-rss")

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// FedSpeech represents a single Federal Reserve speech or FOMC press release
// parsed from the Fed's RSS feeds.
type FedSpeech struct {
	Title       string
	Description string    // venue/context
	URL         string
	GUID        string
	Category    string    // "Speech" | "Press Release" | "Minutes"
	Speaker     string    // parsed from Title (e.g. "Powell")
	IsVoting    bool      // true if voting member
	PublishedAt time.Time
}

// FedAlertLevel describes the alert priority for a Fed speech/release.
type FedAlertLevel string

const (
	FedAlertCritical FedAlertLevel = "CRITICAL" // FOMC Statement
	FedAlertHigh     FedAlertLevel = "HIGH"     // FOMC Minutes or Voting member speech
	FedAlertMedium   FedAlertLevel = "MEDIUM"   // Non-voting member speech
)

// AlertLevel returns the alert priority level for this speech.
func (f *FedSpeech) AlertLevel() FedAlertLevel {
	if strings.Contains(strings.ToLower(f.Title), "fomc statement") ||
		strings.Contains(strings.ToLower(f.Title), "monetary policy statement") {
		return FedAlertCritical
	}
	cat := strings.ToLower(f.Category)
	if strings.Contains(cat, "minutes") ||
		strings.Contains(strings.ToLower(f.Title), "minutes") {
		return FedAlertHigh
	}
	if f.IsVoting {
		return FedAlertHigh
	}
	return FedAlertMedium
}

// ---------------------------------------------------------------------------
// RSS XML structs
// ---------------------------------------------------------------------------

type rssRoot struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	GUID        string `xml:"guid"`
	Category    string `xml:"category"`
	PubDate     string `xml:"pubDate"`
}

// ---------------------------------------------------------------------------
// Voting members (as of 2026)
// ---------------------------------------------------------------------------

// votingMembers lists FOMC voting members by last name (lowercase).
// Includes Powell, Vice Chair Jefferson, and permanent/rotating voters.
var votingMembers = map[string]bool{
	"powell":    true,
	"jefferson": true,
	"waller":    true,
	"cook":      true,
	"kugler":    true,
	"barr":      true,
	// 2026 rotating presidents (Chicago, Boston, St. Louis, Kansas City)
	"goolsbee":  true,
	"collins":   true,
	"musalem":   true,
	"schmid":    true,
}

const (
	fedSpeechesURL = "https://www.federalreserve.gov/feeds/speeches.xml"
	fedFOMCURL     = "https://www.federalreserve.gov/feeds/press_monetary.xml"

	fedFetchTimeout = 15 * time.Second
)

// ---------------------------------------------------------------------------
// parseSpeaker extracts the speaker's last name from a Fed speech title.
// Titles typically have the format "LastName, Topic..." or "Last, First Topic..."
// ---------------------------------------------------------------------------
func parseSpeaker(title string) string {
	if title == "" {
		return ""
	}
	// Pattern 1: "LastName, Remarks on..." → split by comma
	commaIdx := strings.Index(title, ",")
	if commaIdx > 0 {
		candidate := strings.TrimSpace(title[:commaIdx])
		// Only accept if single word (last name only) or two words max
		parts := strings.Fields(candidate)
		if len(parts) >= 1 && len(parts) <= 2 {
			return parts[0] // return last name
		}
	}
	// Pattern 2: no comma — try first word
	parts := strings.Fields(title)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// ---------------------------------------------------------------------------
// fetchFedRSS fetches and parses a Fed RSS feed URL.
// ---------------------------------------------------------------------------
func fetchFedRSS(ctx context.Context, feedURL string) ([]FedSpeech, error) {
	client := &http.Client{Timeout: fedFetchTimeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fed rss: create request: %w", err)
	}
	req.Header.Set("User-Agent", "ark-intelligent/1.0 (market-bot)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fed rss: fetch %s: %w", feedURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fed rss: unexpected status %d for %s", resp.StatusCode, feedURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit
	if err != nil {
		return nil, fmt.Errorf("fed rss: read body: %w", err)
	}

	var root rssRoot
	if err := xml.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("fed rss: parse xml: %w", err)
	}

	var speeches []FedSpeech
	for _, item := range root.Channel.Items {
		pubAt, _ := time.Parse(time.RFC1123Z, item.PubDate)
		if pubAt.IsZero() {
			// Try alternate RFC1123 format without timezone offset
			pubAt, _ = time.Parse(time.RFC1123, item.PubDate)
		}

		speaker := parseSpeaker(item.Title)
		isVoting := votingMembers[strings.ToLower(speaker)]

		// Determine category from feed item
		cat := item.Category
		titleLow := strings.ToLower(item.Title)
		if cat == "" {
			if strings.Contains(titleLow, "minutes") {
				cat = "Minutes"
			} else if strings.Contains(titleLow, "statement") {
				cat = "Press Release"
			} else {
				cat = "Speech"
			}
		}

		// Clean up description (strip HTML tags if any)
		desc := strings.TrimSpace(item.Description)
		desc = strings.ReplaceAll(desc, "<![CDATA[", "")
		desc = strings.ReplaceAll(desc, "]]>", "")

		guid := item.GUID
		if guid == "" {
			guid = item.Link
		}

		speeches = append(speeches, FedSpeech{
			Title:       strings.TrimSpace(item.Title),
			Description: desc,
			URL:         strings.TrimSpace(item.Link),
			GUID:        guid,
			Category:    cat,
			Speaker:     speaker,
			IsVoting:    isVoting,
			PublishedAt: pubAt,
		})
	}

	return speeches, nil
}

// ---------------------------------------------------------------------------
// Public fetch functions
// ---------------------------------------------------------------------------

// FetchFedSpeeches fetches the Fed Speeches RSS feed.
func FetchFedSpeeches(ctx context.Context) ([]FedSpeech, error) {
	return fetchFedRSS(ctx, fedSpeechesURL)
}

// FetchFOMCPress fetches the FOMC Monetary Policy RSS feed.
func FetchFOMCPress(ctx context.Context) ([]FedSpeech, error) {
	return fetchFedRSS(ctx, fedFOMCURL)
}

// ---------------------------------------------------------------------------
// Fed RSS Scheduler
// ---------------------------------------------------------------------------

// FedRSSScheduler polls the Fed RSS feeds every 30 minutes and dispatches
// alerts for new speeches/press releases.
type FedRSSScheduler struct {
	mu            sync.Mutex
	lastSeenGUIDs map[string]bool // key: GUID, value: true = already seen
	isBanned      func(ctx context.Context, userID int64) bool
	alertSink     func(ctx context.Context, html string, level FedAlertLevel)
}

// NewFedRSSScheduler creates a new FedRSSScheduler.
func NewFedRSSScheduler() *FedRSSScheduler {
	return &FedRSSScheduler{
		lastSeenGUIDs: make(map[string]bool),
	}
}

// SetIsBannedFunc sets the ban-check callback.
func (f *FedRSSScheduler) SetIsBannedFunc(fn func(ctx context.Context, userID int64) bool) {
	f.isBanned = fn
}

// Start begins the 30-minute polling loop.
func (f *FedRSSScheduler) Start(ctx context.Context) {
	fedLog.Info().Msg("starting Fed RSS monitor (30min interval)")
	go f.runLoop(ctx)
}

func (f *FedRSSScheduler) runLoop(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			fedLog.Error().Interface("panic", r).Msg("PANIC in FedRSSScheduler.runLoop")
		}
	}()

	// Run immediately on startup, then every 30 minutes.
	f.poll(ctx)

	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.poll(ctx)
		}
	}
}

// poll fetches both feeds and dispatches alerts for new items.
func (f *FedRSSScheduler) poll(ctx context.Context) {
	cutoff := time.Now().UTC().Add(-24 * time.Hour)

	speeches, err := FetchFedSpeeches(ctx)
	if err != nil {
		fedLog.Error().Err(err).Msg("FetchFedSpeeches failed")
	} else {
		f.processItems(ctx, speeches, cutoff)
	}

	fomc, err := FetchFOMCPress(ctx)
	if err != nil {
		fedLog.Error().Err(err).Msg("FetchFOMCPress failed")
	} else {
		f.processItems(ctx, fomc, cutoff)
	}
}

// processItems checks for new items and broadcasts alerts.
func (f *FedRSSScheduler) processItems(ctx context.Context, items []FedSpeech, cutoff time.Time) {
	for _, item := range items {
		// Skip items older than 24 hours (avoid flood on restart).
		if !item.PublishedAt.IsZero() && item.PublishedAt.Before(cutoff) {
			continue
		}

		f.mu.Lock()
		seen := f.lastSeenGUIDs[item.GUID]
		if !seen {
			f.lastSeenGUIDs[item.GUID] = true
		}
		f.mu.Unlock()

		if seen {
			continue
		}

		level := item.AlertLevel()
		fedLog.Info().
			Str("speaker", item.Speaker).
			Str("title", item.Title).
			Str("level", string(level)).
			Msg("new Fed speech/press detected")

		f.broadcastFedAlert(ctx, item, level)
	}
}

// broadcastFedAlert formats and dispatches an alert via the registered alertSink.
func (f *FedRSSScheduler) broadcastFedAlert(ctx context.Context, speech FedSpeech, level FedAlertLevel) {
	if f.alertSink == nil {
		return
	}
	html := FormatFedAlert(speech, level)
	f.alertSink(ctx, html, level)
}

// SetAlertSink registers a callback that receives formatted HTML alerts.
// The callback is responsible for broadcasting to users.
func (f *FedRSSScheduler) SetAlertSink(fn func(ctx context.Context, html string, level FedAlertLevel)) {
	f.alertSink = fn
}

// ---------------------------------------------------------------------------
// FormatFedAlert — Telegram HTML formatter
// ---------------------------------------------------------------------------

// FormatFedAlert formats a FedSpeech into a Telegram HTML alert string.
func FormatFedAlert(speech FedSpeech, level FedAlertLevel) string {
	var icon, header string
	switch level {
	case FedAlertCritical:
		icon = "🚨"
		header = "FOMC Statement Released"
	case FedAlertHigh:
		titleLow := strings.ToLower(speech.Title)
		if strings.Contains(titleLow, "minutes") {
			icon = "📋"
			header = "FOMC Minutes Released"
		} else {
			icon = "🏛️"
			header = "Fed Alert"
		}
	default:
		icon = "🏛️"
		header = "Fed Speech"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s <b>%s</b>\n", icon, header))

	if speech.Speaker != "" {
		sb.WriteString(fmt.Sprintf("Speaker: <b>%s</b>", speech.Speaker))
		if speech.IsVoting {
			sb.WriteString(" ⭐")
		}
		sb.WriteString("\n")
	}

	if speech.Title != "" {
		// Truncate long titles
		title := speech.Title
		if len(title) > 100 {
			title = title[:97] + "..."
		}
		sb.WriteString(fmt.Sprintf("Topic: <i>%s</i>\n", title))
	}

	if speech.Description != "" {
		desc := speech.Description
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}
		sb.WriteString(fmt.Sprintf("Venue: %s\n", desc))
	}

	if !speech.PublishedAt.IsZero() {
		wib := speech.PublishedAt.UTC().Add(7 * time.Hour)
		sb.WriteString(fmt.Sprintf("Time: %s WIB\n", wib.Format("02 Jan 15:04")))
	}

	if speech.URL != "" {
		sb.WriteString(fmt.Sprintf("🔗 <a href=\"%s\">Link</a>", speech.URL))
	}

	return sb.String()
}
