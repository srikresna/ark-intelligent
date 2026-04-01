package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

// ---------------------------------------------------------------------------
// Bounded LRU Mutex Map
// ---------------------------------------------------------------------------

const (
	lruMutexMaxSize = 10_000
	lruMutexTTL     = 30 * time.Minute
	lruEvictEvery   = 5 * time.Minute
)

// lruMutexEntry holds a per-user mutex and its last-access timestamp.
type lruMutexEntry struct {
	mu         sync.Mutex
	lastAccess time.Time
}

// lruMutexMap is a bounded map of per-user mutexes with TTL-based eviction.
// It replaces a bare sync.Map to prevent unbounded memory growth in long-running deployments.
type lruMutexMap struct {
	mu      sync.Mutex
	entries map[int64]*lruMutexEntry
	maxSize int
	ttl     time.Duration
}

func newLRUMutexMap(maxSize int, ttl time.Duration) *lruMutexMap {
	return &lruMutexMap{
		entries: make(map[int64]*lruMutexEntry, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// get returns the per-user mutex, creating one if it does not exist.
// The entry's lastAccess is refreshed on every call.
func (l *lruMutexMap) get(userID int64) *sync.Mutex {
	l.mu.Lock()
	entry, ok := l.entries[userID]
	if !ok {
		entry = &lruMutexEntry{}
		l.entries[userID] = entry
	}
	entry.lastAccess = time.Now()
	l.mu.Unlock()
	return &entry.mu
}

// evict removes entries that have not been accessed for longer than the TTL.
// An entry is only removed if its inner mutex is not currently held (TryLock succeeds),
// which avoids the TOCTOU race where two concurrent callers end up with different mutexes.
func (l *lruMutexMap) evict() {
	cutoff := time.Now().Add(-l.ttl)
	evicted := 0

	l.mu.Lock()
	for uid, entry := range l.entries {
		if entry.lastAccess.Before(cutoff) && entry.mu.TryLock() {
			entry.mu.Unlock()
			delete(l.entries, uid)
			evicted++
		}
	}
	remaining := len(l.entries)
	l.mu.Unlock()

	if evicted > 0 {
		log.Debug().Int("evicted", evicted).Int("remaining", remaining).Msg("lruMutexMap: evicted expired entries")
	}
}

func (l *lruMutexMap) size() int {
	l.mu.Lock()
	n := len(l.entries)
	l.mu.Unlock()
	return n
}

// ---------------------------------------------------------------------------
// Authorization Middleware
// ---------------------------------------------------------------------------

// Middleware enforces access control, tiered rate limiting, and daily quotas.
type Middleware struct {
	userRepo ports.UserRepository
	ownerID  int64

	// Per-user mutex to prevent TOCTOU race conditions on profile read-modify-write.
	// Bounded LRU map with TTL eviction prevents unbounded memory growth.
	userMu *lruMutexMap

	// Per-user sliding window for Member/Admin (per-minute rate limiting)
	windowMu sync.Mutex
	windows  map[int64]*userWindow
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewMiddleware creates a new authorization middleware and starts cleanup goroutine.
func NewMiddleware(userRepo ports.UserRepository, ownerID int64) *Middleware {
	m := &Middleware{
		userRepo: userRepo,
		ownerID:  ownerID,
		userMu:   newLRUMutexMap(lruMutexMaxSize, lruMutexTTL),
		windows:  make(map[int64]*userWindow),
		stopCh:   make(chan struct{}),
	}
	go m.cleanupLoop()
	return m
}

// Stop terminates the background cleanup goroutine. Safe to call multiple times.
func (m *Middleware) Stop() {
	m.stopOnce.Do(func() { close(m.stopCh) })
}

// getUserMutex returns a per-user mutex for atomic profile operations.
func (m *Middleware) getUserMutex(userID int64) *sync.Mutex {
	return m.userMu.get(userID)
}

// AuthResult is the outcome of an authorization check.
type AuthResult struct {
	Allowed bool
	Profile *domain.UserProfile
	Reason  string // human-readable denial reason (empty if allowed)
}

// Authorize checks if a user is allowed to execute a command.
// It handles: user upsert, ban check, daily counter reset, quota enforcement.
// username may be empty.
func (m *Middleware) Authorize(ctx context.Context, userID int64, username, command string) AuthResult {
	if userID == 0 {
		return AuthResult{Allowed: false, Reason: "Invalid user ID."} // reject zero ID
	}

	// Per-user lock prevents TOCTOU race conditions on counters.
	mu := m.getUserMutex(userID)
	mu.Lock()
	defer mu.Unlock()

	profile, err := m.userRepo.GetUser(ctx, userID)
	if err != nil {
		log.Error().Err(err).Int64("user_id", userID).Msg("middleware: get user failed")
		// Fail-closed for safety: deny on DB error (except owner who always passes)
		if m.ownerID != 0 && userID == m.ownerID {
			return AuthResult{Allowed: true}
		}
		return AuthResult{Allowed: false, Reason: "Service temporarily unavailable. Please try again."}
	}

	now := time.Now()

	// New user — create profile
	if profile == nil {
		role := domain.RoleFree
		if m.ownerID != 0 && userID == m.ownerID {
			role = domain.RoleOwner
		}
		profile = &domain.UserProfile{
			UserID:           userID,
			Username:         username,
			Role:             role,
			CreatedAt:        now,
			LastSeenAt:       now,
			CounterResetDate: todayWIB(),
		}
		if err := m.userRepo.UpsertUser(ctx, profile); err != nil {
			log.Error().Err(err).Int64("user_id", userID).Msg("middleware: create user failed")
			// Fail-closed: if we can't persist the profile, deny access to prevent
			// unlimited usage from counter reset loops (user would appear new each time).
			// Exception: Owner always passes.
			if m.ownerID != 0 && userID == m.ownerID {
				return AuthResult{Allowed: true, Profile: profile}
			}
			return AuthResult{Allowed: false, Reason: "Service temporarily unavailable. Please try again."}
		}
		return AuthResult{Allowed: true, Profile: profile}
	}

	// Ensure owner role is always up to date (must run BEFORE ban check)
	if m.ownerID != 0 && userID == m.ownerID && profile.Role != domain.RoleOwner {
		profile.Role = domain.RoleOwner
	}

	// Check banned (after owner re-assertion so owner can never be locked out)
	if profile.Role == domain.RoleBanned {
		return AuthResult{Allowed: false, Profile: profile, Reason: "Your account has been suspended. Contact admin for assistance."}
	}

	// Update username if changed
	if username != "" && profile.Username != username {
		profile.Username = username
	}

	// Update last seen
	profile.LastSeenAt = now

	// Reset daily counters if new day (WIB)
	today := todayWIB()
	if profile.CounterResetDate != today {
		profile.DailyCommandCount = 0
		profile.DailyAICount = 0
		profile.CounterResetDate = today
	}

	limits := domain.GetTierLimits(profile.Role)

	// --- Command rate limit check ---
	if limits.CommandLimit > 0 { // 0 = unlimited (Owner)
		if limits.CommandDaily {
			// Free tier: daily limit
			if profile.DailyCommandCount >= limits.CommandLimit {
				_ = m.userRepo.UpsertUser(ctx, profile)
				return AuthResult{
					Allowed: false,
					Profile: profile,
					Reason: fmt.Sprintf(
						"Daily command limit reached (%d/%d). Use /membership to see upgrade options.",
						profile.DailyCommandCount, limits.CommandLimit,
					),
				}
			}
		} else {
			// Member/Admin: per-minute sliding window
			if !m.allowSlidingWindow(userID, limits.CommandLimit) {
				_ = m.userRepo.UpsertUser(ctx, profile)
				return AuthResult{
					Allowed: false,
					Profile: profile,
					Reason:  "Rate limited — please wait a moment before sending more commands.",
				}
			}
		}
	}

	// Increment daily counter
	profile.DailyCommandCount++

	// Persist updated profile
	if err := m.userRepo.UpsertUser(ctx, profile); err != nil {
		log.Error().Err(err).Int64("user_id", userID).Msg("middleware: update user failed")
	}

	return AuthResult{Allowed: true, Profile: profile}
}

// AuthorizeCallback checks if a user is allowed to execute a callback action.
// Lighter than Authorize: checks ban status and applies sliding-window rate limit,
// but does NOT increment the daily command counter (callbacks are supplementary interactions).
// Uses per-user lock for consistency with Authorize/SetUserRole (prevents stale role reads).
func (m *Middleware) AuthorizeCallback(ctx context.Context, userID int64) AuthResult {
	if userID == 0 {
		return AuthResult{Allowed: false, Reason: "Invalid user ID."}
	}

	// Owner always passes (no lock needed — role is immutable)
	if m.ownerID != 0 && userID == m.ownerID {
		return AuthResult{Allowed: true}
	}

	// Per-user lock prevents reading a stale role during a concurrent SetUserRole.
	mu := m.getUserMutex(userID)
	mu.Lock()
	defer mu.Unlock()

	profile, err := m.userRepo.GetUser(ctx, userID)
	if err != nil {
		log.Error().Err(err).Int64("user_id", userID).Msg("middleware: callback get user failed")
		return AuthResult{Allowed: false, Reason: "Service temporarily unavailable."}
	}

	if profile == nil {
		// Unknown user clicking a callback — allow (they'll register on next /start)
		return AuthResult{Allowed: true}
	}

	// Owner re-assertion (persist if changed)
	if m.ownerID != 0 && userID == m.ownerID && profile.Role != domain.RoleOwner {
		profile.Role = domain.RoleOwner
		_ = m.userRepo.UpsertUser(ctx, profile)
	}

	// Ban check
	if profile.Role == domain.RoleBanned {
		return AuthResult{Allowed: false, Profile: profile, Reason: "Your account has been suspended."}
	}

	// Sliding window rate limit for callbacks (use same limits as commands)
	limits := domain.GetTierLimits(profile.Role)
	if limits.CommandLimit > 0 && !limits.CommandDaily {
		if !m.allowSlidingWindow(userID, limits.CommandLimit) {
			return AuthResult{Allowed: false, Profile: profile, Reason: "Rate limited — please wait a moment."}
		}
	}

	return AuthResult{Allowed: true, Profile: profile}
}

// CheckAIQuota checks if the user is allowed to make an AI call.
// Returns (allowed, reason). If allowed, increments the daily AI counter.
func (m *Middleware) CheckAIQuota(ctx context.Context, userID int64) (bool, string) {
	if userID == 0 {
		return false, "Invalid user ID."
	}

	// Per-user lock prevents race condition on AI counter.
	mu := m.getUserMutex(userID)
	mu.Lock()
	defer mu.Unlock()

	profile, err := m.userRepo.GetUser(ctx, userID)
	if err != nil {
		// Fail-closed for AI quota on DB error (costs money)
		if m.ownerID != 0 && userID == m.ownerID {
			return true, ""
		}
		return false, "Service temporarily unavailable. Please try again."
	}

	// Auto-create free-tier profile for new users who start with chat.
	// Without this, users who send a chat message before any slash command
	// would be blocked because AuthorizeCallback doesn't create profiles.
	if profile == nil {
		if m.ownerID != 0 && userID == m.ownerID {
			return true, ""
		}
		now := time.Now()
		role := domain.RoleFree
		profile = &domain.UserProfile{
			UserID:           userID,
			Role:             role,
			CreatedAt:        now,
			LastSeenAt:       now,
			CounterResetDate: todayWIB(),
		}
		if err := m.userRepo.UpsertUser(ctx, profile); err != nil {
			log.Error().Err(err).Int64("user_id", userID).Msg("middleware: auto-create user in CheckAIQuota failed")
			return false, "Service temporarily unavailable. Please try again."
		}
	}

	limits := domain.GetTierLimits(profile.Role)

	// Owner: unlimited
	if limits.AICallsPerDay == 0 {
		return true, ""
	}

	// Reset daily AI counter only (not command counter — that's managed by Authorize)
	today := todayWIB()
	if profile.CounterResetDate != today {
		profile.DailyAICount = 0
		// Also reset command count to keep consistency
		profile.DailyCommandCount = 0
		profile.CounterResetDate = today
	}

	if profile.DailyAICount >= limits.AICallsPerDay {
		return false, fmt.Sprintf(
			"Daily AI limit reached (%d/%d). Use /membership to see upgrade options.",
			profile.DailyAICount, limits.AICallsPerDay,
		)
	}

	profile.DailyAICount++
	_ = m.userRepo.UpsertUser(ctx, profile)
	return true, ""
}

// RefundAIQuota decrements the daily AI counter for a user.
// Used when AI quota was consumed but no real AI call succeeded (e.g., template fallback).
// Safe to call even if counter is already 0 (no underflow).
func (m *Middleware) RefundAIQuota(ctx context.Context, userID int64) {
	if userID == 0 {
		return
	}
	mu := m.getUserMutex(userID)
	mu.Lock()
	defer mu.Unlock()

	profile, err := m.userRepo.GetUser(ctx, userID)
	if err != nil || profile == nil {
		return
	}
	if profile.DailyAICount > 0 {
		profile.DailyAICount--
		_ = m.userRepo.UpsertUser(ctx, profile)
	}
}

// GetAICooldown returns the AI cooldown duration for the user's tier.
func (m *Middleware) GetAICooldown(ctx context.Context, userID int64) time.Duration {
	if userID == 0 || (m.ownerID != 0 && userID == m.ownerID) {
		return 0
	}

	profile, err := m.userRepo.GetUser(ctx, userID)
	if err != nil || profile == nil {
		return 30 * time.Second // default
	}

	limits := domain.GetTierLimits(profile.Role)
	return time.Duration(limits.AICooldownSec) * time.Second
}

// GetUserRole returns the role of a user. Returns RoleFree if not found.
func (m *Middleware) GetUserRole(ctx context.Context, userID int64) domain.UserRole {
	if m.ownerID != 0 && userID == m.ownerID {
		return domain.RoleOwner
	}
	profile, err := m.userRepo.GetUser(ctx, userID)
	if err != nil || profile == nil {
		return domain.RoleFree
	}
	return profile.Role
}

// IsUserBanned returns true if the user is banned. Used by broadcast loops.
func (m *Middleware) IsUserBanned(ctx context.Context, userID int64) bool {
	role := m.GetUserRole(ctx, userID)
	return role == domain.RoleBanned
}

// EffectiveAlertFilters returns the currency filter and impact filter to use
// for alert delivery, applying tier overrides for Free users.
// Banned users get impossible filters to suppress alerts.
// Returns (currencies, impacts).
func (m *Middleware) EffectiveAlertFilters(ctx context.Context, userID int64, prefsCurrencies []string, prefsImpacts []string) ([]string, []string) {
	role := m.GetUserRole(ctx, userID)

	if role == domain.RoleBanned {
		// Return impossible filter to suppress alerts
		return []string{"__BANNED__"}, []string{"__BANNED__"}
	}

	if role == domain.RoleFree {
		return domain.FreeAlertCurrencies(), domain.FreeAlertImpacts()
	}

	// Member+ uses their own prefs
	return prefsCurrencies, prefsImpacts
}

// ShouldReceiveFREDAlerts returns whether this user should receive FRED macro alerts.
// Free-tier and banned users do not receive FRED alerts.
func (m *Middleware) ShouldReceiveFREDAlerts(ctx context.Context, userID int64) bool {
	role := m.GetUserRole(ctx, userID)
	return role != domain.RoleFree && role != domain.RoleBanned
}

// ---------------------------------------------------------------------------
// Sliding window helper (for Member/Admin per-minute limits)
// ---------------------------------------------------------------------------

func (m *Middleware) allowSlidingWindow(userID int64, maxPerMinute int) bool {
	m.windowMu.Lock()
	defer m.windowMu.Unlock()

	now := time.Now()
	w, ok := m.windows[userID]
	if !ok {
		w = &userWindow{}
		m.windows[userID] = w
	}

	cutoff := now.Add(-60 * time.Second)
	start := 0
	for start < len(w.timestamps) && w.timestamps[start].Before(cutoff) {
		start++
	}
	w.timestamps = w.timestamps[start:]

	if len(w.timestamps) >= maxPerMinute {
		return false
	}
	w.timestamps = append(w.timestamps, now)
	return true
}

// cleanupLoop periodically removes stale entries from the sliding window map
// and per-user mutex map to prevent unbounded memory growth.
func (m *Middleware) cleanupLoop() {
	windowTicker := time.NewTicker(2 * time.Minute)
	mutexTicker := time.NewTicker(lruEvictEvery)
	defer windowTicker.Stop()
	defer mutexTicker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-windowTicker.C:
			m.cleanupWindows()
		case <-mutexTicker.C:
			m.userMu.evict()
		}
	}
}

// cleanupWindows removes window entries for users idle > 5 minutes.
func (m *Middleware) cleanupWindows() {
	m.windowMu.Lock()
	defer m.windowMu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute)
	for uid, w := range m.windows {
		if len(w.timestamps) == 0 || w.timestamps[len(w.timestamps)-1].Before(cutoff) {
			delete(m.windows, uid)
		}
	}
}

// ---------------------------------------------------------------------------
// Admin operations
// ---------------------------------------------------------------------------

// SetUserRole updates a user's role. Only Owner/Admin should call this.
// Uses per-user mutex to prevent TOCTOU race with concurrent Authorize/CheckAIQuota calls.
func (m *Middleware) SetUserRole(ctx context.Context, targetUserID int64, newRole domain.UserRole) error {
	mu := m.getUserMutex(targetUserID)
	mu.Lock()
	defer mu.Unlock()
	return m.userRepo.SetRole(ctx, targetUserID, newRole)
}

// GetAllUsers returns all registered user profiles.
func (m *Middleware) GetAllUsers(ctx context.Context) ([]*domain.UserProfile, error) {
	return m.userRepo.GetAllUsers(ctx)
}

// FormatUserList formats users for display in a Telegram message.
func FormatUserList(users []*domain.UserProfile) string {
	if len(users) == 0 {
		return "No registered users."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>Registered Users</b> (%d)\n\n", len(users)))

	roleIcons := map[domain.UserRole]string{
		domain.RoleOwner:  "\xf0\x9f\x91\x91",
		domain.RoleAdmin:  "\xf0\x9f\x9b\xa1",
		domain.RoleMember: "\xe2\xad\x90",
		domain.RoleFree:   "\xf0\x9f\x91\xa4",
		domain.RoleBanned: "\xf0\x9f\x9a\xab",
	}

	for _, u := range users {
		icon := roleIcons[u.Role]
		if icon == "" {
			icon = "\xe2\x9d\x93"
		}
		name := html.EscapeString(u.Username)
		if name == "" {
			name = fmt.Sprintf("ID:%d", u.UserID)
		}

		b.WriteString(fmt.Sprintf("%s <code>%s</code> [%s]\n", icon, name, u.Role))
		b.WriteString(fmt.Sprintf("   Cmd: %d | AI: %d | Last: %s\n",
			u.DailyCommandCount, u.DailyAICount, u.LastSeenAt.Format("Jan 02 15:04")))
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func todayWIB() string {
	return timeutil.NowWIB().Format("2006-01-02")
}

// GetUserProfile retrieves the user profile for the given user ID.
// Returns nil if the user doesn't exist or on error.
func (m *Middleware) GetUserProfile(ctx context.Context, userID int64) *domain.UserProfile {
	if userID == 0 {
		return nil
	}

	// Owner always gets RoleOwner
	if m.ownerID != 0 && userID == m.ownerID {
		profile, err := m.userRepo.GetUser(ctx, userID)
		if err != nil || profile == nil {
			return &domain.UserProfile{UserID: userID, Role: domain.RoleOwner}
		}
		return profile
	}

	profile, err := m.userRepo.GetUser(ctx, userID)
	if err != nil || profile == nil {
		return nil
	}
	return profile
}
