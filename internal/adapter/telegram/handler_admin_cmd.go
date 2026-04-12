package telegram

// /membership, Admin Commands

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// /membership — Tier comparison & upgrade info
// ---------------------------------------------------------------------------

// cmdMembership shows the tier comparison and how to upgrade.
func (h *Handler) cmdMembership(ctx context.Context, chatID string, userID int64, args string) error {
	// Determine caller's current tier
	currentRole := domain.RoleFree
	if h.middleware != nil {
		currentRole = h.middleware.GetUserRole(ctx, userID)
	} else if h.bot.isOwner(userID) {
		currentRole = domain.RoleOwner
	}

	currentLabel := strings.ToUpper(string(currentRole))

	html := fmt.Sprintf(""+
		"\xf0\x9f\xa6\x85 <b>ARK Intelligence Membership</b>\n"+
		"Your tier: <b>%s</b>\n\n", currentLabel)

	html += "" +
		"<b>\xf0\x9f\x86\x93 FREE</b>\n" +
		"<code>Commands   : 10/day</code>\n" +
		"<code>AI Analysis: 3/day (30s cooldown)</code>\n" +
		"<code>News Alert : USD only, High impact</code>\n" +
		"<code>FRED Macro : </code>\xe2\x9d\x8c\n" +
		"<code>COT Data   : </code>\xe2\x9c\x85 Full access\n" +
		"<code>Calendar   : </code>\xe2\x9c\x85 Full access\n\n"

	html += "" +
		"<b>\xe2\xad\x90 MEMBER</b>\n" +
		"<code>Commands   : 15/min (no daily cap)</code>\n" +
		"<code>AI Analysis: 10/day (30s cooldown)</code>\n" +
		"<code>News Alert : All currencies &amp; impacts</code>\n" +
		"<code>FRED Macro : </code>\xe2\x9c\x85 Regime alerts\n" +
		"<code>COT Data   : </code>\xe2\x9c\x85 Full access\n" +
		"<code>Calendar   : </code>\xe2\x9c\x85 Full access\n\n"

	// Only show ADMIN tier to admins and owners
	isAdmin := domain.RoleHierarchy(currentRole) >= domain.RoleHierarchy(domain.RoleAdmin)
	if isAdmin {
		html += "" +
			"<b>\xf0\x9f\x9b\xa1 ADMIN</b>\n" +
			"<code>Commands   : 30/min (no daily cap)</code>\n" +
			"<code>AI Analysis: 50/day (10s cooldown)</code>\n" +
			"<code>News Alert : All currencies &amp; impacts</code>\n" +
			"<code>FRED Macro : </code>\xe2\x9c\x85 Regime alerts\n" +
			"<code>User Mgmt  : </code>\xe2\x9c\x85 /users, /ban, /setrole\n\n"
	}

	// Show upgrade CTA for non-owner users
	if currentRole == domain.RoleFree || currentRole == domain.RoleMember {
		ownerID := h.bot.OwnerID()
		if ownerID > 0 {
			html += fmt.Sprintf(
				"\xf0\x9f\x94\x91 <b>Upgrade to Member</b>\n"+
					"Contact the owner to upgrade your access:\n"+
					"\xe2\x9e\xa1 <a href=\"tg://user?id=%d\">Contact Owner</a>\n\n"+
					"<i>Include your User ID: <code>%d</code></i>",
				ownerID, userID)
		} else {
			html += fmt.Sprintf(
				"\xf0\x9f\x94\x91 <b>Upgrade to Member</b>\n"+
					"Contact the group admin to upgrade your access.\n\n"+
					"<i>Your User ID: <code>%d</code></i>",
				userID)
		}
	} else if currentRole == domain.RoleOwner {
		html += "<i>You have unlimited access as Owner.</i>"
	}

	_, err := h.bot.SendHTML(ctx, chatID, html)
	return err
}

// ---------------------------------------------------------------------------
// Admin Commands — /users, /setrole, /ban, /unban
// ---------------------------------------------------------------------------

