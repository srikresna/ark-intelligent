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

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var fedLog = logger.Component("fed-rss")

// Fed RSS feed URLs (free, no API key required).
const (
	FedSpeechesURL = "https://www.federalreserve.gov/feeds/speeches.xml"
	FedFOMCPressURL = "https://www.federalreserve.gov/feeds/press_monetary.xml"
)

// FedSpeech represents a single Fed speech or FOMC press release parsed from RSS.
type FedSpeech struct {
	Title       string    // Full title from RSS
	Description string    // Venue/context from <description>
	URL         string    // Full link to the speech/release
	Category    string    // "Speech" | "Press Release" | "Minutes" | "Statement"
	Speaker     string    // Parsed from title (e.g. "Powell", "Barr")
	IsVoting    bool      // true if speaker is a current FOMC voting member
	PublishedAt time.Time // Parsed from <pubDate>
}

// rssDoc represents the top-level RSS XML structure.
type rssDoc struct {
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
	Category    string `xml:"category"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// votingMembers is the list of current FOMC voting members (2026 rotation).
// Includes permanent voters (Board of Governors) + rotating Reserve Bank presidents.
// Update this list annually when the FOMC rotation changes (January each year).
var votingMembers = map[string]bool{
	"powell":    true, // Chair
	"jefferson": true, // Vice Chair
	"barr":     true, // Vice Chair Supervision
	"waller":   true, // Governor
	"cook":     true, // Governor
	"kugler":   true, // Governor
	"bowman":   true, // Governor
	// 2026 rotating voters (update annually):
	"bostic":    true, // Atlanta
	"collins":   true, // Boston
	"musalem":   true, // St. Louis
	"hammack":   true, // Cleveland
}

// FetchFedSpeeches fetches and parses the Fed speeches RSS feed.
func FetchFedSpeeches(ctx context.Context) ([]FedSpeech, error) {
	return fetchFedRSS(ctx, FedSpeechesURL, "Speech")
}

// FetchFOMCPress fetches and parses the FOMC monetary policy press RSS feed.
func FetchFOMCPress(ctx context.Context) ([]FedSpeech, error) {
	return fetchFedRSS(ctx, FedFOMCPressURL, "")
}

func fetchFedRSS(ctx context.Context, url, defaultCategory string) ([]FedSpeech, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fed-rss: create request: %w", err)
	}
	req.Header.Set("User-Agent", "ark-intelligent/1.0 (forex analytics bot)")

	client := httpclient.New(httpclient.WithTimeout(15 * time.Second))
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fed-rss: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fed-rss: %s returned HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	if err != nil {
		return nil, fmt.Errorf("fed-rss: read body: %w", err)
	}

	var doc rssDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("fed-rss: parse XML: %w", err)
	}

	var speeches []FedSpeech
	for _, item := range doc.Channel.Items {
		pubDate, parseErr := time.Parse(time.RFC1123, item.PubDate)
		if parseErr != nil {
			// Try alternate format
			pubDate, parseErr = time.Parse(time.RFC1123Z, item.PubDate)
			if parseErr != nil {
				// Try yet another common RSS format
				pubDate, parseErr = time.Parse("Mon, 02 Jan 2006 15:04:05 MST", item.PubDate)
				if parseErr != nil {
					fedLog.Warn().Str("pubDate", item.PubDate).Msg("skip: unparseable pubDate")
					continue
				}
			}
		}

		category := item.Category
		if category == "" {
			category = defaultCategory
		}
		category = classifyFedCategory(category, item.Title)

		speaker := parseSpeaker(item.Title)
		isVoting := speaker != "" && votingMembers[strings.ToLower(speaker)]

		speeches = append(speeches, FedSpeech{
			Title:       item.Title,
			Description: item.Description,
			URL:         item.Link,
			Category:    category,
			Speaker:     speaker,
			IsVoting:    isVoting,
			PublishedAt: pubDate.UTC(),
		})
	}

	fedLog.Debug().Int("count", len(speeches)).Str("feed", url).Msg("parsed Fed RSS")
	return speeches, nil
}

// parseSpeaker extracts the speaker name from a title like:
//   "Barr, Brief Remarks on the Economic Outlook" → "Barr"
//   "Chair Powell, Press Conference" → "Powell"
//   "Vice Chair Jefferson, Speech at..." → "Jefferson"
func parseSpeaker(title string) string {
	// Remove common prefixes
	t := title
	for _, prefix := range []string{
		"Chair ", "Vice Chair for Supervision ",
		"Vice Chair ", "Governor ", "President ",
	} {
		t = strings.TrimPrefix(t, prefix)
	}

	// Speaker name is before the first comma
	if idx := strings.Index(t, ","); idx > 0 {
		name := strings.TrimSpace(t[:idx])
		// Sanity check: name should be 1-2 words
		words := strings.Fields(name)
		if len(words) >= 1 && len(words) <= 3 {
			return words[len(words)-1] // Last word = surname
		}
	}
	return ""
}

// classifyFedCategory refines the category based on title content.
func classifyFedCategory(raw, title string) string {
	lower := strings.ToLower(title)
	switch {
	case strings.Contains(lower, "fomc statement"):
		return "Statement"
	case strings.Contains(lower, "minutes"):
		return "Minutes"
	case strings.Contains(lower, "press conference"):
		return "Press Conference"
	case strings.Contains(lower, "summary of economic projections"),
		strings.Contains(lower, "dot plot"),
		strings.Contains(lower, "sep"):
		return "SEP"
	case raw == "Speech" || strings.Contains(lower, "speech") ||
		strings.Contains(lower, "remarks") || strings.Contains(lower, "testimony"):
		return "Speech"
	default:
		if raw != "" {
			return raw
		}
		return "Press Release"
	}
}

// --------------------------------------------------------------------------
// FedRSSTracker — stateful tracker for detecting new items
// --------------------------------------------------------------------------

// FedRSSTracker manages last-seen state for Fed RSS feeds to detect new items.
type FedRSSTracker struct {
	mu       sync.Mutex
	seenURLs map[string]bool // URLs we've already alerted on
	initDone bool            // false until first successful poll (skip alerts on first load)
}

// NewFedRSSTracker creates a new tracker.
func NewFedRSSTracker() *FedRSSTracker {
	return &FedRSSTracker{
		seenURLs: make(map[string]bool),
	}
}

// Update processes a batch of speeches and returns only the NEW ones.
// On the very first call, all items are marked as "seen" but NOT returned
// (to avoid flooding users on bot startup).
func (t *FedRSSTracker) Update(speeches []FedSpeech) []FedSpeech {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.initDone {
		// First run: seed the tracker with all current items, return nothing.
		for _, s := range speeches {
			t.seenURLs[s.URL] = true
		}
		t.initDone = true
		fedLog.Info().Int("seeded", len(speeches)).Msg("Fed RSS tracker initialized")
		return nil
	}

	var newItems []FedSpeech
	for _, s := range speeches {
		if !t.seenURLs[s.URL] {
			// Only alert for items published within last 24 hours
			if time.Since(s.PublishedAt) <= 24*time.Hour {
				newItems = append(newItems, s)
			}
			t.seenURLs[s.URL] = true
		}
	}
	return newItems
}

// FormatFedAlert formats a FedSpeech into an HTML alert for Telegram.
func FormatFedAlert(s FedSpeech) string {
	var b strings.Builder

	switch s.Category {
	case "Statement":
		b.WriteString("🚨 <b>FOMC Statement Released</b>\n\n")
	case "Minutes":
		b.WriteString("📋 <b>FOMC Minutes Released</b>\n\n")
	case "SEP":
		b.WriteString("📊 <b>Fed SEP / Dot Plot Released</b>\n\n")
	case "Press Conference":
		b.WriteString("🎙️ <b>Fed Press Conference</b>\n\n")
	default:
		if s.IsVoting {
			b.WriteString("🏛️ <b>Fed Speech Alert</b>\n\n")
		} else {
			b.WriteString("🏛️ <b>Fed Speech</b>\n\n")
		}
	}

	if s.Speaker != "" {
		voting := ""
		if s.IsVoting {
			voting = " (Voting)"
		}
		b.WriteString(fmt.Sprintf("<b>Speaker:</b> %s%s\n", s.Speaker, voting))
	}

	b.WriteString(fmt.Sprintf("<b>Topic:</b> %s\n", s.Title))

	if s.Description != "" {
		b.WriteString(fmt.Sprintf("<b>Venue:</b> %s\n", s.Description))
	}

	b.WriteString(fmt.Sprintf("<b>Date:</b> %s\n", s.PublishedAt.Format("Jan 02, 2006 15:04 UTC")))
	b.WriteString(fmt.Sprintf("\n🔗 <a href=\"%s\">Read Full Text</a>", s.URL))

	return b.String()
}

// --------------------------------------------------------------------------
// Scheduler integration — Fed RSS polling loop
// --------------------------------------------------------------------------

// runFedRSSLoop polls Fed RSS feeds every 30 minutes and alerts users on new items.
func (s *Scheduler) runFedRSSLoop(ctx context.Context) {
	tracker := NewFedRSSTracker()

	// Initial seed run (no alerts, just learn current state)
	s.pollFedRSS(ctx, tracker)

	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollFedRSS(ctx, tracker)
		}
	}
}

// pollFedRSS fetches both RSS feeds and broadcasts alerts for new items.
func (s *Scheduler) pollFedRSS(ctx context.Context, tracker *FedRSSTracker) {
	var allSpeeches []FedSpeech

	speeches, err := FetchFedSpeeches(ctx)
	if err != nil {
		fedLog.Error().Err(err).Msg("failed to fetch Fed speeches RSS")
	} else {
		allSpeeches = append(allSpeeches, speeches...)
	}

	fomc, err := FetchFOMCPress(ctx)
	if err != nil {
		fedLog.Error().Err(err).Msg("failed to fetch FOMC press RSS")
	} else {
		allSpeeches = append(allSpeeches, fomc...)
	}

	newItems := tracker.Update(allSpeeches)
	if len(newItems) == 0 {
		return
	}

	fedLog.Info().Int("new", len(newItems)).Msg("new Fed items detected")

	// Store latest speeches for AI context enrichment
	s.latestFedMu.Lock()
	s.latestFedSpeeches = allSpeeches
	s.latestFedMu.Unlock()

	// Broadcast alerts to all active users
	allPrefs, err := s.prefsRepo.GetAllActive(ctx)
	if err != nil {
		fedLog.Error().Err(err).Msg("failed to get active prefs for Fed alerts")
		return
	}

	for _, item := range newItems {
		html := FormatFedAlert(item)

		for _, prefs := range allPrefs {
			if prefs.ChatID == "" || !prefs.AlertsEnabled {
				continue
			}
			if _, sendErr := s.messenger.SendHTML(ctx, prefs.ChatID, html); sendErr != nil {
				fedLog.Error().Str("chatID", prefs.ChatID).Err(sendErr).Msg("failed to send Fed alert")
			}
			time.Sleep(config.TelegramFloodDelay)
		}
	}
}

// LatestFedSpeeches returns the most recent Fed speeches (up to n, max 5 days old).
// Thread-safe. Used by AI context builder.
func (s *Scheduler) LatestFedSpeeches(n int) []FedSpeech {
	s.latestFedMu.RLock()
	defer s.latestFedMu.RUnlock()

	cutoff := time.Now().UTC().Add(-5 * 24 * time.Hour)
	var result []FedSpeech
	for _, sp := range s.latestFedSpeeches {
		if sp.PublishedAt.After(cutoff) {
			result = append(result, sp)
			if len(result) >= n {
				break
			}
		}
	}
	return result
}
