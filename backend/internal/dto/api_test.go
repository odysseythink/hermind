package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRawTextRequest_UnmarshalsNodeShape(t *testing.T) {
	raw := []byte(`{
        "textContent": "Hello world",
        "title": "greeting",
        "addToWorkspaces": "w1,w2",
        "metadata": {"title": "greeting", "docSource": "test"}
    }`)
	var got APIRawTextRequest
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "Hello world", got.Text)
	assert.Equal(t, "greeting", got.Title)
	assert.Equal(t, "w1,w2", got.AddToWorkspaces)
	assert.NotNil(t, got.Metadata)
}

func TestAPIUpdatePinRequest_NodeShape(t *testing.T) {
	raw := []byte(`{"docPath":"custom-documents/a.json","pinStatus":true}`)
	var got APIUpdatePinRequest
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "custom-documents/a.json", got.DocPath)
	assert.True(t, got.PinValue)
}

func TestAPIDocumentRemoveFolderRequest_NodeShape(t *testing.T) {
	raw := []byte(`{"name":"my-folder"}`)
	var got APIDocumentRemoveFolderRequest
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "my-folder", got.Name)
}

func TestAPISystemRemoveDocumentsRequest_NodeShape(t *testing.T) {
	raw := []byte(`{"names":["custom-documents/a.json","custom-documents/b.json"]}`)
	var got APISystemRemoveDocumentsRequest
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, []string{"custom-documents/a.json", "custom-documents/b.json"}, got.Names)
}

func TestOpenAIChatRequest_NodeShape(t *testing.T) {
	raw := []byte(`{
        "model": "my-workspace",
        "messages": [
            {"role":"system","content":"Be helpful."},
            {"role":"user","content":"Hi"}
        ],
        "temperature": 0.4,
        "stream": true
    }`)
	var got OpenAIChatRequest
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "my-workspace", got.Model)
	require.Len(t, got.Messages, 2)
	assert.Equal(t, "system", got.Messages[0].Role)
	require.NotNil(t, got.Temperature)
	assert.InDelta(t, 0.4, *got.Temperature, 0.001)
	assert.True(t, got.Stream)
}

func TestOpenAIEmbeddingRequest_NodeShape_StringInput(t *testing.T) {
	raw := []byte(`{"model":"text-embedding-ada-002","input":"hello"}`)
	var got OpenAIEmbeddingRequest
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "text-embedding-ada-002", got.Model)
}

func TestAPIDocumentUploadRequest_NodeShape(t *testing.T) {
	raw := []byte(`{"addToWorkspaces":"w1,w2","metadata":{"title":"x"}}`)
	var got APIDocumentUploadRequest
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "w1,w2", got.AddToWorkspaces)
}

func TestChatRequest_AcceptsOverrides(t *testing.T) {
	raw := []byte(`{
        "message": "Hi",
        "mode": "chat",
        "systemPromptOverride": "You are a stoic.",
        "temperatureOverride": 0.2
    }`)
	var got ChatRequest
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "Hi", got.Message)
	require.NotNil(t, got.SystemPromptOverride)
	assert.Equal(t, "You are a stoic.", *got.SystemPromptOverride)
	require.NotNil(t, got.TemperatureOverride)
	assert.InDelta(t, 0.2, *got.TemperatureOverride, 0.001)
}

func TestChatRequest_OmitsOverridesWhenNil(t *testing.T) {
	req := ChatRequest{Message: "Hi"}
	out, err := json.Marshal(req)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "systemPromptOverride")
	assert.NotContains(t, string(out), "temperatureOverride")
}

func TestStreamChatRequest_AcceptsOverrides(t *testing.T) {
	raw := []byte(`{"message":"Hi","systemPromptOverride":"X"}`)
	var got StreamChatRequest
	require.NoError(t, json.Unmarshal(raw, &got))
	require.NotNil(t, got.SystemPromptOverride)
	assert.Equal(t, "X", *got.SystemPromptOverride)
}
