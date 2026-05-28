package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/odysseythink/hermind/backend/internal/collector"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/require"
)

func newGmailTestBridge(t *testing.T, handler http.HandlerFunc) (*oauth.BridgeClient, *httptest.Server) {
	srv := httptest.NewServer(handler)
	oauth.SetTestBaseURL(srv.URL)
	t.Cleanup(func() { oauth.SetTestBaseURL(""); srv.Close() })
	return oauth.NewBridgeClient(5 * time.Second), srv
}

func TestGmailAgent_CheckFn_FalseWithoutUser(t *testing.T) {
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gmailConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{
		Cfg:    &config.Config{MultiUserMode: true},
		Bridge: bc,
	}
	entry := NewGmailAgentSkill(tc, deps)
	require.False(t, entry.CheckFn())
}

func TestGmailAgent_CheckFn_TrueInMultiUserModeWithUser(t *testing.T) {
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		User:      &models.User{ID: 1},
		Settings:  map[string]string{gmailConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{
		Cfg:    &config.Config{MultiUserMode: true},
		Bridge: bc,
	}
	entry := NewGmailAgentSkill(tc, deps)
	require.True(t, entry.CheckFn())
}

func TestGmailAgent_CheckFn_FalseWhenConfigMissing(t *testing.T) {
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{
		Cfg:    &config.Config{},
		Bridge: bc,
	}
	entry := NewGmailAgentSkill(tc, deps)
	require.False(t, entry.CheckFn())
}

func TestGmailAgent_CheckFn_TrueWhenConfigured(t *testing.T) {
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		User:      &models.User{ID: 1},
		Settings:  map[string]string{gmailConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{
		Cfg:    &config.Config{},
		Bridge: bc,
	}
	entry := NewGmailAgentSkill(tc, deps)
	require.True(t, entry.CheckFn())
}

func TestGmailAgent_Search_ForwardsToBridge(t *testing.T) {
	var captured map[string]any
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"data":   map[string]any{"results": []string{"msg1"}},
		})
	}))

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gmailConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{Bridge: bc}
	entry := NewGmailAgentSkill(tc, deps)

	result, err := entry.Handler(context.Background(), json.RawMessage(`{"action":"search","query":"hello"}`))
	require.NoError(t, err)
	require.Contains(t, result, "msg1")

	require.Equal(t, "search", captured["action"])
	require.Equal(t, "hello", captured["query"])
	require.NotContains(t, captured, "params")
}

func TestGmailAgent_SendEmail_TriggersApproval(t *testing.T) {
	var approvalCalled bool
	approval := func(ctx context.Context, skillName string, args any, description string) (bool, string) {
		approvalCalled = true
		require.Equal(t, "gmail-agent:send_email", skillName)
		return true, ""
	}

	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": map[string]any{"sent": true}})
	}))

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gmailConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
		Approval:  approval,
	}
	deps := BuilderDeps{Bridge: bc}
	entry := NewGmailAgentSkill(tc, deps)

	_, err := entry.Handler(context.Background(), json.RawMessage(`{"action":"send_email","to":"a@b.com","subject":"hi","body":"hello"}`))
	require.NoError(t, err)
	require.True(t, approvalCalled)
}

func TestGmailAgent_SendEmail_RejectedByUser_ReturnsToolError(t *testing.T) {
	approval := func(ctx context.Context, skillName string, args any, description string) (bool, string) {
		return false, "user said no"
	}

	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("bridge should not be called when approval rejects")
	}))

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gmailConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
		Approval:  approval,
	}
	deps := BuilderDeps{Bridge: bc}
	entry := NewGmailAgentSkill(tc, deps)

	result, err := entry.Handler(context.Background(), json.RawMessage(`{"action":"send_email","to":"a@b.com","subject":"hi","body":"hello"}`))
	require.NoError(t, err)
	require.Contains(t, result, "rejected")
	require.Contains(t, result, "user said no")
}

