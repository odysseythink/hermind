package terminal

import (
	"context"
	"encoding/json"
	"time"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// shellExecuteSchema is the JSON Schema for shell_execute tool arguments.
// The LLM sees this when deciding how to call the tool.
const shellExecuteSchema = `{
  "type": "object",
  "properties": {
    "command": {
      "type": "string",
      "description": "Shell command to run. Executed via /bin/sh -c or cmd /C."
    },
    "cwd": {
      "type": "string",
      "description": "Working directory for the command. Defaults to the agent's working directory."
    },
    "timeout_seconds": {
      "type": "number",
      "description": "Timeout in seconds. Default 180."
    },
    "stdin": {
      "type": "string",
      "description": "Input piped to the command's stdin."
    }
  },
  "required": ["command"]
}`

// shellExecuteArgs is the decoded argument shape.
type shellExecuteArgs struct {
	Command        string  `json:"command"`
	Cwd            string  `json:"cwd,omitempty"`
	TimeoutSeconds float64 `json:"timeout_seconds,omitempty"`
	Stdin          string  `json:"stdin,omitempty"`
}

// shellExecuteResult is the encoded result shape returned to the LLM.
type shellExecuteResult struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
}

// RegisterShellExecute wires the shell_execute tool into a Registry using
// the provided Backend for execution.
func RegisterShellExecute(reg *tool.Registry, backend Backend) {
	reg.Register(&tool.Entry{
		Name:        "shell_execute",
		Toolset:     "terminal",
		Description: "Run a shell command on the host. Returns stdout, stderr, exit code.",
		Emoji:       "⚡",
		Schema: core.ToolDefinition{
			Name:        "shell_execute",
			Description: "Run a shell command. Returns stdout, stderr, and exit code.",
			Parameters:  core.MustSchemaFromJSON([]byte(shellExecuteSchema)),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args shellExecuteArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if args.Command == "" {
				return tool.ToolError("command is required"), nil
			}

			timeout := 180 * time.Second
			if args.TimeoutSeconds > 0 {
				timeout = time.Duration(args.TimeoutSeconds * float64(time.Second))
			}

			res, err := backend.Execute(ctx, args.Command, &ExecOptions{
				Cwd:     args.Cwd,
				Timeout: timeout,
				Stdin:   args.Stdin,
			})
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}

			return tool.ToolResult(shellExecuteResult{
				Stdout:     res.Stdout,
				Stderr:     res.Stderr,
				ExitCode:   res.ExitCode,
				DurationMS: res.Duration.Milliseconds(),
			}), nil
		},
	})
}
