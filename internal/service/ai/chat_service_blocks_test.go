package ai

import (
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// TestDescribeContentBlocks verifies that zero-value ContentBlock entries are
// skipped safely without panic, and that the function returns expected labels.
func TestDescribeContentBlocks(t *testing.T) {
	tests := []struct {
		name   string
		blocks []ports.ContentBlock
		want   string
	}{
		{
			name:   "empty slice",
			blocks: []ports.ContentBlock{},
			want:   "[Media message]",
		},
		{
			name: "zero-value block only",
			blocks: []ports.ContentBlock{
				{}, // zero-value: Type == ""
			},
			want: "[Media message]",
		},
		{
			name: "mixed zero-value and image",
			blocks: []ports.ContentBlock{
				{},                    // zero-value — should be skipped
				{Type: "image"},       // valid image block
			},
			want: "[Image]",
		},
		{
			name: "mixed zero-value and document with filename",
			blocks: []ports.ContentBlock{
				{},                                           // zero-value
				{Type: "document", FileName: "report.pdf"},  // valid document
			},
			want: "[Document: report.pdf]",
		},
		{
			name: "document without filename",
			blocks: []ports.ContentBlock{
				{Type: "document"},
			},
			want: "[Document]",
		},
		{
			name: "multiple valid blocks",
			blocks: []ports.ContentBlock{
				{Type: "image"},
				{Type: "document", FileName: "data.csv"},
			},
			want: "[Image] [Document: data.csv]",
		},
		{
			name: "all zero-value blocks",
			blocks: []ports.ContentBlock{
				{},
				{},
				{},
			},
			want: "[Media message]",
		},
		{
			name: "text block is ignored by describeContentBlocks",
			blocks: []ports.ContentBlock{
				{Type: "text", Text: "hello"},
			},
			want: "[Media message]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := describeContentBlocks(tc.blocks)
			if got != tc.want {
				t.Errorf("describeContentBlocks(%v) = %q; want %q", tc.blocks, got, tc.want)
			}
		})
	}
}

// TestExtractClaudeContentZeroValueBlocks verifies that extractClaudeContent
// handles zero-value claudeContentBlock entries without panic.
func TestExtractClaudeContentZeroValueBlocks(t *testing.T) {
	resp := &claudeResponse{
		Content: []claudeContentBlock{
			{},                              // zero-value block — Type == ""
			{Type: "text", Text: "hello"},   // valid text block
			{},                              // another zero-value block
		},
		StopReason: "end_turn",
		Model:      "claude-test",
	}

	text, toolsUsed := extractClaudeContent(resp)

	if text != "hello" {
		t.Errorf("expected text=%q, got %q", "hello", text)
	}
	if len(toolsUsed) != 0 {
		t.Errorf("expected no toolsUsed, got %v", toolsUsed)
	}
}
