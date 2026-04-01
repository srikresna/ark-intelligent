package telegram

// Admin confirmation flow for destructive actions (/ban, /unban, /setrole).
// Shows a preview with inline keyboard before executing the action.
// Pending confirmations auto-expire after 60 seconds.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Pending admin confirmations
// ---------------------------------------------------------------------------

// adminAction describes a pending admin action awaiting confirmation.
type adminAction struct {
	Action   string          // "ban", "unban", "setrole"
	CallerID int64           // admin who initiated
	TargetID int64           // target user
	NewRole  domain.UserRole // only used for setrole/unban
	Expires  time.Time
}

// adminConfirmStore is a thread-safe map of pending admin confirmations.
// Key format: "admin:<callerID>:<nonce>".
type adminConfirmStore struct {
	mu      sync.Mutex
	pending map[string]adminAction
	nonce   int64
}

func newAdminConfirmStore() *adminConfirmStore {
	return &adminConfirmStore{
		pending: make(map[string]adminAction),
	}
}

// add stores a pending action and returns its key.
func (s *adminConfirmStore) add(a adminAction) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonce++
	key := fmt.Sprintf("%d_%d", a.CallerID, s.nonce)
	a.Expires = time.Now().Add(60 * time.Second)
	s.pending[key] = a

	// Opportunistic cleanup of expired entries.
	if len(s.pending) > 20 {
		now := time.Now()
		for k, v := range s.pending {
			if now.After(v.Expires) {
				delete(s.pending, k)
			}
		}
	}
	return key
}

// take removes and returns the action for the given key.
// Returns false if not found or expired.
func (s *adminConfirmStore) take(key string) (adminAction, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.pending[key]
	if !ok {
		return adminAction{}, false
	}
	delete(s.pending, key)
	if time.Now().After(a.Expires) {
		return adminAction{}, false
	}
	return a, true
}

// remove deletes a pending action (used on cancel).
func (s *adminConfirmStore) remove(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, key)
}

// ---------------------------------------------------------------------------
// Callback handler — registered as "adm_cf:"
// ---------------------------------------------------------------------------

// cbAdminConfirm handles "adm_cf:yes:<key>" and "adm_cf:no:<key>" and
// "adm_cf:role:<role>:<key>" callbacks.
func (h *Handler) cbAdminConfirm(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// data examples:
	//   "adm_cf:yes:<key>"
	//   "adm_cf:no:<key>"
	//   "adm_cf:role:free:<key>"
	//   "adm_cf:role:member:<key>"

	parts := splitCallbackData(data, "adm_cf:")
	if len(parts) < 2 {
		return nil
	}

	switch parts[0] {
	case "yes":
		return h.executeAdminAction(ctx, chatID, msgID, userID, parts[1])
	case "no":
		return h.cancelAdminAction(ctx, chatID, msgID, userID, parts[1])
	case "role":
		// "role:free:<key>" or "role:member:<key>"
		if len(parts) < 3 {
			return nil
		}
		return h.executeUnbanWithRole(ctx, chatID, msgID, userID, parts[1], parts[2])
	default:
		return nil
	}
}

