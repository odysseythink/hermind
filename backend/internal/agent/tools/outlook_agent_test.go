package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/odysseythink/hermind/backend/internal/collector"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newOutlookTestDB(t *testing.T) (*gorm.DB, *utils.EncryptionManager) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.OutlookOAuthToken{}))
	enc, err := utils.NewEncryptionManager(t.TempDir())
	require.NoError(t, err)
	return db, enc
}

func TestOutlookAgent_CheckFn_FalseInMultiUserMode_WithoutToken(t *testing.T) {
	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)

	tc := &ToolContext{
		Ctx:      context.Background(),
		Settings: map[string]string{outlookConfigKey: `{"clientId":"c","clientSecret":"s"}`},
		User:     &models.User{ID: 1},
	}
	deps := BuilderDeps{
		Cfg:          &config.Config{MultiUserMode: true},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
	}
	entry := NewOutlookAgentSkill(tc, deps)
	require.False(t, entry.CheckFn())
}

func TestOutlookAgent_MultiUserMode_RequiresPerUserToken(t *testing.T) {
	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	ctx := context.Background()

	tc := &ToolContext{
		Ctx:      ctx,
		Settings: map[string]string{outlookConfigKey: `{"clientId":"c","clientSecret":"s"}`},
		User:     &models.User{ID: 1},
	}
	deps := BuilderDeps{
		Cfg:          &config.Config{MultiUserMode: true},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
	}

	// No token yet → false
	entry := NewOutlookAgentSkill(tc, deps)
	require.False(t, entry.CheckFn())

	// Now issue token
	require.NoError(t, store.Save(ctx, 1, &oauth.TokenSet{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))
	entry2 := NewOutlookAgentSkill(tc, deps)
	require.True(t, entry2.CheckFn())
}

func TestOutlookAgent_CheckFn_FalseWithoutConfig(t *testing.T) {
	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)

	tc := &ToolContext{
		Ctx:      context.Background(),
		Settings: map[string]string{},
		User:     &models.User{ID: 1},
	}
	deps := BuilderDeps{
		Cfg:          &config.Config{MultiUserMode: false},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
	}
	entry := NewOutlookAgentSkill(tc, deps)
	require.False(t, entry.CheckFn())
}

func TestOutlookAgent_CheckFn_FalseWithoutToken(t *testing.T) {
	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)

	tc := &ToolContext{
		Ctx:      context.Background(),
		Settings: map[string]string{outlookConfigKey: `{"clientId":"c","clientSecret":"s"}`},
		User:     &models.User{ID: 1},
	}
	deps := BuilderDeps{
		Cfg:          &config.Config{MultiUserMode: false},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
	}
	entry := NewOutlookAgentSkill(tc, deps)
	require.False(t, entry.CheckFn())
}

func TestOutlookAgent_CheckFn_TrueWithConfigAndToken(t *testing.T) {
	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	require.NoError(t, store.Save(context.Background(), 1, &oauth.TokenSet{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))

	tc := &ToolContext{
		Ctx:      context.Background(),
		Settings: map[string]string{outlookConfigKey: `{"clientId":"c","clientSecret":"s"}`},
		User:     &models.User{ID: 1},
	}
	deps := BuilderDeps{
		Cfg:          &config.Config{MultiUserMode: false},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
	}
	entry := NewOutlookAgentSkill(tc, deps)
	require.True(t, entry.CheckFn())
}

func TestOutlookAgent_Search_CallsGraphAPI(t *testing.T) {
	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		require.Equal(t, "/me/messages", r.URL.Path)
		require.Equal(t, "Bearer valid-token", r.Header.Get("Authorization"))
		q := r.URL.Query().Get("$search")
		require.Equal(t, `"hello"`, q)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": []map[string]any{{"id": "msg1"}}})
	}))
	defer srv.Close()
	SetTestGraphBase(srv.URL)
	defer SetTestGraphBase("")

	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	require.NoError(t, store.Save(context.Background(), 1, &oauth.TokenSet{
		AccessToken:  "valid-token",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))

	tc := &ToolContext{
		Ctx:      context.Background(),
		Settings: map[string]string{outlookConfigKey: `{"clientId":"c","clientSecret":"s"}`},
		User:     &models.User{ID: 1},
		Emit:     func(string) {},
	}
	deps := BuilderDeps{
		Cfg:          &config.Config{MultiUserMode: false},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
	}
	entry := NewOutlookAgentSkill(tc, deps)
	result, err := entry.Handler(context.Background(), []byte(`{"action":"search","query":"hello"}`))
	require.NoError(t, err)
	require.True(t, called.Load())
	require.Contains(t, result, "msg1")
}

