package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSessionListResponse_JSONShape(t *testing.T) {
	resp := SessionListResponse{
		Total: 1,
		Sessions: []SessionDTO{{
			ID: "s1", Source: "cli", Model: "m", MessageCount: 3,
		}},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"sessions":[{"id":"s1","source":"cli","model":"m","system_prompt":"","started_at":0,"ended_at":0,"message_count":3,"title":""}],"total":1}`
	if string(data) != want {
		t.Errorf("got %s\nwant %s", data, want)
	}
}

func TestMessagesResponse_JSONShape(t *testing.T) {
	resp := MessagesResponse{
		Total: 2,
		Messages: []MessageDTO{
			{ID: 1, Role: "user", Content: "hi"},
			{ID: 2, Role: "assistant", Content: "hello"},
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"id":1`, `"role":"user"`, `"content":"hi"`, `"total":2`} {
		if !strings.Contains(string(data), key) {
			t.Errorf("missing %s in %s", key, data)
		}
	}
}

func TestStatusResponse_JSONShape(t *testing.T) {
	resp := StatusResponse{Version: "v1", UptimeSec: 10, StorageDriver: "sqlite", InstanceRoot: "/tmp/i"}
	data, _ := json.Marshal(resp)
	want := `{"version":"v1","uptime_sec":10,"storage_driver":"sqlite","instance_root":"/tmp/i"}`
	if string(data) != want {
		t.Errorf("got %s\nwant %s", data, want)
	}
}
