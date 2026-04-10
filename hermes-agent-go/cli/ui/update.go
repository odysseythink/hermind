// cli/ui/update.go
package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nousresearch/hermes-agent/message"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.input.SetWidth(msg.Width - 2)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			// Ctrl+C: interrupt current op or exit if idle
			if m.state == StateIdle {
				return m.quit()
			}
			// Cancel the in-flight conversation (the goroutine watches the context via CancelFunc held by run.go).
			// For Plan 4, we simply transition back to idle and drop the partial output.
			// Plan 6 wires proper context cancellation through a channel.
			m.state = StateIdle
			m.appendRenderedLine(m.skin.Muted.Render("[interrupted]"))
			return m, nil

		case "ctrl+d":
			// Exit only when the input is empty (so it doesn't eat user text).
			if m.input.Value() == "" && m.state == StateIdle {
				return m.quit()
			}

		case "ctrl+l":
			// Clear the viewport (keep history in memory)
			m.rendered = nil
			m.viewport.SetContent("")
			return m, nil

		case "enter":
			// Submit only if not already streaming
			if m.state != StateIdle {
				return m, nil
			}
			text := m.input.Value()
			if text == "" {
				return m, nil
			}
			m.input.Reset()

			// Handle slash commands inline
			if handled, cmd := m.handleSlashCommand(text); handled {
				return m, cmd
			}

			// Render the user message into the conversation
			m.appendRenderedLine(m.skin.Accent.Render(m.skin.PromptChar+" ") + text)
			m.state = StateStreaming
			m.appendRenderedLine(m.skin.Muted.Render(m.skin.ThinkingChar + " Thinking..."))

			// Start the Engine goroutine — run.go provides the actual implementation
			// via a cmd function stored on the Model. For Plan 4 we return a direct cmd.
			return m, m.startConversation(text)
		}

	case streamDeltaMsg:
		if msg.Delta != nil && msg.Delta.Content != "" {
			m.streamingText.WriteString(msg.Delta.Content)
			// Re-render the last assistant line in-place
			m.refreshStreamingLine()
		}

	case toolStartMsg:
		m.state = StateToolRunning
		m.toolCalls++
		m.streamingTool = &streamingToolState{
			Name:  msg.Call.ToolUseName,
			Input: string(msg.Call.ToolUseInput),
		}
		m.appendRenderedLine(
			m.skin.Muted.Render(m.skin.ToolChar+" "+msg.Call.ToolUseName+": ") +
				m.skin.Code.Render(string(msg.Call.ToolUseInput)),
		)

	case toolResultMsg:
		m.state = StateStreaming
		m.streamingTool = nil
		// Render tool result snippet
		lines := renderToolResult(msg.Result, m.skin)
		for _, l := range lines {
			m.appendRenderedLine(l)
		}

	case convDoneMsg:
		if msg.Result != nil {
			m.history = msg.Result.Messages
			m.totalUsage.InputTokens += msg.Result.Usage.InputTokens
			m.totalUsage.OutputTokens += msg.Result.Usage.OutputTokens
			m.turnCount++
		}
		// Finalize the streaming text as a rendered assistant message
		if m.streamingText.Len() > 0 {
			rendered := renderAssistantText(m.streamingText.String(), m.skin)
			m.replaceLastStreamingLine(rendered)
			m.streamingText.Reset()
		}
		m.state = StateIdle

	case convErrorMsg:
		m.state = StateIdle
		m.streamingText.Reset()
		if msg.Err != nil {
			m.appendRenderedLine(m.skin.Error.Render(m.skin.ErrorChar + " error: " + msg.Err.Error()))
		}
	}

	// Delegate input + viewport updates
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// quit transitions the model to exit state.
func (m Model) quit() (tea.Model, tea.Cmd) {
	m.quitting = true
	return m, tea.Quit
}

// startConversation kicks off an Engine goroutine via the dispatcher
// installed by Run(). The goroutine posts tea.Msg values back to the
// running Program.
func (m Model) startConversation(text string) tea.Cmd {
	if m.dispatch == nil {
		return func() tea.Msg {
			return convErrorMsg{Err: fmt.Errorf("ui: no dispatcher installed")}
		}
	}
	// Fire and forget — the dispatcher spawns its own goroutine.
	m.dispatch(text, m.history)
	return nil
}

// refreshStreamingLine replaces the last "thinking" line with the current streaming text.
func (m *Model) refreshStreamingLine() {
	if len(m.rendered) == 0 {
		return
	}
	m.rendered[len(m.rendered)-1] = m.streamingText.String() + m.skin.Muted.Render(m.skin.StreamingChar)
	m.viewport.SetContent(joinRendered(m.rendered))
	m.viewport.GotoBottom()
}

// replaceLastStreamingLine replaces the last line with a final rendered text.
func (m *Model) replaceLastStreamingLine(text string) {
	if len(m.rendered) == 0 {
		m.appendRenderedLine(text)
		return
	}
	m.rendered[len(m.rendered)-1] = text
	m.viewport.SetContent(joinRendered(m.rendered))
	m.viewport.GotoBottom()
}

// joinRendered joins rendered lines with newlines.
func joinRendered(lines []string) string {
	total := 0
	for _, l := range lines {
		total += len(l) + 1
	}
	out := make([]byte, 0, total)
	for i, l := range lines {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, l...)
	}
	return string(out)
}

// Silence unused imports
var (
	_ = context.Background
	_ = message.RoleUser
)
