package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gosimple/slug"
	"github.com/odysseythink/hermind/backend/internal/agent/flow"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

func flowToEntry(f services.FlowSummary, flowSvc *services.AgentFlowService, executor *flow.Executor, emit StatusEmitter) *tool.Entry {
	name := "flow-" + slug.Make(strings.ToLower(f.Name))
	desc := f.Description
	if desc == "" {
		desc = "User-defined agent flow: " + f.Name
	}
	return &tool.Entry{
		Name:           name,
		Toolset:        "flow",
		Description:    desc,
		MaxResultChars: 4 * 1024,
		Schema: core.ToolDefinition{
			Name:        name,
			Description: desc,
			Parameters:  nil,
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			emit("Invoking flow: " + f.Name)
			loaded, err := flowSvc.LoadFlow(f.UUID)
			if err != nil {
				return tool.Error("flow load: " + err.Error()), nil
			}

			if executor == nil {
				return tool.Error("flow execution requires AgentFlowExecutor (not configured)"), nil
			}

			var args map[string]string
			_ = json.Unmarshal(raw, &args)
			if args == nil {
				args = map[string]string{}
			}
			args["__flow_invoked_by"] = "agent"

			output, err := executor.Run(ctx, loaded, args, emit)
			if err != nil {
				return tool.Error(err.Error()), nil
			}
			return tool.Result(map[string]any{
				"flow":   f.Name,
				"output": output,
			}), nil
		},
	}
}
