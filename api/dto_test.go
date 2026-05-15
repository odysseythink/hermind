package api

import (
	"encoding/json"
	"strings"
	"testing"

	_ "github.com/odysseythink/hermind/logging"
)

func TestStatusResponse_JSONShape(t *testing.T) {
	resp := StatusResponse{
		Version: "v1", UptimeSec: 10, StorageDriver: "sqlite",
		InstanceRoot: "/tmp/i", CurrentModel: "anthropic/claude-opus-4-6",
	}
	data, _ := json.Marshal(resp)
	want := `{"version":"v1","uptime_sec":10,"storage_driver":"sqlite","instance_root":"/tmp/i","current_model":"anthropic/claude-opus-4-6"}`
	if string(data) != want {
		t.Errorf("got %s\nwant %s", data, want)
	}
}

func TestConversationHistoryResponse_JSONShape(t *testing.T) {
	resp := ConversationHistoryResponse{
		Messages: []StoredMessageDTO{
			{ID: 1, Role: "user", Content: "hi"},
			{ID: 2, Role: "assistant", Content: "hello"},
		},
	}
	data, _ := json.Marshal(resp)
	for _, key := range []string{`"id":1`, `"role":"user"`, `"content":"hi"`, `"role":"assistant"`} {
		if !strings.Contains(string(data), key) {
			t.Errorf("missing %s in %s", key, data)
		}
	}
}
