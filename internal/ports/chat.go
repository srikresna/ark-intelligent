package ports

import "context"

// ---------------------------------------------------------------------------
// ChatEngine — Claude AI chatbot interface
// ---------------------------------------------------------------------------

// ContentBlock represents a single content block in a multimodal message.
// Can be text, image (base64), or document reference.
type ContentBlock struct {
	Type      string `json:"type"`                 // "text", "image", "document"
	Text      string `json:"text,omitempty"`        // for type="text"
	MediaType string `json:"media_type,omitempty"`  // e.g. "image/jpeg", "application/pdf"
	Data      string `json:"data,omitempty"`        // base64-encoded data for images
	FileName  string `json:"file_name,omitempty"`   // original filename for documents
}

// ChatMessage represents a single message in a conversation.
// Content is used for simple text-only messages.
// ContentBlocks is used for multimodal messages (text + images).
// If ContentBlocks is non-empty, Content is ignored.
type ChatMessage struct {
	Role          string         `json:"role"`                     // "user" or "assistant"
	Content       string         `json:"content"`                  // simple text content
	ContentBlocks []ContentBlock `json:"content_blocks,omitempty"` // multimodal content
}

// IsMultimodal returns true if the message contains non-text content blocks.
func (m ChatMessage) IsMultimodal() bool {
	return len(m.ContentBlocks) > 0
}

// GetText returns the text content of the message, whether from Content or ContentBlocks.
func (m ChatMessage) GetText() string {
	if m.Content != "" {
		return m.Content
	}
	for _, b := range m.ContentBlocks {
		if b.Type == "text" && b.Text != "" {
			return b.Text
		}
	}
	return ""
}

// ServerTool represents an Anthropic server-side tool.
type ServerTool struct {
	Type    string `json:"type"`              // e.g. "web_search_20250305"
	Name    string `json:"name,omitempty"`    // e.g. "web_search"
	MaxUses int    `json:"max_uses,omitempty"` // 0 = unlimited
}

// ChatRequest bundles all inputs for a chat engine call.
type ChatRequest struct {
	UserID       int64
	Messages     []ChatMessage
	SystemPrompt string
	Tools        []ServerTool
	MaxTokens    int
}

// ChatResponse holds the output from a chat engine call.
type ChatResponse struct {
	Content      string
	ToolsUsed    []string // names of server tools that were invoked
	InputTokens  int
	OutputTokens int

	// Prompt caching metrics (Anthropic-specific).
	// CacheCreationTokens: tokens written to cache this request.
	// CacheReadTokens: tokens read from cache (saves cost).
	CacheCreationTokens int
	CacheReadTokens     int
}

// ChatEngine defines the interface for conversational AI.
// Primary implementation: Claude (via marketriskmonitor proxy).
// Separate from AIAnalyzer — ChatEngine handles freeform multi-turn conversation,
// while AIAnalyzer handles structured single-shot analysis with template fallback.
type ChatEngine interface {
	// Chat sends a conversation to the AI and returns a response.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// IsAvailable returns true if the chat engine is configured and reachable.
	IsAvailable(ctx context.Context) bool
}
