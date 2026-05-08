package rl

import (
	"context"
	"encoding/json"
	"fmt"

	rlpkg "github.com/odysseythink/hermind/rl"
	"github.com/odysseythink/hermind/tool"
)

// RegisterAll registers all RL training tools into the given registry.
func RegisterAll(reg *tool.Registry, manager *rlpkg.Manager) {
	reg.Register(&tool.Entry{
		Name:        "rl_get_current_config",
		Toolset:     "rl",
		Description: "Get the current RL training configuration",
		Emoji:       "\u2699\ufe0f",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			cfg := rlpkg.DefaultConfig()
			data, _ := json.MarshalIndent(cfg, "", "  ")
			return string(data), nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "rl_get_current_config",
				Description: "Get the current RL training configuration.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "rl_start_training",
		Toolset:     "rl",
		Description: "Start a training run",
		Emoji:       "\U0001f680",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.Command == "" {
				return "", fmt.Errorf("command is required")
			}
			id, err := manager.Start(ctx, params.Command, params.Args)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf(`{"run_id":"%s","status":"started"}`, id), nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "rl_start_training",
				Description: "Start a new RL training run.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"},"args":{"type":"array","items":{"type":"string"}}},"required":["command"]}`),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "rl_check_status",
		Toolset:     "rl",
		Description: "Check training run status",
		Emoji:       "\U0001f4ca",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				RunID string `json:"run_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			status := manager.Status(params.RunID)
			data, _ := json.MarshalIndent(status, "", "  ")
			return string(data), nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "rl_check_status",
				Description: "Check the status of a training run.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"run_id":{"type":"string"}},"required":["run_id"]}`),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "rl_stop_training",
		Toolset:     "rl",
		Description: "Stop a training run",
		Emoji:       "\U0001f6d1",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				RunID string `json:"run_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if err := manager.Stop(params.RunID); err != nil {
				return "", err
			}
			return fmt.Sprintf(`{"run_id":"%s","status":"stopped"}`, params.RunID), nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "rl_stop_training",
				Description: "Stop an active training run.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"run_id":{"type":"string"}},"required":["run_id"]}`),
			},
		},
	})
}
