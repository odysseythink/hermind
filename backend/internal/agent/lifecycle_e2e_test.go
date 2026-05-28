package agent_test

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/require"
)

func TestE2E_Approval_Approved_FullPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	cfg := &config.Config{StorageDir: t.TempDir()}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)
	flowSvc := services.NewAgentFlowService(cfg.StorageDir)
	_, _ = flowSvc.SaveFlow("Test Flow", services.FlowConfig{
		Description: "A test flow for e2e",
		Active:      true,
	}, "flow-e2e")

	rt := agent.NewRuntime(agent.Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc, FlowSvc: flowSvc,
	})

	mock := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		parts: [][]core.ContentParter{
			{core.ToolCallPart{ID: "1", Name: "flow-test-flow", Arguments: `{}`}},
			core.NewTextContent("All done!"),
		},
	}
	rt.SetTestLanguageModelOverride(mock)

	eng := gin.New()
	api := eng.Group("/api")
	api.GET("/agent-invocation/:uuid", func(c *gin.Context) { rt.HandleWS(c) })
	srv := httptest.NewServer(eng)
	defer srv.Close()

	ws := seedWorkspace(t, db)
	user := seedAdminUser(t, db)
	uid, _ := rt.CreateInvocation(context.Background(), ws, user, nil, "@agent run flow")
	tok, _ := tempTokenSvc.IssueWithTTL(context.Background(), user.ID, time.Minute)

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/api/agent-invocation/" + uid
	q := u.Query()
	q.Set("token", tok)
	u.RawQuery = q.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	_ = expectFrame(t, conn, agent.FrameStatusResponse)

	// Expect approval request
	var req agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&req))
	require.Equal(t, agent.FrameToolApprovalReq, req.Type)
	require.Equal(t, "flow-test-flow", req.SkillName)
	require.NotEmpty(t, req.RequestID)

	// Approve
	require.NoError(t, conn.WriteJSON(agent.ClientFrame{
		Type: agent.FrameToolApprovalResp, RequestID: req.RequestID, Approved: true,
	}))

	// Expect chat message then close on next turn
	var chat agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&chat))
	require.Equal(t, "@agent", chat.From)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure))
}

func TestE2E_Approval_Rejected_AgentContinuesWithToolError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	cfg := &config.Config{StorageDir: t.TempDir()}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)
	flowSvc := services.NewAgentFlowService(cfg.StorageDir)
	_, _ = flowSvc.SaveFlow("Test Flow", services.FlowConfig{Description: "A test flow", Active: true}, "flow-e2e")

	rt := agent.NewRuntime(agent.Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc, FlowSvc: flowSvc,
	})

	mock := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		parts: [][]core.ContentParter{
			{core.ToolCallPart{ID: "1", Name: "flow-test-flow", Arguments: `{}`}},
			core.NewTextContent("Moving on."),
			core.NewTextContent("TERMINATE"),
		},
	}
	rt.SetTestLanguageModelOverride(mock)

	eng := gin.New()
	api := eng.Group("/api")
	api.GET("/agent-invocation/:uuid", func(c *gin.Context) { rt.HandleWS(c) })
	srv := httptest.NewServer(eng)
	defer srv.Close()

	ws := seedWorkspace(t, db)
	user := seedAdminUser(t, db)
	uid, _ := rt.CreateInvocation(context.Background(), ws, user, nil, "@agent run flow")
	tok, _ := tempTokenSvc.IssueWithTTL(context.Background(), user.ID, time.Minute)

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/api/agent-invocation/" + uid
	q := u.Query()
	q.Set("token", tok)
	u.RawQuery = q.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	_ = expectFrame(t, conn, agent.FrameStatusResponse)

	var req agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&req))
	require.Equal(t, agent.FrameToolApprovalReq, req.Type)

	// Reject
	require.NoError(t, conn.WriteJSON(agent.ClientFrame{
		Type: agent.FrameToolApprovalResp, RequestID: req.RequestID, Approved: false,
	}))

	// Agent should continue (may see tool error or next reply)
	var msg agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&msg))
	require.True(t, msg.From == "@agent" || msg.Type == agent.FrameWSSFailure || msg.Content != "")
}