// requireAdmin checks that the caller is Owner or Admin. Returns false and sends an error if not.
func (h *Handler) requireAdmin(ctx context.Context, chatID string, userID int64) bool {
	if h.middleware == nil {
		return h.bot.isOwner(userID) // fallback
	}
	role := h.middleware.GetUserRole(ctx, userID)
	if domain.RoleHierarchy(role) >= domain.RoleHierarchy(domain.RoleAdmin) {
		return true
	}
	_, _ = h.bot.SendHTML(ctx, chatID, "This command requires Admin privileges.")
	return false
}

// cmdUsers lists all registered users with their roles and usage stats.
func (h *Handler) cmdUsers(ctx context.Context, chatID string, userID int64, args string) error {
	if !h.requireAdmin(ctx, chatID, userID) {
		return nil
	}
	if h.middleware == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "User management not available.")
		return err
	}

	users, err := h.middleware.GetAllUsers(ctx)
	if err != nil {
		log.Error().Err(err).Int64("caller", userID).Msg("cmdUsers: failed to list users")
		_, err = h.bot.SendHTML(ctx, chatID, "Failed to list users. Check server logs.")
		return err
	}

	html := FormatUserList(users)
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

// cmdSetRole changes a user's role (with confirmation step).
// Usage: /setrole <userID> <role>
func (h *Handler) cmdSetRole(ctx context.Context, chatID string, userID int64, args string) error {
	if !h.requireAdmin(ctx, chatID, userID) {
		return nil
	}
	if h.middleware == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "User management not available.")
		return err
	}

	parts := strings.Fields(args)
	if len(parts) < 2 {
		_, err := h.bot.SendHTML(ctx, chatID,
			"Usage: <code>/setrole &lt;userID&gt; &lt;role&gt;</code>\nRoles: owner, admin, member, free, banned")
		return err
	}

	var targetID int64
	if _, err := fmt.Sscanf(parts[0], "%d", &targetID); err != nil {
		_, err = h.bot.SendHTML(ctx, chatID, "Invalid user ID. Must be a number.")
		return err
	}

	newRole := domain.UserRole(strings.ToLower(parts[1]))
	switch newRole {
	case domain.RoleOwner, domain.RoleAdmin, domain.RoleMember, domain.RoleFree, domain.RoleBanned:
		// valid
	default:
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("Unknown role <code>%s</code>. Valid: owner, admin, member, free, banned", html.EscapeString(parts[1])))
		return err
	}

	// Prevent non-owner from setting privileged roles (owner or admin)
	callerRole := h.middleware.GetUserRole(ctx, userID)
	if (newRole == domain.RoleOwner || newRole == domain.RoleAdmin) && callerRole != domain.RoleOwner {
		_, err := h.bot.SendHTML(ctx, chatID, "Only the Owner can assign Owner or Admin roles.")
		return err
	}

	// Prevent banning/demoting the Owner
	if h.bot.isOwner(targetID) && newRole != domain.RoleOwner {
		_, err := h.bot.SendHTML(ctx, chatID, "Cannot change the Owner's role.")
		return err
	}

	// Prevent Admin from modifying users with equal or higher privilege
	targetRole := h.middleware.GetUserRole(ctx, targetID)
	if callerRole != domain.RoleOwner && domain.RoleHierarchy(targetRole) >= domain.RoleHierarchy(callerRole) {
		_, err := h.bot.SendHTML(ctx, chatID, "You cannot modify a user with equal or higher privileges.")
		return err
	}

	// Show confirmation instead of executing immediately.
	key := h.adminConfirm.add(adminAction{
		Action:   "setrole",
		CallerID: userID,
		TargetID: targetID,
		NewRole:  newRole,
	})

	preview := fmt.Sprintf(
		"\xf0\x9f\x9b\xa1 <b>Confirm Set Role</b>\n\n"+
			"Target: <code>%d</code>\n"+
			"Current role: <b>%s</b>\n"+
			"New role: <b>%s</b>\n\n"+
			"<i>Auto-cancels in 60s.</i>",
		targetID, targetRole, newRole)
	_, err := h.bot.SendWithKeyboard(ctx, chatID, preview, adminConfirmKeyboard(key))
	return err
}

