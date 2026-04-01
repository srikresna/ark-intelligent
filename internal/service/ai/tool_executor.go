package ai

import (
	"context"
	"encoding/json"
)

// MemoryToolExecutor implements ToolExecutor by routing tool_use requests
// to the appropriate handler. Currently supports the memory_20250818 tool.
type MemoryToolExecutor struct {
	memoryStore *MemoryStore
}

// NewMemoryToolExecutor creates a ToolExecutor backed by the given MemoryStore.
func NewMemoryToolExecutor(store *MemoryStore) *MemoryToolExecutor {
	return &MemoryToolExecutor{memoryStore: store}
}

// Execute processes a tool call and returns the result text.
func (e *MemoryToolExecutor) Execute(ctx context.Context, userID int64, toolName string, input map[string]any) string {
	switch toolName {
	case "memory":
		return e.executeMemory(ctx, userID, input)
	default:
		return "Tool not supported: " + toolName
	}
}

// executeMemory handles memory tool commands by parsing the input map
// into a memoryCommand and delegating to the MemoryStore.
func (e *MemoryToolExecutor) executeMemory(ctx context.Context, userID int64, input map[string]any) string {
	// Parse the input map into a memoryCommand struct
	data, err := json.Marshal(input)
	if err != nil {
		return "Error parsing memory command: " + err.Error()
	}

	var cmd memoryCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		return "Error parsing memory command: " + err.Error()
	}

	return e.memoryStore.Execute(ctx, userID, cmd)
}
