// tool/delegate/delegate.go
package delegate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/tool"
)

const delegateSchema = `{
  "type": "object",
  "properties": {
    "task":          { "type": "string", "description": "A specific, self-contained task for the subagent to complete" },
    "context":       { "type": "string", "description": "Optional background context" },
    "max_turns":     { "type": "number", "description": "Max turns the subagent may take (default 20, max 50)" }
  },
  "required": ["task"]
}`

type delegateArgs struct {
	Task     string `json:"task"`
	Context  string `json:"context,omitempty"`
	MaxTurns int    `json:"max_turns,omitempty"`
}

type delegateResult struct {
	Response   string `json:"response"`
	Iterations int    `json:"iterations"`
	ToolCalls  int    `json:"tool_calls"`
}

// SubagentRunner is an injection point for running a subagent turn.
// The CLI wires this to a closure that spawns a fresh Engine and runs one
// conversation without tools that would cause recursion (delegate itself).
type SubagentRunner func(ctx context.Context, task, extraContext string, maxTurns int) (*SubagentResult, error)

// SubagentResult is returned by a SubagentRunner.
type SubagentResult struct {
	Response   message.Message
	Iterations int
	ToolCalls  int
}

// newDelegateHandler returns a handler bound to the given runner.
func newDelegateHandler(runner SubagentRunner) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args delegateArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if args.Task == "" {
			return tool.ToolError("task is required"), nil
		}
		maxTurns := args.MaxTurns
		if maxTurns <= 0 {
			maxTurns = 20
		}
		if maxTurns > 50 {
			maxTurns = 50
		}
		if runner == nil {
			return tool.ToolError("delegate: no subagent runner configured"), nil
		}

		result, err := runner(ctx, args.Task, args.Context, maxTurns)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("subagent failed: %s", err.Error())), nil
		}
		responseText := ""
		if result.Response.Content.IsText() {
			responseText = result.Response.Content.Text()
		} else {
			for _, b := range result.Response.Content.Blocks() {
				if b.Type == "text" {
					responseText += b.Text
				}
			}
		}
		return tool.ToolResult(delegateResult{
			Response:   responseText,
			Iterations: result.Iterations,
			ToolCalls:  result.ToolCalls,
		}), nil
	}
}
