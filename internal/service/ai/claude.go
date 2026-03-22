package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var claudeLog = logger.Component("claude")

// ClaudeClient implements ports.ChatEngine using the Anthropic Messages API
// via a proxy endpoint (marketriskmonitor.com/api/analyze).
type ClaudeClient struct {
	endpoint   string
	httpClient *http.Client
	model      string
	maxTokens  int
}

// NewClaudeClient creates a Claude client targeting the given endpoint.
func NewClaudeClient(endpoint string, timeout time.Duration, maxTokens int) *ClaudeClient {
	return &ClaudeClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		model:     "claude-opus-4-6",
		maxTokens: maxTokens,
	}
}

// SetModel overrides the default model name.
func (c *ClaudeClient) SetModel(model string) {
	c.model = model
}

// ---------------------------------------------------------------------------
// Anthropic Messages API types
// ---------------------------------------------------------------------------

type claudeRequest struct {
	Model     string               `json:"model"`
	MaxTokens int                  `json:"max_tokens"`
	System    string               `json:"system,omitempty"`
	Messages  []claudeMessage      `json:"messages"`
	Tools     []claudeToolDef      `json:"tools,omitempty"`
}

type claudeMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string for text-only, []claudeContentInput for multimodal
}

// claudeContentInput represents a single content block in a multimodal user message.
type claudeContentInput struct {
	Type   string              `json:"type"`             // "text", "image", "document"
	Text   string              `json:"text,omitempty"`   // for type="text"
	Title  string              `json:"title,omitempty"`  // for type="document" (optional, filename)
	Source *claudeImageSource  `json:"source,omitempty"` // for type="image" or "document"
}

// claudeImageSource represents the source of an image in a Claude message.
type claudeImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // e.g. "image/jpeg"
	Data      string `json:"data"`       // base64-encoded
}

type claudeToolDef struct {
	Type    string `json:"type"`
	Name    string `json:"name,omitempty"`
	MaxUses int    `json:"max_uses,omitempty"`
}

type claudeResponse struct {
	ID           string              `json:"id"`
	Type         string              `json:"type"`
	Model        string              `json:"model"`
	Role         string              `json:"role"`
	Content      []claudeContentBlock `json:"content"`
	StopReason   string              `json:"stop_reason"`
	Usage        *claudeUsage        `json:"usage,omitempty"`
	Error        *claudeError        `json:"error,omitempty"`
}

type claudeContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"` // for type="thinking" (extended thinking)
	Name     string `json:"name,omitempty"`     // for tool_use blocks
	ID       string `json:"id,omitempty"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type claudeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// ChatEngine implementation
// ---------------------------------------------------------------------------

// Chat sends a conversation to Claude and returns the response.
// Implements ports.ChatEngine.
func (c *ClaudeClient) Chat(ctx context.Context, req ports.ChatRequest) (*ports.ChatResponse, error) {
	// Build messages array
	messages := make([]claudeMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = buildClaudeMessage(m)
	}

	// Build tools array
	var tools []claudeToolDef
	for _, t := range req.Tools {
		name := t.Name
		if name == "" {
			// Derive name from type: "web_search_20250305" -> "web_search"
			name = deriveToolName(t.Type)
		}
		tools = append(tools, claudeToolDef{
			Type:    t.Type,
			Name:    name,
			MaxUses: t.MaxUses,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = c.maxTokens
	}

	apiReq := claudeRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages:  messages,
		Tools:     tools,
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			claudeLog.Warn().Int("attempt", attempt).Dur("backoff", backoff).Msg("retrying Claude request")
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := c.doRequest(ctx, &apiReq)
		if err != nil {
			lastErr = fmt.Errorf("claude request (attempt %d): %w", attempt+1, err)
			if isClaudeTransient(err) {
				continue
			}
			return nil, lastErr
		}

		// Check for API-level error
		if resp.Error != nil {
			lastErr = fmt.Errorf("claude API error: %s: %s", resp.Error.Type, resp.Error.Message)
			if resp.Error.Type == "overloaded_error" || strings.Contains(resp.Error.Message, "rate") {
				continue
			}
			return nil, lastErr
		}

		// Extract text content and tool usage
		text, toolsUsed := extractClaudeContent(resp)
		if text == "" {
			// Log response metadata for diagnostics (content blocks logged by extractClaudeContent)
			claudeLog.Warn().
				Str("stop_reason", resp.StopReason).
				Str("model", resp.Model).
				Int("content_blocks", len(resp.Content)).
				Msg("empty text from Claude response")
			lastErr = fmt.Errorf("empty Claude response (attempt %d, stop_reason=%s, blocks=%d)", attempt+1, resp.StopReason, len(resp.Content))
			continue
		}

		result := &ports.ChatResponse{
			Content:   text,
			ToolsUsed: toolsUsed,
		}
		if resp.Usage != nil {
			result.InputTokens = resp.Usage.InputTokens
			result.OutputTokens = resp.Usage.OutputTokens
		}

		return result, nil
	}

	return nil, fmt.Errorf("claude: all retries exhausted: %w", lastErr)
}

// IsAvailable returns true if the client is configured.
func (c *ClaudeClient) IsAvailable(_ context.Context) bool {
	return c.endpoint != ""
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func (c *ClaudeClient) doRequest(ctx context.Context, apiReq *claudeRequest) (*claudeResponse, error) {
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024)) // 2MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("claude server error %d: %s", resp.StatusCode, string(respBody[:min(200, len(respBody))]))
	}
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("claude rate limited (429): %s", string(respBody[:min(200, len(respBody))]))
	}
	if resp.StatusCode >= 400 {
		// Non-retryable client errors (400, 401, 403, etc.)
		return nil, fmt.Errorf("claude client error %d: %s", resp.StatusCode, string(respBody[:min(500, len(respBody))]))
	}

	var apiResp claudeResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &apiResp, nil
}

// extractClaudeContent pulls the text and tool usage from a Claude response.
// Handles standard text blocks, server tool use blocks, and thinking blocks
// (from extended thinking). Thinking content is not shown to users but is
// logged for diagnostics.
func extractClaudeContent(resp *claudeResponse) (string, []string) {
	var textParts []string
	var toolsUsed []string
	var blockTypes []string
	hasThinking := false

	for _, block := range resp.Content {
		blockTypes = append(blockTypes, block.Type)
		switch block.Type {
		case "text":
			if block.Text != "" {
				textParts = append(textParts, block.Text)
			}
		case "server_tool_use":
			if block.Name != "" {
				toolsUsed = append(toolsUsed, block.Name)
			}
		case "thinking":
			hasThinking = true
			// Thinking blocks are internal reasoning — not shown to user.
			// Log presence for diagnostics.
		}
	}

	text := strings.Join(textParts, "\n")

	// Diagnostic logging when no text was extracted but blocks exist
	if text == "" && len(resp.Content) > 0 {
		claudeLog.Warn().
			Strs("block_types", blockTypes).
			Bool("has_thinking", hasThinking).
			Str("stop_reason", resp.StopReason).
			Str("model", resp.Model).
			Msg("Claude response has content blocks but no text extracted")
	}

	return text, toolsUsed
}

// deriveToolName extracts the tool name from a versioned type string.
// "web_search_20250305" -> "web_search"
func deriveToolName(toolType string) string {
	// Find last underscore followed by digits
	for i := len(toolType) - 1; i >= 0; i-- {
		if toolType[i] == '_' {
			suffix := toolType[i+1:]
			allDigits := true
			for _, c := range suffix {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits && len(suffix) >= 8 {
				return toolType[:i]
			}
			break
		}
	}
	return toolType
}

// isClaudeTransient checks if an error is worth retrying.
func isClaudeTransient(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "deadline") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "server error") ||
		strings.Contains(errStr, "connection refused")
}

// buildClaudeMessage converts a ports.ChatMessage to a claudeMessage.
// For multimodal messages (with ContentBlocks), it builds an array of content blocks.
// For text-only messages, it uses a plain string.
func buildClaudeMessage(m ports.ChatMessage) claudeMessage {
	if !m.IsMultimodal() {
		return claudeMessage{Role: m.Role, Content: m.Content}
	}

	var blocks []claudeContentInput
	for _, b := range m.ContentBlocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				blocks = append(blocks, claudeContentInput{
					Type: "text",
					Text: b.Text,
				})
			}
		case "image":
			if b.Data != "" {
				mediaType := b.MediaType
				if mediaType == "" {
					mediaType = "image/jpeg"
				}
				blocks = append(blocks, claudeContentInput{
					Type: "image",
					Source: &claudeImageSource{
						Type:      "base64",
						MediaType: mediaType,
						Data:      b.Data,
					},
				})
			}
		case "document":
			// For supported document types (PDF), use the document type.
			if b.MediaType == "application/pdf" && b.Data != "" {
				block := claudeContentInput{
					Type: "document",
					Source: &claudeImageSource{
						Type:      "base64",
						MediaType: b.MediaType,
						Data:      b.Data,
					},
				}
				if b.FileName != "" {
					block.Title = b.FileName
				}
				blocks = append(blocks, block)
			} else if b.FileName != "" {
				blocks = append(blocks, claudeContentInput{
					Type: "text",
					Text: fmt.Sprintf("[Document attached: %s (%s) — binary content not directly processable]", b.FileName, b.MediaType),
				})
			}
		}
	}

	if len(blocks) == 0 {
		// Fallback: use text content
		return claudeMessage{Role: m.Role, Content: m.GetText()}
	}

	return claudeMessage{Role: m.Role, Content: blocks}
}