func TestGmailAgent_ReadOnlyActions_BypassApproval(t *testing.T) {
	var approvalCalled bool
	approval := func(ctx context.Context, skillName string, args any, description string) (bool, string) {
		approvalCalled = true
		return true, ""
	}

	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": nil})
	}))

	actions := []string{"search", "read_thread", "list_drafts", "get_draft", "mailbox_stats"}
	for _, action := range actions {
		approvalCalled = false
		tc := &ToolContext{
			Ctx:       context.Background(),
			Workspace: &models.Workspace{ID: 1},
			Settings:  map[string]string{gmailConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
			Emit:      func(string) {},
			Approval:  approval,
		}
		deps := BuilderDeps{Bridge: bc}
		entry := NewGmailAgentSkill(tc, deps)

		var args json.RawMessage
		switch action {
		case "search":
			args = json.RawMessage(`{"action":"search","query":"x"}`)
		case "read_thread":
			args = json.RawMessage(`{"action":"read_thread","thread_id":"t1"}`)
		case "list_drafts":
			args = json.RawMessage(`{"action":"list_drafts"}`)
		case "get_draft":
			args = json.RawMessage(`{"action":"get_draft","draft_id":"d1"}`)
		case "mailbox_stats":
			args = json.RawMessage(`{"action":"mailbox_stats"}`)
		}

		_, err := entry.Handler(context.Background(), args)
		require.NoError(t, err, "action %s", action)
		require.False(t, approvalCalled, "action %s should not trigger approval", action)
	}
}

func TestGmailAgent_BridgeError_Surfaced(t *testing.T) {
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "error",
			"error":  "quota exceeded",
		})
	}))

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gmailConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{Bridge: bc}
	entry := NewGmailAgentSkill(tc, deps)

	result, err := entry.Handler(context.Background(), json.RawMessage(`{"action":"search","query":"hello"}`))
	require.NoError(t, err)
	require.Contains(t, result, "quota exceeded")
}

func TestGmailAgent_UnknownAction_ReturnsToolError(t *testing.T) {
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("bridge should not be called for empty action")
	}))

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gmailConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{Bridge: bc}
	entry := NewGmailAgentSkill(tc, deps)

	result, err := entry.Handler(context.Background(), json.RawMessage(`{"action":""}`))
	require.NoError(t, err)
	require.Contains(t, result, "action is required")
}

func TestGmailAgent_SendEmail_WithAttachment_InlinesText(t *testing.T) {
	var captured map[string]any
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": map[string]any{"sent": true}})
	}))

	coll, err := collector.NewLocalCollector(t.TempDir())
	require.NoError(t, err)
	defer coll.Close()

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		User:      &models.User{ID: 1},
		Settings:  map[string]string{gmailConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{Bridge: bc, Collector: coll}
	entry := NewGmailAgentSkill(tc, deps)

	attJSON := `[{"filename":"note.txt","data_base64":"` + base64.StdEncoding.EncodeToString([]byte("attachment content")) + `"}]`
	_, err = entry.Handler(context.Background(), json.RawMessage(`{"action":"send_email","to":"a@b.com","subject":"hi","body":"hello","attachments":`+attJSON+`}`))
	require.NoError(t, err)

	body, _ := captured["body"].(string)
	require.Contains(t, body, "hello")
	require.Contains(t, body, "attachment content")
}

func TestGmailAgent_DispatchViaRegistry(t *testing.T) {
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"data":   map[string]any{"results": []string{"thread1"}},
		})
	}))

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gmailConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{Bridge: bc}
	entry := NewGmailAgentSkill(tc, deps)

	reg := tool.NewRegistry()
	reg.Register(entry)

	result, err := reg.Dispatch(context.Background(), "gmail-agent", json.RawMessage(`{"action":"search","query":"test"}`))
	require.NoError(t, err)
	require.Contains(t, result, "thread1")
}
