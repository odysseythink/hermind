package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

func newBridgeBackedSkill(name, toolset, configKey, desc string, schema *core.Schema, destructive map[string]bool, tc *ToolContext, deps BuilderDeps) *tool.Entry {
	return &tool.Entry{
		Name:           name,
		Toolset:        toolset,
		Description:    desc,
		Emoji:          "📅",
		MaxResultChars: 12 * 1024,
		CheckFn: func() bool {
			if deps.Cfg == nil {
				return false
			}
			if deps.Bridge == nil {
				return false
			}
			if tc.User == nil {
				return false
			}
			_, ok := parseGmailBridgeConfig(tc.Settings[configKey])
			return ok
		},
		Schema: core.ToolDefinition{
			Name:        name,
			Description: desc,
			Parameters:  schema,
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args map[string]any
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error("invalid args: " + err.Error()), nil
			}
			action, _ := args["action"].(string)
			if action == "" {
				return tool.Error("action is required"), nil
			}

			cfg, ok := parseGmailBridgeConfig(tc.Settings[configKey])
			if !ok {
				return tool.Error(name + " not configured"), nil
			}

			if destructive[action] && tc.Approval != nil {
				desc := fmt.Sprintf("%s %s", name, action)
				if to, _ := args["to"].(string); to != "" {
					desc += " to " + to
				}
				if ok, reason := tc.Approval(ctx, name+":"+action, args, desc); !ok {
					return tool.Error("rejected: " + reason), nil
				}
			}

			// Parse attachments and inline their text into the body.
			if rawAtts, ok := args["attachments"].([]any); ok && len(rawAtts) > 0 {
				if deps.Collector != nil {
					var atts []oauth.Attachment
					for _, item := range rawAtts {
						blob, _ := json.Marshal(item)
						var a oauth.Attachment
						_ = json.Unmarshal(blob, &a)
						atts = append(atts, a)
					}
					attText, err := oauth.ParseAttachments(ctx, deps.Collector, atts)
					if err != nil {
						return tool.Error("attachment parse: " + err.Error()), nil
					}
					if attText != "" {
						body, _ := args["body"].(string)
						args["body"] = body + attText
					}
				}
			}

			tc.Emit(name + ": " + action)

			params := make(map[string]any, len(args))
			for k, v := range args {
				if k == "action" {
					continue
				}
				params[k] = v
			}
			data, err := deps.Bridge.Call(ctx, cfg.DeploymentID, cfg.APIKey, action, params)
			if err != nil {
				return tool.Error(err.Error()), nil
			}
			return string(data), nil
		},
	}
}
