package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
)

// ClarifyRequest is emitted by the clarify tool when the model
// wants the user to answer a question. The REPL detects this by the
// presence of a "clarify_pending": true marker in the returned JSON.
func RegisterClarify(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "clarify",
		Toolset:     "meta",
		Description: "Ask the user a clarification question.",
		Emoji:       "❓",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "clarify",
				Description: "Pause and request user clarification. Use this instead of guessing when a requirement is ambiguous.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"question":{"type":"string"}},
  "required":["question"]
}`),
			},
		},
		IsInteractive: true,
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct{ Question string `json:"question"` }
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Question) == "" {
				return tool.ToolError("question is required"), nil
			}
			return tool.ToolResult(map[string]any{
				"clarify_pending": true,
				"question":        args.Question,
			}), nil
		},
	})
}

// RegisterCheckpoint wires checkpoint_save / checkpoint_restore
// against a directory under $HERMES_HOME/checkpoints.
func RegisterCheckpoint(reg *tool.Registry) {
	base := defaultCheckpointDir()
	reg.Register(&tool.Entry{
		Name:        "checkpoint_save",
		Toolset:     "meta",
		Description: "Save a named checkpoint of arbitrary JSON state.",
		Emoji:       "💾",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name: "checkpoint_save",
				Description: "Save a checkpoint. Name must be alphanumeric. State is stored as JSON under $HERMES_HOME/checkpoints/<name>.json.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "name":{"type":"string"},
    "state":{"type":"object"}
  },
  "required":["name","state"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Name  string          `json:"name"`
				State json.RawMessage `json:"state"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if !validName(args.Name) {
				return tool.ToolError("name must be alphanumeric"), nil
			}
			if err := os.MkdirAll(base, 0o755); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			if err := os.WriteFile(filepath.Join(base, args.Name+".json"), args.State, 0o644); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"ok": true, "path": filepath.Join(base, args.Name+".json")}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "checkpoint_restore",
		Toolset:     "meta",
		Description: "Load a previously saved checkpoint.",
		Emoji:       "♻️",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name: "checkpoint_restore",
				Description: "Load a named checkpoint and return its JSON state.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"name":{"type":"string"}},
  "required":["name"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct{ Name string `json:"name"` }
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if !validName(args.Name) {
				return tool.ToolError("name must be alphanumeric"), nil
			}
			data, err := os.ReadFile(filepath.Join(base, args.Name+".json"))
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"state": json.RawMessage(data)}), nil
		},
	})
}

// RegisterSessionSearch wires session_search against the shared
// storage.SearchMessages + SearchMemories indexes.
func RegisterSessionSearch(reg *tool.Registry, store storage.Storage) {
	if store == nil {
		return
	}
	reg.Register(&tool.Entry{
		Name:        "session_search",
		Toolset:     "meta",
		Description: "Search prior session messages and memories.",
		Emoji:       "🔎",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name: "session_search",
				Description: "Full-text search across past session messages.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "query":{"type":"string"},
    "limit":{"type":"number"}
  },
  "required":["query"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
				Limit int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if args.Limit <= 0 {
				args.Limit = 20
			}
			hits, err := store.SearchMessages(ctx, args.Query, &storage.SearchOptions{Limit: args.Limit})
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			out := make([]map[string]any, 0, len(hits))
			for _, h := range hits {
				out = append(out, map[string]any{
					"session_id": h.SessionID,
					"snippet":    h.Snippet,
					"rank":       h.Rank,
				})
			}
			return tool.ToolResult(map[string]any{"hits": out}), nil
		},
	})
}

// RegisterApproval wires a simple gate tool. It is IsInteractive so
// parallel executor won't schedule it alongside others, and the REPL
// can hook on its return.
func RegisterApproval(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "approval_request",
		Toolset:     "meta",
		Description: "Pause and ask the user to approve a destructive action.",
		Emoji:       "🛑",
		IsInteractive: true,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name: "approval_request",
				Description: "Request human approval. The REPL should prompt the user and inject the answer before continuing.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "action":{"type":"string"},
    "reason":{"type":"string"}
  },
  "required":["action"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Action string `json:"action"`
				Reason string `json:"reason,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Action) == "" {
				return tool.ToolError("action is required"), nil
			}
			return tool.ToolResult(map[string]any{
				"approval_pending": true,
				"action":           args.Action,
				"reason":           args.Reason,
			}), nil
		},
	})
}

func defaultCheckpointDir() string {
	if v := os.Getenv("HERMES_HOME"); v != "" {
		return filepath.Join(v, "checkpoints")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hermes", "checkpoints")
}

func validName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// Unused guard: keep fmt imported if we later add more diagnostics.
var _ = fmt.Sprintf
var _ = time.Now
