package message

import (
	"encoding/json"
	"testing"

	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleConstants(t *testing.T) {
	assert.Equal(t, core.MessageRoleType("user"), core.MESSAGE_ROLE_USER)
	assert.Equal(t, core.MessageRoleType("assistant"), core.MESSAGE_ROLE_ASSISTANT)
	assert.Equal(t, core.MessageRoleType("tool"), core.MESSAGE_ROLE_TOOL)
	assert.Equal(t, core.MessageRoleType("system"), core.MESSAGE_ROLE_SYSTEM)
}

func TestMessageJSONRoundtripText(t *testing.T) {
	msg := core.NewTextMessage(core.MESSAGE_ROLE_USER, "hello world")
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded HermindMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, core.MESSAGE_ROLE_USER, decoded.Role)
	assert.Equal(t, "hello world", decoded.Text())
}

func TestUsageZeroValue(t *testing.T) {
	var u core.Usage
	assert.Equal(t, 0, u.PromptTokens)
	assert.Equal(t, 0, u.CompletionTokens)
}

func TestTextContentEmpty(t *testing.T) {
	c := core.NewTextContent("")
	assert.Nil(t, c)
}

func TestNewTextMessage(t *testing.T) {
	m := core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, "hi")
	assert.Equal(t, core.MESSAGE_ROLE_ASSISTANT, m.Role)
	assert.Equal(t, "hi", m.Text())
}

func TestToolResultContent(t *testing.T) {
	m := core.NewToolResultContent("call_1", "search", "results")
	assert.Equal(t, core.MESSAGE_ROLE_TOOL, m.Role)
	assert.Equal(t, "results", m.Text())
	assert.Equal(t, "call_1", m.Content[0].(core.ToolResultPart).ToolCallID)
}

func TestExtractToolCalls(t *testing.T) {
	m := HermindMessage{
		Role: core.MESSAGE_ROLE_ASSISTANT,
		Content: []core.ContentParter{
			core.TextPart{Text: "I will search"},
			core.ToolCallPart{ID: "c1", Name: "search", Arguments: `{}`},
		},
	}
	calls := m.ExtractToolCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "c1", calls[0].ID)
}

func TestExtractReasoning(t *testing.T) {
	m := HermindMessage{
		Role: core.MESSAGE_ROLE_ASSISTANT,
		Content: []core.ContentParter{
			core.TextPart{Text: "answer"},
			core.ReasoningPart{Text: "step 1"},
			core.ReasoningPart{Text: "step 2"},
		},
	}
	assert.Equal(t, "step 1\nstep 2", m.ExtractReasoning())
}

func TestHasToolCalls(t *testing.T) {
	withTool := HermindMessage{Role: core.MESSAGE_ROLE_ASSISTANT, Content: []core.ContentParter{core.ToolCallPart{ID: "c1", Name: "x", Arguments: `{}`}}}
	without := HermindMessage{Role: core.MESSAGE_ROLE_ASSISTANT, Content: []core.ContentParter{core.TextPart{Text: "hi"}}}
	assert.True(t, withTool.HasToolCalls())
	assert.False(t, without.HasToolCalls())
}

func TestIsTextOnly(t *testing.T) {
	textOnly := HermindMessage{Content: []core.ContentParter{core.TextPart{Text: "hi"}}}
	mixed := HermindMessage{Content: []core.ContentParter{core.TextPart{Text: "hi"}, core.ToolCallPart{ID: "c1", Name: "x", Arguments: `{}`}}}
	assert.True(t, textOnly.IsTextOnly())
	assert.False(t, mixed.IsTextOnly())
}
