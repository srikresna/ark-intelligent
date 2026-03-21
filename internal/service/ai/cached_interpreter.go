package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

var clog = logger.Component("ai-cache")

// CachedInterpreter wraps an AIAnalyzer with BadgerDB-backed caching.
// Cache keys embed data versions (report dates, week starts) so when
// underlying data updates, the key changes and triggers a cache miss.
// All entries have a 7-day TTL as a safety net for stale data.
//
// AnalyzeActualRelease is NOT cached — each economic release is unique
// and time-sensitive.
type CachedInterpreter struct {
	inner ports.AIAnalyzer
	cache ports.AICacheRepository
}

// NewCachedInterpreter creates a caching decorator around an AIAnalyzer.
// If cache is nil, all calls pass through directly to inner.
func NewCachedInterpreter(inner ports.AIAnalyzer, cache ports.AICacheRepository) *CachedInterpreter {
	return &CachedInterpreter{inner: inner, cache: cache}
}

// Ensure CachedInterpreter implements ports.AIAnalyzer at compile time.
var _ ports.AIAnalyzer = (*CachedInterpreter)(nil)

// IsAvailable delegates to the inner analyzer.
func (c *CachedInterpreter) IsAvailable() bool {
	return c.inner.IsAvailable()
}

// AnalyzeCOT caches by latest COT report date.
func (c *CachedInterpreter) AnalyzeCOT(ctx context.Context, analyses []domain.COTAnalysis) (string, error) {
	return c.AnalyzeCOTWithPrice(ctx, analyses, nil)
}

// AnalyzeCOTWithPrice caches by latest COT report date + price availability.
func (c *CachedInterpreter) AnalyzeCOTWithPrice(ctx context.Context, analyses []domain.COTAnalysis, priceCtx map[string]*domain.PriceContext) (string, error) {
	if len(analyses) == 0 || c.cache == nil {
		return c.inner.AnalyzeCOTWithPrice(ctx, analyses, priceCtx)
	}

	version := latestReportDate(analyses)
	priceSuffix := "np"
	if len(priceCtx) > 0 {
		priceSuffix = "wp"
	}
	key := fmt.Sprintf("aicache:cot:%s:%s", version, priceSuffix)

	if cached, ok := c.cache.Get(ctx, key); ok {
		clog.Debug().Str("key", key).Msg("cache HIT")
		return cached, nil
	}

	result, err := c.inner.AnalyzeCOTWithPrice(ctx, analyses, priceCtx)
	if err != nil {
		return result, err
	}

	if sErr := c.cache.Set(ctx, key, result, "cot", version); sErr != nil {
		clog.Warn().Err(sErr).Str("key", key).Msg("store failed")
	} else {
		clog.Debug().Str("key", key).Msg("cache STORE")
	}
	return result, nil
}

// GenerateWeeklyOutlook caches by week start + language + latest COT report date.
// Including the report date ensures the cache auto-misses when new COT data arrives
// mid-week, even before explicit invalidation.
func (c *CachedInterpreter) GenerateWeeklyOutlook(ctx context.Context, data ports.WeeklyData) (string, error) {
	if c.cache == nil {
		return c.inner.GenerateWeeklyOutlook(ctx, data)
	}

	weekVer := currentWeekStart()
	cotVer := latestReportDate(data.COTAnalyses)
	lang := data.Language
	if lang == "" {
		lang = "id"
	}
	key := fmt.Sprintf("aicache:weekly:%s:%s:%s", weekVer, cotVer, lang)

	if cached, ok := c.cache.Get(ctx, key); ok {
		clog.Debug().Str("key", key).Msg("cache HIT")
		return cached, nil
	}

	result, err := c.inner.GenerateWeeklyOutlook(ctx, data)
	if err != nil {
		return result, err
	}

	if sErr := c.cache.Set(ctx, key, result, "weekly", weekVer+":"+cotVer); sErr != nil {
		clog.Warn().Err(sErr).Str("key", key).Msg("store failed")
	} else {
		clog.Debug().Str("key", key).Msg("cache STORE")
	}
	return result, nil
}

