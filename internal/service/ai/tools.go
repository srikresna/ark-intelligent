package ai

import (
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ToolConfig manages tier-based server tool access.
type ToolConfig struct {
	tierTools map[domain.UserRole][]ports.ServerTool
}

// NewToolConfig creates the default tool configuration.
// Server-managed tools (web_search, web_fetch, code_execution) are executed
// automatically by Anthropic's servers. The memory tool (memory_20250818)
// requires client-side round-trips handled by our ToolExecutor.
func NewToolConfig() *ToolConfig {
	return &ToolConfig{
		tierTools: map[domain.UserRole][]ports.ServerTool{
			domain.RoleFree: {
				// Memory only for free tier (enables personalization)
				{Type: "memory_20250818", Name: "memory"},
			},
			domain.RoleMember: {
				{Type: "web_search_20250305", Name: "web_search", MaxUses: 3},
				{Type: "memory_20250818", Name: "memory"},
			},
			domain.RoleAdmin: {
				{Type: "web_search_20250305", Name: "web_search", MaxUses: 5},
				{Type: "web_fetch_20260309", Name: "web_fetch", MaxUses: 3},
				{Type: "memory_20250818", Name: "memory"},
			},
			domain.RoleOwner: {
				{Type: "web_search_20250305", Name: "web_search", MaxUses: 5},
				{Type: "web_fetch_20260309", Name: "web_fetch", MaxUses: 5},
				{Type: "code_execution_20260120", Name: "code_execution"},
				{Type: "memory_20250818", Name: "memory"},
			},
			domain.RoleBanned: {}, // no tools
		},
	}
}

// ToolsForRole returns the allowed server tools for the given user role.
func (tc *ToolConfig) ToolsForRole(role domain.UserRole) []ports.ServerTool {
	if tools, ok := tc.tierTools[role]; ok {
		return tools
	}
	// Default to Free tier
	return tc.tierTools[domain.RoleFree]
}
