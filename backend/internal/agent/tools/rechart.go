package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

var allowedChartTypes = map[string]bool{
	"line": true, "bar": true, "pie": true, "area": true, "scatter": true,
}

func NewRechartSkill(tc *ToolContext) *tool.Entry {
	return &tool.Entry{
		Name:           "rechart",
		Toolset:        "chart",
		Description:    "Generate a chart (line/bar/pie/area/scatter) from data. The frontend renders the returned chart spec.",
		MaxResultChars: 4 * 1024,
		Schema: core.ToolDefinition{
			Name:        "rechart",
			Description: "Generate a chart from data",
			Parameters:  rechartSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Type string         `json:"type"`
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}
			if !allowedChartTypes[args.Type] {
				return tool.Error(fmt.Sprintf("chart type %q not in [line,bar,pie,area,scatter]", args.Type)), nil
			}
			if args.Data == nil {
				return tool.Error("data is required"), nil
			}
			tc.Emit("Rendering " + args.Type + " chart")
			return tool.Result(map[string]any{
				"chart_type": args.Type,
				"spec":       args.Data,
				"renderable": true,
			}), nil
		},
	}
}

func rechartSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"type": {"type": "string", "enum": ["line", "bar", "pie", "area", "scatter"]},
			"data": {"type": "object"}
		},
		"required": ["type", "data"]
	}`))
}
