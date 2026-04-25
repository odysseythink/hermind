package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/message"
)

func TestInbound_TextOnly(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet-4-6",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hi"}]}]
	}`)
	req, model, stream, err := Inbound(body)
	require.NoError(t, err)
	require.Equal(t, "claude-sonnet-4-6", model)
	require.False(t, stream)
	require.Equal(t, 1024, req.MaxTokens)
	require.Len(t, req.Messages, 1)
	require.Equal(t, message.RoleUser, req.Messages[0].Role)
	require.Equal(t, "hi", req.Messages[0].Content.Text())
}

func TestInbound_SystemAsString(t *testing.T) {
	body := []byte(`{
		"model": "x", "max_tokens": 64,
		"system": "you are concise",
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hi"}]}]
	}`)
	req, _, _, err := Inbound(body)
	require.NoError(t, err)
	require.Equal(t, "you are concise", req.SystemPrompt)
}

func TestInbound_SystemAsArrayRejected(t *testing.T) {
	body := []byte(`{
		"model": "x", "max_tokens": 64,
		"system": [{"type": "text", "text": "you are concise"}],
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hi"}]}]
	}`)
	_, _, _, err := Inbound(body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported_system_format")
}

func TestInbound_SystemRoleInMessagesRejected(t *testing.T) {
	body := []byte(`{
		"model": "x", "max_tokens": 64,
		"messages": [{"role": "system", "content": [{"type": "text", "text": "x"}]}]
	}`)
	_, _, _, err := Inbound(body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "system_role_in_messages")
}

func TestInbound_MultipleTextBlocks(t *testing.T) {
	body := []byte(`{
		"model": "x", "max_tokens": 64,
		"messages": [{"role": "user", "content": [
			{"type": "text", "text": "a"},
			{"type": "text", "text": "b"}
		]}]
	}`)
	req, _, _, err := Inbound(body)
	require.NoError(t, err)
	require.Len(t, req.Messages, 1)
	require.Equal(t, "a\nb", req.Messages[0].Content.Text())
}

func TestInbound_AssistantToolUse(t *testing.T) {
	body := []byte(`{
		"model": "x", "max_tokens": 64,
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "weather?"}]},
			{"role": "assistant", "content": [
				{"type": "text", "text": "checking"},
				{"type": "tool_use", "id": "toolu_1", "name": "get_weather", "input": {"city": "SF"}}
			]}
		]
	}`)
	req, _, _, err := Inbound(body)
	require.NoError(t, err)
	require.Len(t, req.Messages, 2)
	require.Equal(t, message.RoleAssistant, req.Messages[1].Role)
	require.Equal(t, "checking", req.Messages[1].Content.Text())
	require.Len(t, req.Messages[1].ToolCalls, 1)
	require.Equal(t, "toolu_1", req.Messages[1].ToolCalls[0].ID)
	require.Equal(t, "get_weather", req.Messages[1].ToolCalls[0].Function.Name)
	require.JSONEq(t, `{"city":"SF"}`, req.Messages[1].ToolCalls[0].Function.Arguments)
}

func TestInbound_UserMultipleToolResultsSplit(t *testing.T) {
	body := []byte(`{
		"model": "x", "max_tokens": 64,
		"messages": [
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "toolu_1", "content": "Sunny"},
				{"type": "tool_result", "tool_use_id": "toolu_2", "content": "Rainy"},
				{"type": "text", "text": "thanks"}
			]}
		]
	}`)
	req, _, _, err := Inbound(body)
	require.NoError(t, err)
	require.Len(t, req.Messages, 3, "two tool results then a user text remainder")
	require.Equal(t, message.RoleTool, req.Messages[0].Role)
	require.Equal(t, "toolu_1", req.Messages[0].ToolCallID)
	require.Equal(t, "Sunny", req.Messages[0].Content.Text())
	require.Equal(t, message.RoleTool, req.Messages[1].Role)
	require.Equal(t, "toolu_2", req.Messages[1].ToolCallID)
	require.Equal(t, message.RoleUser, req.Messages[2].Role)
	require.Equal(t, "thanks", req.Messages[2].Content.Text())
}

func TestInbound_EmptyMessagesRejected(t *testing.T) {
	body := []byte(`{"model":"x","max_tokens":64,"messages":[]}`)
	_, _, _, err := Inbound(body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid_request_error")
}

func TestInbound_MissingMaxTokensRejected(t *testing.T) {
	body := []byte(`{
		"model": "x",
		"messages": [{"role":"user","content":[{"type":"text","text":"hi"}]}]
	}`)
	_, _, _, err := Inbound(body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid_request_error")
}

func TestInbound_ToolsTranslated(t *testing.T) {
	body := []byte(`{
		"model": "x", "max_tokens": 64,
		"tools": [{"name":"get_weather","description":"...","input_schema":{"type":"object"}}],
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hi"}]}]
	}`)
	req, _, _, err := Inbound(body)
	require.NoError(t, err)
	require.Len(t, req.Tools, 1)
	require.Equal(t, "function", req.Tools[0].Type)
	require.Equal(t, "get_weather", req.Tools[0].Function.Name)
	require.JSONEq(t, `{"type":"object"}`, string(req.Tools[0].Function.Parameters))
}

func TestInbound_StreamFlag(t *testing.T) {
	body := []byte(`{
		"model": "x", "max_tokens": 64, "stream": true,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hi"}]}]
	}`)
	_, _, stream, err := Inbound(body)
	require.NoError(t, err)
	require.True(t, stream)
}

func TestInbound_BadJSON(t *testing.T) {
	_, _, _, err := Inbound([]byte("{not json"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid_request_error")
}

// rawJSON helper for assertions
var _ = json.RawMessage(nil)
