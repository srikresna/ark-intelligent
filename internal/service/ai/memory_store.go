package ai

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var memLog = logger.Component("memory-store")

// MemoryStore provides per-user file-based memory that integrates with
// the Anthropic memory_20250818 tool. Memory is stored as virtual files
// in a per-user namespace, persisted via a MemoryPersister backend.
//
// The memory tool uses filesystem-like commands:
//   - view <path>    → list directory or read file
//   - create <path>  → create file with file_text
//   - edit <path>    → edit file (old_str → new_str)
//   - delete <path>  → delete file
type MemoryStore struct {
	mu        sync.RWMutex
	persister MemoryPersister
	// In-memory cache: userID → path → content
	cache map[int64]map[string]string
}

// MemoryPersister is the storage backend for memory files.
type MemoryPersister interface {
	// LoadAll loads all memory files for a user.
	LoadAll(ctx context.Context, userID int64) (map[string]string, error)
	// Save persists a single memory file.
	Save(ctx context.Context, userID int64, path string, content string) error
	// Delete removes a memory file.
	Delete(ctx context.Context, userID int64, path string) error
}

// NewMemoryStore creates a MemoryStore with the given persister.
func NewMemoryStore(persister MemoryPersister) *MemoryStore {
	return &MemoryStore{
		persister: persister,
		cache:     make(map[int64]map[string]string),
	}
}

// memoryCommand represents a parsed memory tool command.
type memoryCommand struct {
	Command  string `json:"command"`             // "view", "create", "edit", "delete"
	Path     string `json:"path"`                // file/directory path
	FileText string `json:"file_text,omitempty"` // for "create"
	OldStr   string `json:"old_str,omitempty"`   // for "edit"
	NewStr   string `json:"new_str,omitempty"`   // for "edit"
}

// Execute runs a memory tool command and returns the result text.
func (ms *MemoryStore) Execute(ctx context.Context, userID int64, cmd memoryCommand) string {
	switch cmd.Command {
	case "view":
		return ms.view(ctx, userID, cmd.Path)
	case "create":
		return ms.create(ctx, userID, cmd.Path, cmd.FileText)
	case "edit":
		return ms.edit(ctx, userID, cmd.Path, cmd.OldStr, cmd.NewStr)
	case "delete":
		return ms.del(ctx, userID, cmd.Path)
	default:
		return fmt.Sprintf("Unknown command: %s", cmd.Command)
	}
}

// view lists a directory or reads a file.
// Holds RLock during the entire operation to prevent concurrent map read/write panic.
func (ms *MemoryStore) view(ctx context.Context, userID int64, p string) string {
	ms.ensureLoaded(ctx, userID)

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	files := ms.cache[userID]
	if files == nil {
		return "No files or directories found at " + normalizePath(p)
	}

	// Normalize path
	p = normalizePath(p)

	// Check if exact file match
	if content, ok := files[p]; ok {
		return content
	}

	// List directory: find files under this path
	dir := p
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}

	var entries []string
	seen := make(map[string]bool)
	for fp := range files {
		if strings.HasPrefix(fp, dir) || (dir == "/" && len(fp) > 0) {
			rel := strings.TrimPrefix(fp, dir)
			if rel == "" {
				continue
			}
			// Show immediate children only
			parts := strings.SplitN(rel, "/", 2)
			entry := parts[0]
			if len(parts) > 1 {
				entry += "/" // it's a directory
			}
			if !seen[entry] {
				seen[entry] = true
				entries = append(entries, entry)
			}
		}
	}

	if len(entries) == 0 {
		return "No files or directories found at " + p
	}

	sort.Strings(entries)
	return strings.Join(entries, "\n")
}

// create creates a new file.
func (ms *MemoryStore) create(ctx context.Context, userID int64, p string, content string) string {
	p = normalizePath(p)
	ms.ensureLoaded(ctx, userID)

	ms.mu.Lock()
	ms.cache[userID][p] = content
	ms.mu.Unlock()

	if ms.persister != nil {
		if err := ms.persister.Save(ctx, userID, p, content); err != nil {
			memLog.Warn().Err(err).Int64("user_id", userID).Str("path", p).Msg("failed to persist memory file")
		}
	}

	return fmt.Sprintf("File created successfully at %s", p)
}

// edit modifies an existing file using old_str → new_str replacement.
func (ms *MemoryStore) edit(ctx context.Context, userID int64, p string, oldStr, newStr string) string {
	p = normalizePath(p)
	ms.ensureLoaded(ctx, userID)

	ms.mu.Lock()
	defer ms.mu.Unlock()

	files := ms.cache[userID]
	if files == nil {
		return fmt.Sprintf("File not found: %s", p)
	}

	content, ok := files[p]
	if !ok {
		return fmt.Sprintf("File not found: %s", p)
	}

	if !strings.Contains(content, oldStr) {
		return fmt.Sprintf("String not found in %s", p)
	}

	newContent := strings.Replace(content, oldStr, newStr, 1)
	files[p] = newContent

	if ms.persister != nil {
		if err := ms.persister.Save(ctx, userID, p, newContent); err != nil {
			memLog.Warn().Err(err).Int64("user_id", userID).Str("path", p).Msg("failed to persist memory edit")
		}
	}

	return fmt.Sprintf("File edited successfully: %s", p)
}

// del deletes a file.
func (ms *MemoryStore) del(ctx context.Context, userID int64, p string) string {
	p = normalizePath(p)
	ms.ensureLoaded(ctx, userID)

	ms.mu.Lock()
	if files, ok := ms.cache[userID]; ok {
		delete(files, p)
	}
	ms.mu.Unlock()

	if ms.persister != nil {
		if err := ms.persister.Delete(ctx, userID, p); err != nil {
			memLog.Warn().Err(err).Int64("user_id", userID).Str("path", p).Msg("failed to delete memory file")
		}
	}

	return fmt.Sprintf("File deleted: %s", p)
}

// ensureLoaded loads the user's memory files from the persister into cache
// if not already cached. Safe for concurrent access.
func (ms *MemoryStore) ensureLoaded(ctx context.Context, userID int64) {
	ms.mu.RLock()
	if _, ok := ms.cache[userID]; ok {
		ms.mu.RUnlock()
		return
	}
	ms.mu.RUnlock()

	// Load from persister
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Double-check after acquiring write lock
	if _, ok := ms.cache[userID]; ok {
		return
	}

	files := make(map[string]string)
	if ms.persister != nil {
		loaded, err := ms.persister.LoadAll(ctx, userID)
		if err != nil {
			memLog.Warn().Err(err).Int64("user_id", userID).Msg("failed to load memory files")
		} else if loaded != nil {
			files = loaded
		}
	}
	ms.cache[userID] = files
}

// normalizePath cleans up a memory file path.
func normalizePath(p string) string {
	p = path.Clean(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