func TestOutlookAgent_SendEmail_TriggersApproval(t *testing.T) {
	var approved atomic.Bool
	approval := func(context.Context, string, any, string) (bool, string) {
		approved.Store(true)
		return true, ""
	}

	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	require.NoError(t, store.Save(context.Background(), 1, &oauth.TokenSet{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))

	tc := &ToolContext{
		Ctx:      context.Background(),
		Settings: map[string]string{outlookConfigKey: `{"clientId":"c","clientSecret":"s"}`},
		User:     &models.User{ID: 1},
		Emit:     func(string) {},
		Approval: approval,
	}
	deps := BuilderDeps{
		Cfg:          &config.Config{MultiUserMode: false},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
	}
	entry := NewOutlookAgentSkill(tc, deps)
	_, err := entry.Handler(context.Background(), []byte(`{"action":"send_email","to":"a@b.com","subject":"hi","body":"hello"}`))
	require.NoError(t, err)
	require.True(t, approved.Load())
}

func TestOutlookAgent_SendEmail_RejectedByUser_ReturnsToolError(t *testing.T) {
	approval := func(context.Context, string, any, string) (bool, string) {
		return false, "user said no"
	}

	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	require.NoError(t, store.Save(context.Background(), 1, &oauth.TokenSet{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))

	tc := &ToolContext{
		Ctx:      context.Background(),
		Settings: map[string]string{outlookConfigKey: `{"clientId":"c","clientSecret":"s"}`},
		User:     &models.User{ID: 1},
		Emit:     func(string) {},
		Approval: approval,
	}
	deps := BuilderDeps{
		Cfg:          &config.Config{MultiUserMode: false},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
	}
	entry := NewOutlookAgentSkill(tc, deps)
	result, err := entry.Handler(context.Background(), []byte(`{"action":"send_email","to":"a@b.com","subject":"hi","body":"hello"}`))
	require.NoError(t, err)
	require.Contains(t, result, "rejected")
	require.Contains(t, result, "user said no")
}

func TestOutlookAgent_SendEmail_WithAttachment_InlinesText(t *testing.T) {
	var called atomic.Bool
	var capturedBody string
	graphSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		if r.URL.Path == "/me/sendMail" && r.Method == "POST" {
			var reqBody map[string]any
			_ = json.NewDecoder(r.Body).Decode(&reqBody)
			if msg, ok := reqBody["message"].(map[string]any); ok {
				if b, ok := msg["body"].(map[string]any); ok {
					capturedBody, _ = b["content"].(string)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "sent"})
	}))
	defer graphSrv.Close()
	SetTestGraphBase(graphSrv.URL)
	defer SetTestGraphBase("")

	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	require.NoError(t, store.Save(context.Background(), 1, &oauth.TokenSet{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))

	coll, err := collector.NewLocalCollector(t.TempDir())
	require.NoError(t, err)
	defer coll.Close()

	tc := &ToolContext{
		Ctx:      context.Background(),
		Settings: map[string]string{outlookConfigKey: `{"clientId":"c","clientSecret":"s"}`},
		User:     &models.User{ID: 1},
		Emit:     func(string) {},
	}
	deps := BuilderDeps{
		Cfg:          &config.Config{MultiUserMode: false},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
		Collector:    coll,
	}
	entry := NewOutlookAgentSkill(tc, deps)

	attJSON := `[{"filename":"note.txt","data_base64":"` + base64.StdEncoding.EncodeToString([]byte("attachment text")) + `"}]`
	_, err = entry.Handler(context.Background(), []byte(`{"action":"send_email","to":"a@b.com","subject":"hi","body":"hello","attachments":`+attJSON+`}`))
	require.NoError(t, err)
	require.True(t, called.Load())
	require.Contains(t, capturedBody, "hello")
	require.Contains(t, capturedBody, "attachment text")
}

func TestOutlookAgent_TokenAutoRefreshOnExpiry(t *testing.T) {
	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	ctx := context.Background()
	require.NoError(t, store.Save(ctx, 1, &oauth.TokenSet{
		AccessToken:  "expired-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour),
		Tenant:       "common",
	}))

	// Mock Microsoft token endpoint
	msSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/common/oauth2/v2.0/token", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "refreshed-access",
			"refresh_token": "refreshed-refresh",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer msSrv.Close()
	oauth.SetTestMicrosoftBase(msSrv.URL)
	defer oauth.SetTestMicrosoftBase("")

	// Mock Graph API
	graphSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer refreshed-access", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": []map[string]any{{"id": "refreshed-msg"}}})
	}))
	defer graphSrv.Close()
	SetTestGraphBase(graphSrv.URL)
	defer SetTestGraphBase("")

	tc := &ToolContext{
		Ctx:      ctx,
		Settings: map[string]string{outlookConfigKey: `{"clientId":"c","clientSecret":"s"}`},
		User:     &models.User{ID: 1},
		Emit:     func(string) {},
	}
	deps := BuilderDeps{
		Cfg:          &config.Config{MultiUserMode: false},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
	}
	entry := NewOutlookAgentSkill(tc, deps)
	result, err := entry.Handler(ctx, []byte(`{"action":"search","query":"test"}`))
	require.NoError(t, err)
	require.Contains(t, result, "refreshed-msg")

	// Verify store was updated with refreshed token
	ts, err := store.Get(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "refreshed-access", ts.AccessToken)
}

