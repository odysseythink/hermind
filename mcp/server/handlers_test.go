package server

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
)

func newTestServer(t *testing.T) (*Server, storage.Storage) {
	t.Helper()
	dir := t.TempDir()
	store, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	bridge := NewEventBridge(store, 10*time.Millisecond)
	perms := NewPermissionQueue()
	s := NewServer(&ServerOpts{Storage: store, Events: bridge, Permissions: perms})
	return s, store
}

func seed(t *testing.T, store storage.Storage, _ string) {
	t.Helper()
	ctx := context.Background()
	if err := store.AppendMessage(ctx, &storage.StoredMessage{Role: "user", Content: "hi", Timestamp: time.Now()}); err != nil {
		t.Fatalf("append message: %v", err)
	}
}

func TestHandleInitialize(t *testing.T) {
	s, _ := newTestServer(t)
	raw, err := s.handleInitialize(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	_ = json.Unmarshal(raw, &resp)
	if resp["protocolVersion"] == nil {
		t.Errorf("missing protocolVersion: %v", resp)
	}
	caps, _ := resp["capabilities"].(map[string]any)
	if caps == nil || caps["tools"] == nil {
		t.Errorf("missing tools capability: %v", resp)
	}
}

func TestHandleToolsList(t *testing.T) {
	s, _ := newTestServer(t)
	raw, err := s.handleToolsList(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Tools []Tool `json:"tools"`
	}
	_ = json.Unmarshal(raw, &resp)
	if len(resp.Tools) != 10 {
		t.Errorf("tools = %d", len(resp.Tools))
	}
}

func TestHandleToolsCall_ConversationsList(t *testing.T) {
	s, store := newTestServer(t)
	seed(t, store, "a")
	seed(t, store, "b")

	params, _ := json.Marshal(map[string]any{
		"name":      "conversations_list",
		"arguments": map[string]any{"limit": 5},
	})
	raw, err := s.handleToolsCall(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	_ = json.Unmarshal(raw, &resp)
	if len(resp.Content) != 1 || resp.Content[0].Type != "text" {
		t.Fatalf("bad shape: %s", raw)
	}
	if resp.Content[0].Text == "" {
		t.Error("empty text")
	}
}

func TestHandleToolsCall_MessagesRead(t *testing.T) {
	s, store := newTestServer(t)
	seed(t, store, "s1")

	params, _ := json.Marshal(map[string]any{
		"name":      "messages_read",
		"arguments": map[string]any{"session_key": "s1", "limit": 10},
	})
	raw, _ := s.handleToolsCall(context.Background(), params)
	var resp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	_ = json.Unmarshal(raw, &resp)
	if resp.Content[0].Text == "" || !contains(resp.Content[0].Text, `"hi"`) {
		t.Errorf("unexpected text: %s", resp.Content[0].Text)
	}
}

func TestHandleToolsCall_UnknownTool(t *testing.T) {
	s, _ := newTestServer(t)
	params, _ := json.Marshal(map[string]any{
		"name":      "not_a_real_tool",
		"arguments": map[string]any{},
	})
	_, err := s.handleToolsCall(context.Background(), params)
	if err == nil {
		t.Fatal("expected error")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
