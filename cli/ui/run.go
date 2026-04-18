// cli/ui/run.go
package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
)

// RuntimeSnapshot is the set of config-derived values that live-reload
// can swap between user messages. Callers return a fresh snapshot every
// time GetRuntime is invoked.
type RuntimeSnapshot struct {
	Provider    provider.Provider
	AuxProvider provider.Provider
	Model       string
	AgentCfg    config.AgentConfig
}

// RunOptions holds the dependencies required to start a TUI REPL.
type RunOptions struct {
	Config    *config.Config
	Storage   storage.Storage
	ToolReg   *tool.Registry
	SessionID string
	// GetRuntime returns the current provider/model/agent-cfg set. Called
	// once per user message (dispatcher) and on every View render (for the
	// model-name in the header). Must be safe for concurrent use.
	GetRuntime func() RuntimeSnapshot
}

// Run starts the bubbletea TUI. Blocks until the user exits.
// The Engine is driven by a dispatcher closure stored on the Model that
// sends tea.Msg values back to the running Program via program.Send.
func Run(ctx context.Context, opts RunOptions) error {
	skin := DetectSkin()

	m := NewModel(ModelConfig{
		Config:     opts.Config,
		Storage:    opts.Storage,
		ToolReg:    opts.ToolReg,
		Skin:       skin,
		SessionID:  opts.SessionID,
		GetRuntime: opts.GetRuntime,
	})

	// Create the bubbletea program using a pointer so we can pass it to the
	// dispatcher closure (which needs program.Send to post messages from the
	// Engine goroutine).
	var program *tea.Program

	// The dispatcher: called from Update when the user submits a message.
	// It spawns a goroutine that runs the Engine and posts tea.Msg values
	// via program.Send. Reads the current runtime per invocation so that a
	// config reload mid-session applies to the next message.
	dispatcher := func(userInput string, history []message.Message) {
		go func() {
			rt := opts.GetRuntime()
			engine := agent.NewEngineWithToolsAndAux(
				rt.Provider, rt.AuxProvider, opts.Storage, opts.ToolReg,
				rt.AgentCfg, "cli",
			)
			engine.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
				program.Send(streamDeltaMsg{Delta: d})
			})
			engine.SetToolStartCallback(func(call message.ContentBlock) {
				program.Send(toolStartMsg{Call: call})
			})
			engine.SetToolResultCallback(func(call message.ContentBlock, result string) {
				program.Send(toolResultMsg{Call: call, Result: result})
			})

			result, err := engine.RunConversation(ctx, &agent.RunOptions{
				UserMessage: userInput,
				History:     history,
				SessionID:   opts.SessionID,
				Model:       rt.Model,
			})
			if err != nil {
				program.Send(convErrorMsg{Err: err})
				return
			}
			program.Send(convDoneMsg{Result: result})
		}()
	}

	// Install the dispatcher on the model.
	m.dispatch = dispatcher

	program = tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
