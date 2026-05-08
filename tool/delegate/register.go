// tool/delegate/register.go
package delegate

import (
	"encoding/json"

	"github.com/odysseythink/hermind/tool"
)

// RegisterDelegate registers the delegate tool bound to a SubagentRunner.
// If runner is nil, the tool is still registered but returns an error
// at dispatch time. The CLI wires a real runner; tests can inject fakes.
func RegisterDelegate(reg *tool.Registry, runner SubagentRunner) {
	reg.Register(&tool.Entry{
		Name:        "delegate",
		Toolset:     "delegate",
		Description: "Delegate a self-contained task to a subagent. The subagent has its own budget and history.",
		Emoji:       "👥",
		Handler:     newDelegateHandler(runner),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "delegate",
				Description: "Run a fresh subagent on a specific, self-contained task.",
				Parameters:  json.RawMessage(delegateSchema),
			},
		},
	})
}
