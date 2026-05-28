package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/require"
)

func TestGCalAgent_CheckFn_FalseWithoutUser(t *testing.T) {
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gcalConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{
		Cfg:    &config.Config{MultiUserMode: true},
		Bridge: bc,
	}
	entry := NewGCalAgentSkill(tc, deps)
	require.False(t, entry.CheckFn())
}

func TestGCalAgent_CheckFn_TrueInMultiUserModeWithUser(t *testing.T) {
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		User:      &models.User{ID: 1},
		Settings:  map[string]string{gcalConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{
		Cfg:    &config.Config{MultiUserMode: true},
		Bridge: bc,
	}
	entry := NewGCalAgentSkill(tc, deps)
	require.True(t, entry.CheckFn())
}

func TestGCalAgent_CheckFn_FalseWhenConfigMissing(t *testing.T) {
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
	entry := NewGCalAgentSkill(tc, deps)
	require.False(t, entry.CheckFn())
}

func TestGCalAgent_CheckFn_TrueWhenConfigured(t *testing.T) {
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		User:      &models.User{ID: 1},
		Settings:  map[string]string{gcalConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{
		Cfg:    &config.Config{},
		Bridge: bc,
	}
	entry := NewGCalAgentSkill(tc, deps)
	require.True(t, entry.CheckFn())
}

func TestGCalAgent_ListCalendars_ForwardsToBridge(t *testing.T) {
	var captured map[string]any
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"data":   []map[string]string{{"id": "cal1", "name": "My Calendar"}},
		})
	}))

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gcalConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{Bridge: bc}
	entry := NewGCalAgentSkill(tc, deps)

	result, err := entry.Handler(context.Background(), json.RawMessage(`{"action":"list_calendars"}`))
	require.NoError(t, err)
	require.Contains(t, result, "My Calendar")

	require.Equal(t, "list_calendars", captured["action"])
	require.NotContains(t, captured, "params")
}

func TestGCalAgent_CreateEvent_TriggersApproval(t *testing.T) {
	var approvalCalled bool
	approval := func(ctx context.Context, skillName string, args any, description string) (bool, string) {
		approvalCalled = true
		require.Equal(t, "google-calendar-agent:create_event", skillName)
		return true, ""
	}

	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": map[string]any{"id": "evt1"}})
	}))

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gcalConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
		Approval:  approval,
	}
	deps := BuilderDeps{Bridge: bc}
	entry := NewGCalAgentSkill(tc, deps)

	_, err := entry.Handler(context.Background(), json.RawMessage(`{"action":"create_event","calendar_id":"cal1","title":"Meeting","start_time":"2024-01-01T10:00:00Z","end_time":"2024-01-01T11:00:00Z"}`))
	require.NoError(t, err)
	require.True(t, approvalCalled)
}

func TestGCalAgent_CreateEvent_RejectedByUser_ReturnsToolError(t *testing.T) {
	approval := func(ctx context.Context, skillName string, args any, description string) (bool, string) {
		return false, "denied"
	}

	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("bridge should not be called when approval rejects")
	}))

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gcalConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
		Approval:  approval,
	}
	deps := BuilderDeps{Bridge: bc}
	entry := NewGCalAgentSkill(tc, deps)

	result, err := entry.Handler(context.Background(), json.RawMessage(`{"action":"create_event","calendar_id":"cal1","title":"Meeting","start_time":"2024-01-01T10:00:00Z","end_time":"2024-01-01T11:00:00Z"}`))
	require.NoError(t, err)
	require.Contains(t, result, "rejected")
	require.Contains(t, result, "denied")
}

func TestGCalAgent_ReadOnlyActions_BypassApproval(t *testing.T) {
	var approvalCalled bool
	approval := func(ctx context.Context, skillName string, args any, description string) (bool, string) {
		approvalCalled = true
		return true, ""
	}

	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": nil})
	}))

	actions := []string{"list_calendars", "get_calendar", "get_event", "get_events_for_day", "get_events"}
	for _, action := range actions {
		approvalCalled = false
		tc := &ToolContext{
			Ctx:       context.Background(),
			Workspace: &models.Workspace{ID: 1},
			Settings:  map[string]string{gcalConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
			Emit:      func(string) {},
			Approval:  approval,
		}
		deps := BuilderDeps{Bridge: bc}
		entry := NewGCalAgentSkill(tc, deps)

		var args json.RawMessage
		switch action {
		case "list_calendars":
			args = json.RawMessage(`{"action":"list_calendars"}`)
		case "get_calendar":
			args = json.RawMessage(`{"action":"get_calendar","calendar_id":"cal1"}`)
		case "get_event":
			args = json.RawMessage(`{"action":"get_event","calendar_id":"cal1","event_id":"evt1"}`)
		case "get_events_for_day":
			args = json.RawMessage(`{"action":"get_events_for_day","calendar_id":"cal1","date":"2024-01-01"}`)
		case "get_events":
			args = json.RawMessage(`{"action":"get_events","calendar_id":"cal1","start_time":"2024-01-01T00:00:00Z","end_time":"2024-01-02T00:00:00Z"}`)
		}

		_, err := entry.Handler(context.Background(), args)
		require.NoError(t, err, "action %s", action)
		require.False(t, approvalCalled, "action %s should not trigger approval", action)
	}
}

func TestGCalAgent_QuickAdd_TriggersApproval(t *testing.T) {
	var approvalCalled bool
	approval := func(ctx context.Context, skillName string, args any, description string) (bool, string) {
		approvalCalled = true
		require.Equal(t, "google-calendar-agent:quick_add", skillName)
		return true, ""
	}

	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": map[string]any{"id": "evt1"}})
	}))

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gcalConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
		Approval:  approval,
	}
	deps := BuilderDeps{Bridge: bc}
	entry := NewGCalAgentSkill(tc, deps)

	_, err := entry.Handler(context.Background(), json.RawMessage(`{"action":"quick_add","text":"Lunch with Bob tomorrow at noon"}`))
	require.NoError(t, err)
	require.True(t, approvalCalled)
}

func TestGCalAgent_DispatchViaRegistry(t *testing.T) {
	bc, _ := newGmailTestBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"data":   []map[string]string{{"id": "cal1", "name": "Work"}},
		})
	}))

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Settings:  map[string]string{gcalConfigKey: `{"deploymentId":"dep1","apiKey":"key1"}`},
		Emit:      func(string) {},
	}
	deps := BuilderDeps{Bridge: bc}
	entry := NewGCalAgentSkill(tc, deps)

	reg := tool.NewRegistry()
	reg.Register(entry)

	result, err := reg.Dispatch(context.Background(), "google-calendar-agent", json.RawMessage(`{"action":"list_calendars"}`))
	require.NoError(t, err)
	require.Contains(t, result, "Work")
}
