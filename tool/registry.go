package tool

import (
	"context"
	"encoding/json"
	"sync"
)

// Handler is the function signature every tool implements.
// Returns a JSON-encoded result string and an error. Errors from the
// handler are caught by Dispatch and returned as a ToolError JSON string
// so the LLM sees a structured error payload.
type Handler func(ctx context.Context, args json.RawMessage) (string, error)

// CheckFunc returns whether a tool is currently available (e.g., the
// required environment variables or external services are present).
type CheckFunc func() bool

// Entry describes a single tool registered in the Registry.
type Entry struct {
	Name           string
	Toolset        string // "terminal", "file", "web", ...
	Schema         ToolDefinition
	Handler        Handler
	CheckFn        CheckFunc
	RequiresEnv    []string
	IsInteractive  bool // interactive tools cannot run in parallel
	MaxResultChars int  // truncate results larger than this (0 = no limit)
	Description    string
	Emoji          string
}

// Registry holds all registered tools. Safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]*Entry
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]*Entry)}
}

// Register adds or replaces a tool entry. Safe to call concurrently.
func (r *Registry) Register(entry *Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[entry.Name] = entry
}

// Dispatch looks up a tool by name and invokes its handler with args.
// Returns a JSON string (always — errors are encoded as {"error": "..."}).
// The outer error return is reserved for fundamental dispatch failures
// that should not be fed back to the LLM (e.g., context canceled).
func (r *Registry) Dispatch(ctx context.Context, name string, args json.RawMessage) (string, error) {
	r.mu.RLock()
	entry, ok := r.entries[name]
	r.mu.RUnlock()
	if !ok {
		return ToolError("unknown tool: " + name), nil
	}

	// Execute the handler, catching errors and panics
	result, err := r.execHandler(ctx, entry, args)
	if err != nil {
		return ToolError(err.Error()), nil
	}

	// Truncate oversized results
	if entry.MaxResultChars > 0 && len(result) > entry.MaxResultChars {
		result = result[:entry.MaxResultChars] + "\n... [truncated]"
	}
	return result, nil
}

// execHandler invokes the handler with panic recovery.
func (r *Registry) execHandler(ctx context.Context, entry *Entry, args json.RawMessage) (result string, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = newPanicError(entry.Name, p)
		}
	}()
	return entry.Handler(ctx, args)
}

// IsInteractive reports whether a tool is marked interactive (cannot run in parallel).
func (r *Registry) IsInteractive(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[name]
	if !ok {
		return false
	}
	return entry.IsInteractive
}

// Definitions returns the OpenAI-format tool definitions for all registered
// tools that pass filter and whose CheckFn returns true.
func (r *Registry) Definitions(filter func(*Entry) bool) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]ToolDefinition, 0, len(r.entries))
	for _, entry := range r.entries {
		if filter != nil && !filter(entry) {
			continue
		}
		if entry.CheckFn != nil && !entry.CheckFn() {
			continue
		}
		defs = append(defs, entry.Schema)
	}
	return defs
}

// Entries returns a shallow copy of all registered entries that pass filter
// and whose CheckFn returns true. Safe to call concurrently.
func (r *Registry) Entries(filter func(*Entry) bool) []*Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Entry, 0, len(r.entries))
	for _, entry := range r.entries {
		if filter != nil && !filter(entry) {
			continue
		}
		if entry.CheckFn != nil && !entry.CheckFn() {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// ToolDefinition is the OpenAI function-calling schema shape.
// Providers convert this to their own wire format.
type ToolDefinition struct {
	Type     string      `json:"type"` // always "function"
	Function FunctionDef `json:"function"`
}

// FunctionDef describes the callable function inside a ToolDefinition.
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema for the input
}