// AnalyzeCrossMarket caches by latest COT report date from the data.
func (c *CachedInterpreter) AnalyzeCrossMarket(ctx context.Context, cotData map[string]*domain.COTAnalysis) (string, error) {
	if len(cotData) == 0 || c.cache == nil {
		return c.inner.AnalyzeCrossMarket(ctx, cotData)
	}

	version := latestReportDateFromMap(cotData)
	key := fmt.Sprintf("aicache:cross:%s", version)

	if cached, ok := c.cache.Get(ctx, key); ok {
		clog.Debug().Str("key", key).Msg("cache HIT")
		return cached, nil
	}

	result, err := c.inner.AnalyzeCrossMarket(ctx, cotData)
	if err != nil {
		return result, err
	}

	if sErr := c.cache.Set(ctx, key, result, "cross", version); sErr != nil {
		clog.Warn().Err(sErr).Str("key", key).Msg("store failed")
	} else {
		clog.Debug().Str("key", key).Msg("cache STORE")
	}
	return result, nil
}

// AnalyzeNewsOutlook caches by week start + language.
func (c *CachedInterpreter) AnalyzeNewsOutlook(ctx context.Context, events []domain.NewsEvent, lang string) (string, error) {
	if len(events) == 0 || c.cache == nil {
		return c.inner.AnalyzeNewsOutlook(ctx, events, lang)
	}

	version := currentWeekStart()
	if lang == "" {
		lang = "id"
	}
	key := fmt.Sprintf("aicache:news:%s:%s", version, lang)

	if cached, ok := c.cache.Get(ctx, key); ok {
		clog.Debug().Str("key", key).Msg("cache HIT")
		return cached, nil
	}

	result, err := c.inner.AnalyzeNewsOutlook(ctx, events, lang)
	if err != nil {
		return result, err
	}

	if sErr := c.cache.Set(ctx, key, result, "news", version); sErr != nil {
		clog.Warn().Err(sErr).Str("key", key).Msg("store failed")
	} else {
		clog.Debug().Str("key", key).Msg("cache STORE")
	}
	return result, nil
}

// AnalyzeCombinedOutlook caches by week start + language + FRED availability.
func (c *CachedInterpreter) AnalyzeCombinedOutlook(ctx context.Context, data ports.WeeklyData) (string, error) {
	if c.cache == nil {
		return c.inner.AnalyzeCombinedOutlook(ctx, data)
	}

	version := currentWeekStart()
	lang := data.Language
	if lang == "" {
		lang = "id"
	}
	hasFred := "nf"
	if data.MacroData != nil {
		hasFred = "wf"
	}
	key := fmt.Sprintf("aicache:combined:%s:%s:%s", version, lang, hasFred)

	if cached, ok := c.cache.Get(ctx, key); ok {
		clog.Debug().Str("key", key).Msg("cache HIT")
		return cached, nil
	}

	result, err := c.inner.AnalyzeCombinedOutlook(ctx, data)
	if err != nil {
		return result, err
	}

	if sErr := c.cache.Set(ctx, key, result, "combined", version); sErr != nil {
		clog.Warn().Err(sErr).Str("key", key).Msg("store failed")
	} else {
		clog.Debug().Str("key", key).Msg("cache STORE")
	}
	return result, nil
}

// AnalyzeFREDOutlook caches by today's date + language.
func (c *CachedInterpreter) AnalyzeFREDOutlook(ctx context.Context, data *fred.MacroData, lang string) (string, error) {
	if data == nil || c.cache == nil {
		return c.inner.AnalyzeFREDOutlook(ctx, data, lang)
	}

	version := time.Now().Format("20060102")
	if lang == "" {
		lang = "id"
	}
	key := fmt.Sprintf("aicache:fred:%s:%s", version, lang)

	if cached, ok := c.cache.Get(ctx, key); ok {
		clog.Debug().Str("key", key).Msg("cache HIT")
		return cached, nil
	}

	result, err := c.inner.AnalyzeFREDOutlook(ctx, data, lang)
	if err != nil {
		return result, err
	}

	if sErr := c.cache.Set(ctx, key, result, "fred", version); sErr != nil {
		clog.Warn().Err(sErr).Str("key", key).Msg("store failed")
	} else {
		clog.Debug().Str("key", key).Msg("cache STORE")
	}
	return result, nil
}