// splitCallbackData removes the prefix and splits by ":".
func splitCallbackData(data, prefix string) []string {
	trimmed := data[len(prefix):]
	if trimmed == "" {
		return nil
	}
	result := make([]string, 0, 4)
	for trimmed != "" {
		idx := indexOf(trimmed, ':')
		if idx < 0 {
			result = append(result, trimmed)
			break
		}
		result = append(result, trimmed[:idx])
		trimmed = trimmed[idx+1:]
	}
	return result
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// executeAdminAction runs the confirmed ban or setrole action.
func (h *Handler) executeAdminAction(ctx context.Context, chatID string, msgID int, userID int64, key string) error {
	a, ok := h.adminConfirm.take(key)
	if !ok {
		_ = h.bot.EditMessage(ctx, chatID, msgID, "<i>⏳ Confirmation expired or already processed.</i>")
		return nil
	}
	// Verify the confirming user is the one who initiated the action.
	if a.CallerID != userID {
		// Put it back for the original caller.
		h.adminConfirm.mu.Lock()
		h.adminConfirm.pending[key] = a
		h.adminConfirm.mu.Unlock()
		return nil
	}

	switch a.Action {
	case "ban":
		return h.executeBan(ctx, chatID, msgID, userID, a.TargetID)
	case "setrole":
		return h.executeSetRole(ctx, chatID, msgID, userID, a.TargetID, a.NewRole)
	default:
		return nil
	}
}

// cancelAdminAction cancels a pending admin action.
func (h *Handler) cancelAdminAction(ctx context.Context, chatID string, msgID int, userID int64, key string) error {
	a, ok := h.adminConfirm.take(key)
	if !ok {
		_ = h.bot.EditMessage(ctx, chatID, msgID, "<i>⏳ Already cancelled or expired.</i>")
		return nil
	}
	if a.CallerID != userID {
		h.adminConfirm.mu.Lock()
		h.adminConfirm.pending[key] = a
		h.adminConfirm.mu.Unlock()
		return nil
	}
	log.Info().
		Int64("admin", userID).
		Str("action", a.Action).
		Int64("target", a.TargetID).
		Msg("admin action cancelled")
	_ = h.bot.EditMessage(ctx, chatID, msgID, "<i>❌ Action cancelled.</i>")
	return nil
}

// executeUnbanWithRole handles the unban flow where admin chooses restore role.
func (h *Handler) executeUnbanWithRole(ctx context.Context, chatID string, msgID int, userID int64, roleStr, key string) error {
	a, ok := h.adminConfirm.take(key)
	if !ok {
		_ = h.bot.EditMessage(ctx, chatID, msgID, "<i>⏳ Confirmation expired or already processed.</i>")
		return nil
	}
	if a.CallerID != userID || a.Action != "unban" {
		h.adminConfirm.mu.Lock()
		h.adminConfirm.pending[key] = a
		h.adminConfirm.mu.Unlock()
		return nil
	}

	newRole := domain.UserRole(roleStr)
	switch newRole {
	case domain.RoleFree, domain.RoleMember:
		// valid
	default:
		newRole = domain.RoleFree
	}

	return h.executeSetRole(ctx, chatID, msgID, userID, a.TargetID, newRole)
}

// ---------------------------------------------------------------------------
// Execution helpers
// ---------------------------------------------------------------------------

func (h *Handler) executeBan(ctx context.Context, chatID string, msgID int, adminID, targetID int64) error {
	if err := h.middleware.SetUserRole(ctx, targetID, domain.RoleBanned); err != nil {
		log.Error().Err(err).Int64("admin", adminID).Int64("target", targetID).Msg("admin ban failed")
		_ = h.bot.EditMessage(ctx, chatID, msgID, "❌ Failed to ban user. Check server logs.")
		return err
	}
	log.Info().
		Int64("admin", adminID).
		Int64("target", targetID).
		Str("action", "ban").
		Msg("admin action executed")
	_ = h.bot.EditMessage(ctx, chatID, msgID,
		fmt.Sprintf("✅ User <code>%d</code> has been <b>banned</b>.", targetID))
	return nil
}

func (h *Handler) executeSetRole(ctx context.Context, chatID string, msgID int, adminID, targetID int64, newRole domain.UserRole) error {
	if err := h.middleware.SetUserRole(ctx, targetID, newRole); err != nil {
		log.Error().Err(err).Int64("admin", adminID).Int64("target", targetID).Str("role", string(newRole)).Msg("admin setrole failed")
		_ = h.bot.EditMessage(ctx, chatID, msgID, "❌ Failed to set role. Check server logs.")
		return err
	}
	log.Info().
		Int64("admin", adminID).
		Int64("target", targetID).
		Str("action", "setrole").
		Str("role", string(newRole)).
		Msg("admin action executed")
	_ = h.bot.EditMessage(ctx, chatID, msgID,
		fmt.Sprintf("✅ User <code>%d</code> role set to <b>%s</b>.", targetID, newRole))
	return nil
}

// ---------------------------------------------------------------------------
// Confirmation keyboard builders
// ---------------------------------------------------------------------------

func adminConfirmKeyboard(key string) ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "✅ Ya, Lanjutkan", CallbackData: "adm_cf:yes:" + key},
				{Text: "❌ Batal", CallbackData: "adm_cf:no:" + key},
			},
		},
	}
}

func unbanRoleKeyboard(key string) ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "🆓 Free", CallbackData: "adm_cf:role:free:" + key},
				{Text: "⭐ Member", CallbackData: "adm_cf:role:member:" + key},
			},
			{
				{Text: "❌ Batal", CallbackData: "adm_cf:no:" + key},
			},
		},
	}
}
