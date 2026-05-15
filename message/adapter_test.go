package message

import (
	"encoding/json"
	"testing"

	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextContentRoundTrip(t *testing.T) {
	original := core.NewTextMessage(core.MESSAGE_ROLE_USER, "hello world")

	pantheon := ToPantheon(original)
	assert.Equal(t, core.MESSAGE_ROLE_USER, pantheon.Role)
	require.Len(t, pantheon.Content, 1)
	assert.Equal(t, core.TextPart{Text: "hello world"}, pantheon.Content[0])

	back := MessageFromPantheon(pantheon)
	assert.Equal(t, core.MESSAGE_ROLE_USER, back.Role)
	assert.Equal(t, "hello world", back.Text())
}

func TestToolUseConversion(t *testing.T) {
	original := HermindMessage{
		Role: core.MESSAGE_ROLE_ASSISTANT,
		Content: []core.ContentParter{core.ToolCallPart{
			ID:        "call_123",
			Name:      "calculator",
			Arguments: `{"expr":"1+1"}`,
		}},
	}

	pantheon := ToPantheon(original)
	require.Len(t, pantheon.Content, 1)
	part, ok := pantheon.Content[0].(core.ToolCallPart)
	require.True(t, ok)
	assert.Equal(t, "call_123", part.ID)
	assert.Equal(t, "calculator", part.Name)
	assert.Equal(t, `{"expr":"1+1"}`, part.Arguments)
}

func TestToolResultConversion(t *testing.T) {
	pantheon := core.Message{
		Role: core.MESSAGE_ROLE_TOOL,
		Content: []core.ContentParter{
			core.ToolResultPart{
				ToolCallID: "call_123",
				Name:       "calculator",
				Content: []core.ContentParter{
					core.TextPart{Text: "The result is 2."},
				},
				IsError: false,
			},
		},
	}

	hermind := MessageFromPantheon(pantheon)
	calls := hermind.ExtractToolCalls()
	assert.Len(t, calls, 0) // tool results are not tool calls
	assert.Equal(t, "The result is 2.", hermind.Text())
}

func TestReasoningExtraction(t *testing.T) {
	pantheon := core.Message{
		Role: core.MESSAGE_ROLE_ASSISTANT,
		Content: []core.ContentParter{
			core.TextPart{Text: "The answer is 42."},
			core.ReasoningPart{Text: "Let me think step by step..."},
		},
	}

	hermind := MessageFromPantheon(pantheon)
	assert.Equal(t, core.MESSAGE_ROLE_ASSISTANT, hermind.Role)
	assert.Equal(t, "Let me think step by step...", hermind.ExtractReasoning())
	assert.Equal(t, "The answer is 42.", hermind.Text())
}

func TestImageURLConversion(t *testing.T) {
	original := HermindMessage{
		Role: core.MESSAGE_ROLE_USER,
		Content: []core.ContentParter{
			core.ImagePart{URL: "https://example.com/img.png", Detail: "high"},
		},
	}

	pantheon := ToPantheon(original)
	require.Len(t, pantheon.Content, 1)
	part, ok := pantheon.Content[0].(core.ImagePart)
	require.True(t, ok)
	assert.Equal(t, "https://example.com/img.png", part.URL)
	assert.Equal(t, "high", part.Detail)
}

func TestMessageJSONRoundTripWithToolCallID(t *testing.T) {
	msg := HermindMessage{
		Role:       core.MESSAGE_ROLE_TOOL,
		Content:    core.NewTextContent("result"),
		ToolCallID: "call_1",
		Name:       "search",
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded HermindMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, core.MESSAGE_ROLE_TOOL, decoded.Role)
	assert.Equal(t, "call_1", decoded.ToolCallID)
	assert.Equal(t, "search", decoded.Name)
}
