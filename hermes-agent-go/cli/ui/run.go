// cli/ui/run.go
package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/storage"
	"github.com/nousresearch/hermes-agent/tool"
)

// RunOptions holds the dependencies required to start a TUI REPL.
type RunOptions struct {
	Config      *config.Config
	Storage     storage.Storage
	Provider    provider.Provider
	AuxProvider provider.Provider // may be nil
	ToolReg     *tool.Registry
	AgentCfg    config.AgentConfig
	SessionID   string
	Model       string
}

// Run starts the bubbletea TUI. Blocks until the user exits.
// The Engine is driven by a dispatcher closure stored on the Model that
// sends tea.Msg values back to the running Program via program.Send.
func Run(ctx context.Context, opts RunOptions) error {
	skin := DetectSkin()

	m := NewModel(ModelConfig{
		Config:    opts.Config,
		Storage:   opts.Storage,
		Provider:  opts.Provider,
		ToolReg:   opts.ToolReg,
		AgentCfg:  opts.AgentCfg,
		Skin:      skin,
		SessionID: opts.SessionID,
		Model:     opts.Model,
	})

	// Create the bubbletea program using a pointer so we can pass it to the
	// dispatcher closure (which needs program.Send to post messages from the
	// Engine goroutine).
	var program *tea.Program

	// The dispatcher: called from Update when the user submits a message.
	// It spawns a goroutine that runs the Engine and posts tea.Msg values
	// via program.Send.
	dispatcher := func(userInput string, history []message.Message) {
		go func() {
			engine := agent.NewEngineWithToolsAndAux(
				opts.Provider, opts.AuxProvider, opts.Storage, opts.ToolReg,
				opts.AgentCfg, "cli",
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
				Model:       opts.Model,
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
