package memprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool"
)

// execCommand is indirected so tests can substitute a fake shell command.
var execCommand = exec.CommandContext

// Byterover is a Provider backed by the local `brv` CLI. It exposes
// three tools — brv_query, brv_curate, brv_status — each of which
// shells out to `brv <subcommand>` and returns stdout/stderr to the
// caller.
type Byterover struct {
	cfg       config.ByteroverConfig
	brvPath   string
	sessionID string
}

func NewByterover(cfg config.ByteroverConfig) *Byterover {
	return &Byterover{cfg: cfg}
}

func (b *Byterover) Name() string { return "byterover" }

func (b *Byterover) Initialize(ctx context.Context, sessionID string) error {
	b.sessionID = sessionID
	path := b.cfg.BrvPath
	if path == "" {
		found, err := exec.LookPath("brv")
		if err != nil {
			return fmt.Errorf("byterover: brv CLI not found: %w", err)
		}
		path = found
	}
	b.brvPath = path
	return nil
}

func (b *Byterover) Shutdown(ctx context.Context) error { return nil }

// SyncTurn is a no-op: Byterover curates memory explicitly via the
// brv_curate tool rather than auto-ingesting every turn.
func (b *Byterover) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	return nil
}

// run executes `brv <args...>` and returns the combined stdout+stderr.
func (b *Byterover) run(ctx context.Context, args ...string) (string, error) {
	cmd := execCommand(ctx, b.brvPath, args...)
	if b.cfg.Cwd != "" {
		cmd.Dir = b.cfg.Cwd
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("byterover: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (b *Byterover) RegisterTools(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "brv_query",
		Toolset:     "memory",
		Description: "Query Byterover for curated context.",
		Emoji:       "🧳",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "brv_query",
				Description: "Run `brv query <text>` and return stdout.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"query":{"type":"string"}},
  "required":["query"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct{ Query string `json:"query"` }
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Query) == "" {
				return tool.ToolError("query is required"), nil
			}
			out, err := b.run(ctx, "query", args.Query)
			if err != nil {
				return tool.ToolError(err.Error() + ": " + out), nil
			}
			return tool.ToolResult(map[string]any{"output": out}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "brv_curate",
		Toolset:     "memory",
		Description: "Curate a new piece of context in Byterover.",
		Emoji:       "🗃",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "brv_curate",
				Description: "Run `brv curate <content>` to store context.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"content":{"type":"string"}},
  "required":["content"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct{ Content string `json:"content"` }
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Content) == "" {
				return tool.ToolError("content is required"), nil
			}
			out, err := b.run(ctx, "curate", args.Content)
			if err != nil {
				return tool.ToolError(err.Error() + ": " + out), nil
			}
			return tool.ToolResult(map[string]any{"output": out}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "brv_status",
		Toolset:     "memory",
		Description: "Show Byterover context status.",
		Emoji:       "📊",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "brv_status",
				Description: "Run `brv status` and return stdout.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			out, err := b.run(ctx, "status")
			if err != nil {
				return tool.ToolError(err.Error() + ": " + out), nil
			}
			return tool.ToolResult(map[string]any{"output": out}), nil
		},
	})
}
