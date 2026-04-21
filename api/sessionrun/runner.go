// Package sessionrun runs one agent.Engine invocation per HTTP request
// and publishes its streaming events to an api.StreamHub.
package sessionrun

import (
	"context"
	"errors"
	"fmt"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/skills"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
)

// EventPublisher is the minimal interface Run needs from api.StreamHub.
// Keeping it as an interface avoids an import cycle (api/sessionrun must
// not import api).
type EventPublisher interface {
	Publish(event Event)
}

// Event mirrors api.StreamEvent.
type Event struct {
	Type      string
	SessionID string
	Data      any
}

// Deps bundles everything Run needs to build an Engine.
type Deps struct {
	Provider    provider.Provider
	AuxProvider provider.Provider
	Storage     storage.Storage
	ToolReg     *tool.Registry
	SkillsReg   *skills.Registry
	AgentCfg    config.AgentConfig
	Hub         EventPublisher
}

// Request is one user message submission.
type Request struct {
	SessionID   string
	UserMessage string
	Model       string
}

// Run builds an Engine, wires stream callbacks into Hub, and invokes
// RunConversation. It publishes status and token events through Hub.
// Returns the Engine's error verbatim (including context.Canceled).
// On panic, recovers and publishes a status(error) event.
func Run(ctx context.Context, deps Deps, req Request) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			deps.Hub.Publish(Event{
				Type:      "status",
				SessionID: req.SessionID,
				Data:      map[string]any{"state": "error", "error": "internal: " + toError(rec).Error()},
			})
			err = toError(rec)
		}
	}()

	engine := agent.NewEngineWithToolsAndAux(
		deps.Provider, deps.AuxProvider, deps.Storage, deps.ToolReg,
		deps.AgentCfg, "web",
	)
	if deps.SkillsReg != nil {
		engine.SetActiveSkillsProvider(ActiveSkillsBridge(deps.SkillsReg))
	}
	engine.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
		deps.Hub.Publish(Event{
			Type:      "token",
			SessionID: req.SessionID,
			Data:      map[string]any{"text": d.Content},
		})
	})
	engine.SetToolStartCallback(func(call message.ContentBlock) {
		deps.Hub.Publish(Event{
			Type:      "tool_call",
			SessionID: req.SessionID,
			Data:      call,
		})
	})
	engine.SetToolResultCallback(func(call message.ContentBlock, result string) {
		deps.Hub.Publish(Event{
			Type:      "tool_result",
			SessionID: req.SessionID,
			Data:      map[string]any{"call": call, "result": result},
		})
	})

	deps.Hub.Publish(Event{
		Type:      "status",
		SessionID: req.SessionID,
		Data:      map[string]any{"state": "running"},
	})

	result, err := engine.RunConversation(ctx, &agent.RunOptions{
		UserMessage: req.UserMessage,
		SessionID:   req.SessionID,
		Model:       req.Model,
	})

	switch {
	case errors.Is(err, context.Canceled):
		deps.Hub.Publish(Event{
			Type:      "status",
			SessionID: req.SessionID,
			Data:      map[string]any{"state": "cancelled"},
		})
		return err
	case err != nil:
		deps.Hub.Publish(Event{
			Type:      "status",
			SessionID: req.SessionID,
			Data:      map[string]any{"state": "error", "error": err.Error()},
		})
		return err
	default:
		assistantText := ""
		if result != nil && result.Response.Content.IsText() {
			assistantText = result.Response.Content.Text()
		}
		deps.Hub.Publish(Event{
			Type:      "message_complete",
			SessionID: req.SessionID,
			Data:      map[string]any{"assistant_text": assistantText},
		})
		deps.Hub.Publish(Event{
			Type:      "status",
			SessionID: req.SessionID,
			Data:      map[string]any{"state": "idle"},
		})
		return nil
	}
}

func toError(rec any) error {
	if e, ok := rec.(error); ok {
		return e
	}
	return fmt.Errorf("%v", rec)
}

// ActiveSkillsBridge converts a skills.Registry into the
// agent.ActiveSkill shape the Engine expects.
func ActiveSkillsBridge(reg *skills.Registry) func() []agent.ActiveSkill {
	return func() []agent.ActiveSkill {
		active := reg.Active()
		out := make([]agent.ActiveSkill, 0, len(active))
		for _, s := range active {
			out = append(out, agent.ActiveSkill{
				Name:        s.Name,
				Description: s.Description,
				Body:        s.Body,
			})
		}
		return out
	}
}