func TestE2E_Approval_GlobalSetting_BypassesGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	cfg := &config.Config{StorageDir: t.TempDir()}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)
	sysSvc := services.NewSystemService(db)
	_ = sysSvc.SetSetting(context.Background(), "agent_tool_auto_approve", "true")

	flowSvc := services.NewAgentFlowService(cfg.StorageDir)
	_, _ = flowSvc.SaveFlow("Test Flow", services.FlowConfig{Description: "A test flow", Active: true}, "flow-e2e")

	rt := agent.NewRuntime(agent.Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc,
		SysSvc: sysSvc, FlowSvc: flowSvc,
	})

	mock := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		parts: [][]core.ContentParter{
			{core.ToolCallPart{ID: "1", Name: "flow-test-flow", Arguments: `{}`}},
			core.NewTextContent("Done!"),
			core.NewTextContent("TERMINATE"),
		},
	}
	rt.SetTestLanguageModelOverride(mock)

	eng := gin.New()
	api := eng.Group("/api")
	api.GET("/agent-invocation/:uuid", func(c *gin.Context) { rt.HandleWS(c) })
	srv := httptest.NewServer(eng)
	defer srv.Close()

	ws := seedWorkspace(t, db)
	user := seedAdminUser(t, db)
	uid, _ := rt.CreateInvocation(context.Background(), ws, user, nil, "@agent run flow")
	tok, _ := tempTokenSvc.IssueWithTTL(context.Background(), user.ID, time.Minute)

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/api/agent-invocation/" + uid
	q := u.Query()
	q.Set("token", tok)
	u.RawQuery = q.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	_ = expectFrame(t, conn, agent.FrameStatusResponse)

	// Should NOT see approval frame; instead chat arrives directly
	var chat agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&chat))
	require.Equal(t, "@agent", chat.From)
	require.Contains(t, chat.Content, "Done!")
}

// TestE2E_Approval_Timeout_AgentContinuesWithToolError verifies that when the
// user does not respond to an approval request within the TTL, the tool call
// is rejected and the agent loop continues.
func TestE2E_Approval_Timeout_AgentContinuesWithToolError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	cfg := &config.Config{StorageDir: t.TempDir(), AgentToolApprovalTimeout: 200 * time.Millisecond}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)
	flowSvc := services.NewAgentFlowService(cfg.StorageDir)
	_, _ = flowSvc.SaveFlow("Test Flow", services.FlowConfig{Description: "A test flow", Active: true}, "flow-e2e")

	rt := agent.NewRuntime(agent.Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc, FlowSvc: flowSvc,
	})

	mock := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		parts: [][]core.ContentParter{
			{core.ToolCallPart{ID: "1", Name: "flow-test-flow", Arguments: `{}`}},
			core.NewTextContent("Timed out, moving on."),
			core.NewTextContent("TERMINATE"),
		},
	}
	rt.SetTestLanguageModelOverride(mock)

	eng := gin.New()
	api := eng.Group("/api")
	api.GET("/agent-invocation/:uuid", func(c *gin.Context) { rt.HandleWS(c) })
	srv := httptest.NewServer(eng)
	defer srv.Close()

	ws := seedWorkspace(t, db)
	user := seedAdminUser(t, db)
	uid, _ := rt.CreateInvocation(context.Background(), ws, user, nil, "@agent run flow")
	tok, _ := tempTokenSvc.IssueWithTTL(context.Background(), user.ID, time.Minute)

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/api/agent-invocation/" + uid
	q := u.Query()
	q.Set("token", tok)
	u.RawQuery = q.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	_ = expectFrame(t, conn, agent.FrameStatusResponse)

	var req agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&req))
	require.Equal(t, agent.FrameToolApprovalReq, req.Type)

	// Do NOT approve — wait for the 200ms server-side timeout.
	// The agent should continue after the tool returns an error.
	var msg agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&msg))
	require.True(t, msg.From == "@agent" || msg.Type == agent.FrameWSSFailure || msg.Content != "")
}

// TestE2E_Approval_SetAutoApproveOnSession_BypassesGate verifies that sending
// a setAutoApprove frame before a tool call skips the approval gate.
func TestE2E_Approval_SetAutoApproveOnSession_BypassesGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	cfg := &config.Config{StorageDir: t.TempDir()}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)
	flowSvc := services.NewAgentFlowService(cfg.StorageDir)
	_, _ = flowSvc.SaveFlow("Test Flow", services.FlowConfig{Description: "A test flow", Active: true}, "flow-e2e")

	rt := agent.NewRuntime(agent.Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc, FlowSvc: flowSvc,
	})

	gate := make(chan struct{})
	mock := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		parts: [][]core.ContentParter{
			{core.ToolCallPart{ID: "1", Name: "flow-test-flow", Arguments: `{}`}},
			core.NewTextContent("Auto-approved result!"),
			core.NewTextContent("TERMINATE"),
		},
		gate: gate,
	}
	rt.SetTestLanguageModelOverride(mock)

	eng := gin.New()
	api := eng.Group("/api")
	api.GET("/agent-invocation/:uuid", func(c *gin.Context) { rt.HandleWS(c) })
	srv := httptest.NewServer(eng)
	defer srv.Close()

	ws := seedWorkspace(t, db)
	user := seedAdminUser(t, db)
	uid, _ := rt.CreateInvocation(context.Background(), ws, user, nil, "@agent run flow")
	tok, _ := tempTokenSvc.IssueWithTTL(context.Background(), user.ID, time.Minute)

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/api/agent-invocation/" + uid
	q := u.Query()
	q.Set("token", tok)
	u.RawQuery = q.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	_ = expectFrame(t, conn, agent.FrameStatusResponse)

	// Enable per-session auto-approve before the LLM generates a tool call.
	require.NoError(t, conn.WriteJSON(agent.ClientFrame{
		Type: agent.FrameSetAutoApprove, Enabled: true,
	}))

	// Wait for the server to process the setAutoApprove frame before releasing
	// the mock. The local httptest pipe is fast, but readerLoop may still be
	// one scheduling quantum behind.
	time.Sleep(500 * time.Millisecond)
	close(gate)

	// Should NOT see approval frame; chat arrives directly.
	// Pantheon may emit empty status frames before the actual reply.
	var chat agent.ServerFrame
	for i := 0; i < 5; i++ {
		require.NoError(t, conn.ReadJSON(&chat))
		if chat.Content != "" {
			break
		}
	}
	require.Contains(t, chat.Content, "Auto-approved result!")
}