// AnalyzeActualRelease is NOT cached — each release is unique and time-sensitive.
func (c *CachedInterpreter) AnalyzeActualRelease(ctx context.Context, event domain.NewsEvent, lang string) (string, error) {
	return c.inner.AnalyzeActualRelease(ctx, event, lang)
}

// InvalidateOnCOTUpdate should be called when new COT data arrives.
// Invalidates: cot, weekly, cross, combined caches.
func (c *CachedInterpreter) InvalidateOnCOTUpdate(ctx context.Context) {
	if c.cache == nil {
		return
	}
	prefixes := []string{"aicache:cot:", "aicache:weekly:", "aicache:cross:", "aicache:combined:"}
	for _, p := range prefixes {
		if err := c.cache.InvalidateByPrefix(ctx, p); err != nil {
			clog.Warn().Err(err).Str("prefix", p).Msg("invalidation failed")
		}
	}
	clog.Info().Msg("invalidated caches on COT update")
}

// InvalidateOnNewsUpdate should be called when calendar events change significantly.
// Invalidates: news, combined caches.
func (c *CachedInterpreter) InvalidateOnNewsUpdate(ctx context.Context) {
	if c.cache == nil {
		return
	}
	prefixes := []string{"aicache:news:", "aicache:combined:"}
	for _, p := range prefixes {
		if err := c.cache.InvalidateByPrefix(ctx, p); err != nil {
			clog.Warn().Err(err).Str("prefix", p).Msg("invalidation failed")
		}
	}
	clog.Info().Msg("invalidated caches on news update")
}

// InvalidateOnFREDUpdate should be called when FRED macro data changes.
// Invalidates: fred, weekly, news, combined caches (all are FRED-aware via Gap E).
func (c *CachedInterpreter) InvalidateOnFREDUpdate(ctx context.Context) {
	if c.cache == nil {
		return
	}
	prefixes := []string{"aicache:fred:", "aicache:weekly:", "aicache:news:", "aicache:combined:"}
	for _, p := range prefixes {
		if err := c.cache.InvalidateByPrefix(ctx, p); err != nil {
			clog.Warn().Err(err).Str("prefix", p).Msg("invalidation failed")
		}
	}
	clog.Info().Msg("invalidated caches on FRED update")
}

// InvalidateAll clears all AI cache entries.
func (c *CachedInterpreter) InvalidateAll(ctx context.Context) {
	if c.cache == nil {
		return
	}
	if err := c.cache.InvalidateByPrefix(ctx, "aicache:"); err != nil {
		clog.Warn().Err(err).Msg("invalidate all failed")
	}
	clog.Info().Msg("invalidated ALL caches")
}

// --- helpers ---

// latestReportDate returns the latest report date from COT analyses as YYYYMMDD.
func latestReportDate(analyses []domain.COTAnalysis) string {
	var latest time.Time
	for _, a := range analyses {
		if a.ReportDate.After(latest) {
			latest = a.ReportDate
		}
	}
	if latest.IsZero() {
		return "unknown"
	}
	return latest.Format("20060102")
}

// latestReportDateFromMap returns the latest report date from a map of COT analyses.
func latestReportDateFromMap(cotData map[string]*domain.COTAnalysis) string {
	var latest time.Time
	for _, a := range cotData {
		if a != nil && a.ReportDate.After(latest) {
			latest = a.ReportDate
		}
	}
	if latest.IsZero() {
		return "unknown"
	}
	return latest.Format("20060102")
}

// currentWeekStart returns the Monday of the current week as YYYYMMDD.
func currentWeekStart() string {
	now := timeutil.NowWIB()
	// Go back to Monday
	for now.Weekday() != time.Monday {
		now = now.AddDate(0, 0, -1)
	}
	return now.Format("20060102")
}
