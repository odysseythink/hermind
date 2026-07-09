package agent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerFrame_MarshalContentString(t *testing.T) {
	f := ServerFrame{From: "@agent", Content: "hello"}
	raw, err := json.Marshal(f)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	require.Equal(t, "@agent", m["from"])
	require.Equal(t, "hello", m["content"])
	// ContentObj not set → "content" is the string
	_, isString := m["content"].(string)
	require.True(t, isString)
}

func TestServerFrame_MarshalContentObj(t *testing.T) {
	f := ServerFrame{
		Type: FrameReportStreamEvent,
		ContentObj: map[string]any{
			"type":      "citations",
			"uuid":      "abc",
			"citations": []any{},
		},
	}
	raw, err := json.Marshal(f)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	require.Equal(t, FrameReportStreamEvent, m["type"])
	content, ok := m["content"].(map[string]any)
	require.True(t, ok, "content should be object not string")
	require.Equal(t, "citations", content["type"])
	require.Equal(t, "abc", content["uuid"])
}

func TestServerFrame_ContentObjWinsOverContent(t *testing.T) {
	f := ServerFrame{
		Content:    "should-not-appear",
		ContentObj: map[string]any{"key": "value"},
	}
	raw, err := json.Marshal(f)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"content":{"key":"value"}`)
	require.NotContains(t, string(raw), `"should-not-appear"`)
}

func TestServerFrame_UUIDField(t *testing.T) {
	f := ServerFrame{UUID: "test-uuid", Content: "x"}
	raw, err := json.Marshal(f)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"uuid":"test-uuid"`)
}

func TestServerFrame_OmitEmptyFields(t *testing.T) {
	f := ServerFrame{Content: "only"}
	raw, err := json.Marshal(f)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"uuid"`, "empty UUID should be omitted")
	require.NotContains(t, string(raw), `"type"`, "empty Type should be omitted")
}