// cmdBan bans a user (with confirmation step).
// Usage: /ban <userID>
func (h *Handler) cmdBan(ctx context.Context, chatID string, userID int64, args string) error {
	if !h.requireAdmin(ctx, chatID, userID) {
		return nil
	}
	if h.middleware == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "User management not available.")
		return err
	}

	targetStr := strings.TrimSpace(args)
	if targetStr == "" {
		_, err := h.bot.SendHTML(ctx, chatID, "Usage: <code>/ban &lt;userID&gt;</code>")
		return err
	}

	var targetID int64
	if _, err := fmt.Sscanf(targetStr, "%d", &targetID); err != nil {
		_, err = h.bot.SendHTML(ctx, chatID, "Invalid user ID.")
		return err
	}

	// Prevent banning the Owner
	if h.bot.isOwner(targetID) {
		_, err := h.bot.SendHTML(ctx, chatID, "Cannot ban the Owner.")
		return err
	}

	// Prevent Admin from banning users with equal or higher privilege
	callerRole := h.middleware.GetUserRole(ctx, userID)
	targetRole := h.middleware.GetUserRole(ctx, targetID)
	if callerRole != domain.RoleOwner && domain.RoleHierarchy(targetRole) >= domain.RoleHierarchy(callerRole) {
		_, err := h.bot.SendHTML(ctx, chatID, "You cannot ban a user with equal or higher privileges.")
		return err
	}

	// Show confirmation instead of executing immediately.
	key := h.adminConfirm.add(adminAction{
		Action:   "ban",
		CallerID: userID,
		TargetID: targetID,
	})

	preview := fmt.Sprintf(
		"\xf0\x9f\x9a\xab <b>Confirm Ban</b>\n\n"+
			"Target: <code>%d</code>\n"+
			"Current role: <b>%s</b>\n\n"+
			"\xe2\x9a\xa0\xef\xb8\x8f This will immediately revoke all access.\n"+
			"<i>Auto-cancels in 60s.</i>",
		targetID, targetRole)
	_, err := h.bot.SendWithKeyboard(ctx, chatID, preview, adminConfirmKeyboard(key))
	return err
}

// cmdUnban unbans a user (with role selection).
// Usage: /unban <userID>
func (h *Handler) cmdUnban(ctx context.Context, chatID string, userID int64, args string) error {
	if !h.requireAdmin(ctx, chatID, userID) {
		return nil
	}
	if h.middleware == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "User management not available.")
		return err
	}

	targetStr := strings.TrimSpace(args)
	if targetStr == "" {
		_, err := h.bot.SendHTML(ctx, chatID, "Usage: <code>/unban &lt;userID&gt;</code>")
		return err
	}

	var targetID int64
	if _, err := fmt.Sscanf(targetStr, "%d", &targetID); err != nil {
		_, err = h.bot.SendHTML(ctx, chatID, "Invalid user ID.")
		return err
	}

	// Show role selection instead of defaulting to Free.
	key := h.adminConfirm.add(adminAction{
		Action:   "unban",
		CallerID: userID,
		TargetID: targetID,
	})

	preview := fmt.Sprintf(
		"\xf0\x9f\x94\x93 <b>Unban User</b>\n\n"+
			"Target: <code>%d</code>\n\n"+
			"Restore as which role?\n"+
			"<i>Auto-cancels in 60s.</i>",
		targetID)
	_, err := h.bot.SendWithKeyboard(ctx, chatID, preview, unbanRoleKeyboard(key))
	return err
}

// notifyOwnerDebug sends a debug message to the bot owner (non-blocking, best-effort).
// Uses context.Background() so the notification survives even if the request context
// is cancelled before the goroutine fires (e.g. Telegram timeout, user disconnect).
// Does nothing if OwnerID is not set.
func (h *Handler) notifyOwnerDebug(_ context.Context, html string) {
	ownerID := h.bot.OwnerID()
	if ownerID <= 0 {
		return
	}
	go func() {
		_, _ = h.bot.SendHTML(context.Background(), fmt.Sprintf("%d", ownerID), html)
	}()
}