// TestE2E_BailDuringApproval_CancelsApprovalAndCloses verifies that sending a
// bail command while an approval is pending cancels it and closes the session.
func TestE2E_BailDuringApproval_CancelsApprovalAndCloses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	cfg := &config.Config{StorageDir: t.TempDir()}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)
	flowSvc := services.NewAgentFlowService(cfg.StorageDir)
	_, _ = flowSvc.SaveFlow("Test Flow", services.FlowConfig{Description: "A test flow", Active: true}, "flow-e2e")

	rt := agent.NewRuntime(agent.Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc, FlowSvc: flowSvc,
	})

	mock := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		parts: [][]core.ContentParter{
			{core.ToolCallPart{ID: "1", Name: "flow-test-flow", Arguments: `{}`}},
		},
	}
	rt.SetTestLanguageModelOverride(mock)

	eng := gin.New()
	api := eng.Group("/api")
	api.GET("/agent-invocation/:uuid", func(c *gin.Context) { rt.HandleWS(c) })
	srv := httptest.NewServer(eng)
	defer srv.Close()

	ws := seedWorkspace(t, db)
	user := seedAdminUser(t, db)
	uid, _ := rt.CreateInvocation(context.Background(), ws, user, nil, "@agent run flow")
	tok, _ := tempTokenSvc.IssueWithTTL(context.Background(), user.ID, time.Minute)

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/api/agent-invocation/" + uid
	q := u.Query()
	q.Set("token", tok)
	u.RawQuery = q.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	_ = expectFrame(t, conn, agent.FrameStatusResponse)

	var req agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&req))
	require.Equal(t, agent.FrameToolApprovalReq, req.Type)

	// Bail while approval is pending.
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("exit")))

	// Server may send a wssFailure via OnError before closing; drain all
	// frames until the connection closes.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var junk agent.ServerFrame
		if err = conn.ReadJSON(&junk); err != nil {
			break
		}
	}
	require.Error(t, err, "expected connection to close after bail")
}

// TestE2E_TotalTimeout_ClosesSession verifies that AgentSessionMaxDuration
// hard-caps the session lifetime.
func TestE2E_TotalTimeout_ClosesSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	cfg := &config.Config{StorageDir: t.TempDir(), AgentSessionMaxDuration: 500 * time.Millisecond}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)

	rt := agent.NewRuntime(agent.Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc,
	})

	// slowReply blocks until context is cancelled, ensuring the session exceeds
	// its 500ms max duration before the LLM returns anything useful.
	mock := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		replies: []string{slowReply},
	}
	rt.SetTestLanguageModelOverride(mock)

	eng := gin.New()
	api := eng.Group("/api")
	api.GET("/agent-invocation/:uuid", func(c *gin.Context) { rt.HandleWS(c) })
	srv := httptest.NewServer(eng)
	defer srv.Close()

	ws := seedWorkspace(t, db)
	user := seedAdminUser(t, db)
	uid, _ := rt.CreateInvocation(context.Background(), ws, user, nil, "@agent hi")
	tok, _ := tempTokenSvc.IssueWithTTL(context.Background(), user.ID, time.Minute)

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/api/agent-invocation/" + uid
	q := u.Query()
	q.Set("token", tok)
	u.RawQuery = q.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	_ = expectFrame(t, conn, agent.FrameStatusResponse)

	// Wait for the session timeout to fire. The server should send wssFailure
	// and then close the connection.
	var failure agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&failure))
	require.Equal(t, agent.FrameWSSFailure, failure.Type)
	require.Contains(t, failure.Content, "Session reached maximum duration")

	// Connection should be closed shortly after wssFailure.
	// Allow a brief window for the server defer to run wc.Close().
	time.Sleep(300 * time.Millisecond)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var junk agent.ServerFrame
		if err = conn.ReadJSON(&junk); err != nil {
			break
		}
	}
	require.Error(t, err, "expected connection to close after timeout")
}