func TestOutlookAgent_AllActions_RouteCorrectly(t *testing.T) {
	var lastMethod, lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		lastPath = r.URL.Path
		if r.Method == "PATCH" || r.Method == "POST" || r.Method == "DELETE" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "ok"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": []map[string]any{{"id": "msg1"}}})
	}))
	defer srv.Close()
	SetTestGraphBase(srv.URL)
	defer SetTestGraphBase("")

	cases := []struct {
		action       string
		args         string
		expectMethod string
		expectPath   string
	}{
		{"search", `{"query":"hello"}`, "GET", "/me/messages"},
		{"read_thread", `{"conversation_id":"cid1"}`, "GET", "/me/messages"},
		{"read_message", `{"message_id":"mid1"}`, "GET", "/me/messages/mid1"},
		{"create_draft", `{"to":"a@b.com","subject":"s","body":"b"}`, "POST", "/me/messages"},
		{"send_email", `{"to":"a@b.com","subject":"s","body":"b"}`, "POST", "/me/sendMail"},
		{"get_inbox", `{"limit":50}`, "GET", "/me/mailFolders/inbox/messages"},
		{"list_drafts", `{"limit":10}`, "GET", "/me/mailFolders/drafts/messages"},
		{"get_draft", `{"draft_id":"abc"}`, "GET", "/me/messages/abc"},
		{"update_draft", `{"draft_id":"abc","subject":"X"}`, "PATCH", "/me/messages/abc"},
		{"delete_draft", `{"draft_id":"abc"}`, "DELETE", "/me/messages/abc"},
		{"send_draft", `{"draft_id":"abc"}`, "POST", "/me/messages/abc/send"},
		{"create_draft_reply", `{"message_id":"abc","body":"hi"}`, "POST", "/me/messages/abc/createReply"},
		{"reply_to_message", `{"message_id":"abc","body":"hi"}`, "POST", "/me/messages/abc/reply"},
		{"mark_read", `{"message_id":"abc"}`, "PATCH", "/me/messages/abc"},
		{"mark_unread", `{"message_id":"abc"}`, "PATCH", "/me/messages/abc"},
	}

	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	require.NoError(t, store.Save(context.Background(), 1, &oauth.TokenSet{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			tcCtx := &ToolContext{
				Ctx:      context.Background(),
				Settings: map[string]string{outlookConfigKey: `{"clientId":"c","clientSecret":"s"}`},
				User:     &models.User{ID: 1},
				Emit:     func(string) {},
			}
			deps := BuilderDeps{
				Cfg:          &config.Config{},
				OutlookOAuth: outlookOAuth,
				OutlookStore: store,
			}
			entry := NewOutlookAgentSkill(tcCtx, deps)
			_, err := entry.Handler(context.Background(), []byte(`{"action":"`+tc.action+`",`+tc.args[1:]))
			require.NoError(t, err)
			require.Equal(t, tc.expectMethod, lastMethod, "method mismatch for %s", tc.action)
			require.Equal(t, tc.expectPath, lastPath, "path mismatch for %s", tc.action)
		})
	}
}

func TestOutlookAgent_DispatchViaRegistry(t *testing.T) {
	graphSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": []map[string]any{{"id": "reg-msg"}}})
	}))
	defer graphSrv.Close()
	SetTestGraphBase(graphSrv.URL)
	defer SetTestGraphBase("")

	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	require.NoError(t, store.Save(context.Background(), 1, &oauth.TokenSet{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))

	tc := &ToolContext{
		Ctx:      context.Background(),
		Settings: map[string]string{outlookConfigKey: `{"clientId":"c","clientSecret":"s"}`},
		User:     &models.User{ID: 1},
		Emit:     func(string) {},
	}
	deps := BuilderDeps{
		Cfg:          &config.Config{MultiUserMode: false},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
	}
	entry := NewOutlookAgentSkill(tc, deps)

	reg := tool.NewRegistry()
	reg.Register(entry)
	result, err := reg.Dispatch(context.Background(), "outlook-agent", []byte(`{"action":"search","query":"x"}`))
	require.NoError(t, err)
	require.Contains(t, result, "reg-msg")
}
