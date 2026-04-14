// cli/ui/ui_test.go
package ui

import (
	"encoding/json"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestModel() Model {
	return NewModel(ModelConfig{
		Config:    &config.Config{},
		AgentCfg:  config.AgentConfig{MaxTurns: 10},
		Skin:      MinimalSkin(),
		SessionID: "test-session",
		Model:     "test-model",
	})
}

func TestSlashExitQuits(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/exit")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	assert.True(t, m2.quitting)
	assert.NotNil(t, cmd)
}

func TestSlashClearResetsHistory(t *testing.T) {
	m := newTestModel()
	m.history = []message.Message{{Role: message.RoleUser, Content: message.TextContent("old")}}
	m.rendered = []string{"line1", "line2"}
	m.input.SetValue("/clear")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	assert.Empty(t, m2.history)
	// After clearing, we append a "conversation cleared" line.
	require.NotEmpty(t, m2.rendered)
	assert.Contains(t, m2.rendered[len(m2.rendered)-1], "cleared")
}

func TestSlashHelpShowsCommands(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/help")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	joined := ""
	for _, l := range m2.rendered {
		joined += l + "\n"
	}
	assert.Contains(t, joined, "/exit")
	assert.Contains(t, joined, "/clear")
	assert.Contains(t, joined, "/help")
}

func TestStreamDeltaAppendsToStreaming(t *testing.T) {
	m := newTestModel()
	m.state = StateStreaming
	m.appendRenderedLine("thinking")

	updated, _ := m.Update(streamDeltaMsg{Delta: &provider.StreamDelta{Content: "Hello "}})
	m = updated.(Model)
	updated, _ = m.Update(streamDeltaMsg{Delta: &provider.StreamDelta{Content: "world"}})
	m = updated.(Model)

	assert.Equal(t, "Hello world", m.streamingText.String())
}

func TestConvDoneResetsState(t *testing.T) {
	m := newTestModel()
	m.state = StateStreaming
	m.streamingText.WriteString("some partial")
	m.appendRenderedLine("streaming line")

	result := &agent.ConversationResult{
		Response: message.Message{Role: message.RoleAssistant, Content: message.TextContent("done")},
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("hi")},
			{Role: message.RoleAssistant, Content: message.TextContent("done")},
		},
		Usage: message.Usage{InputTokens: 10, OutputTokens: 5},
	}
	updated, _ := m.Update(convDoneMsg{Result: result})
	m = updated.(Model)

	assert.Equal(t, StateIdle, m.state)
	assert.Equal(t, 10, m.totalUsage.InputTokens)
	assert.Equal(t, 5, m.totalUsage.OutputTokens)
	assert.Equal(t, 1, m.turnCount)
	assert.Empty(t, m.streamingText.String())
	assert.Len(t, m.history, 2)
}

func TestConvErrorShowsError(t *testing.T) {
	m := newTestModel()
	m.state = StateStreaming
	updated, _ := m.Update(convErrorMsg{Err: assertError{msg: "boom"}})
	m = updated.(Model)
	assert.Equal(t, StateIdle, m.state)
	joined := ""
	for _, l := range m.rendered {
		joined += l
	}
	assert.Contains(t, joined, "boom")
}

func TestToolStartIncrementsCounter(t *testing.T) {
	m := newTestModel()
	m.state = StateStreaming
	call := message.ContentBlock{
		Type:         "tool_use",
		ToolUseID:    "t1",
		ToolUseName:  "read_file",
		ToolUseInput: json.RawMessage(`{"path":"x"}`),
	}
	updated, _ := m.Update(toolStartMsg{Call: call})
	m = updated.(Model)
	assert.Equal(t, StateToolRunning, m.state)
	assert.Equal(t, 1, m.toolCalls)
}

func TestWindowSizeUpdatesDimensions(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	assert.Equal(t, 120, m.width)
	assert.Equal(t, 40, m.height)
}

// assertError is a minimal error type for tests.
type assertError struct{ msg string }

func (e assertError) Error() string { return e.msg }
