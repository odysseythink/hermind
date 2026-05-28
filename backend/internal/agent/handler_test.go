package agent_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestHandleWS_FullConversation_SingleTurn(t *testing.T) {
	env := newAgentTestEnv(t)
	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{"Hello back!", "TERMINATE"},
	}
	env.Runtime.SetTestLanguageModelOverride(mock)
	ws := seedWorkspace(t, env.DB)
	uid, _ := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent hi")
	tok := env.IssueTempToken(t, env.User.ID, time.Minute)
	conn, _ := env.DialWS(t, "/api/agent-invocation/"+uid, tok)

	_ = expectFrame(t, conn, agent.FrameStatusResponse)
	chat := expectFrame(t, conn, "")
	require.Equal(t, "@agent", chat.From)
	require.Equal(t, "USER", chat.To)
	require.Equal(t, "Hello back!", chat.Content)
	require.Equal(t, "success", chat.State)

	_, _, err := conn.ReadMessage()
	require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure))
}

func TestHandleWS_FullConversation_InterruptAndContinue(t *testing.T) {
	env := newAgentTestEnv(t)
	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{"INTERRUPT", "Continuing now.", "TERMINATE"},
	}
	env.Runtime.SetTestLanguageModelOverride(mock)
	ws := seedWorkspace(t, env.DB)
	uid, _ := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent")
	tok := env.IssueTempToken(t, env.User.ID, time.Minute)
	conn, _ := env.DialWS(t, "/api/agent-invocation/"+uid, tok)

	_ = expectFrame(t, conn, agent.FrameStatusResponse)
	waiting := expectFrame(t, conn, agent.FrameWaitingOnInput)
	require.Contains(t, waiting.Question, "Provide feedback")

	require.NoError(t, conn.WriteJSON(agent.ClientFrame{Type: agent.FrameAwaitingFeedback, Feedback: "go ahead"}))

	// After Continue, USER's noop model returns TERMINATE, so conversation ends
	// without an additional @agent reply. Wait for close frame.
	_, _, err := conn.ReadMessage()
	require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure))
}

func TestHandleWS_ContextCancelOnSocketClose(t *testing.T) {
	env := newAgentTestEnv(t)
	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{slowReply},
	}
	env.Runtime.SetTestLanguageModelOverride(mock)
	ws := seedWorkspace(t, env.DB)
	uid, _ := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent")
	tok := env.IssueTempToken(t, env.User.ID, time.Minute)
	conn, _ := env.DialWS(t, "/api/agent-invocation/"+uid, tok)

	_ = expectFrame(t, conn, agent.FrameStatusResponse)

	// Close client side
	_ = conn.Close()

	// Wait for server to clean up
	time.Sleep(200 * time.Millisecond)

	inv, err := env.Runtime.GetInvocation(context.Background(), uid)
	require.ErrorIs(t, err, agent.ErrInvocationClosed)
	require.Nil(t, inv)
}

func TestRuntime_Shutdown_ClosesAllSessions(t *testing.T) {
	env := newAgentTestEnv(t)
	// Provide enough slow replies for 3 sessions × 2 turns each
	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{slowReply, slowReply, slowReply, slowReply, slowReply, slowReply},
	}
	env.Runtime.SetTestLanguageModelOverride(mock)

	// Open 3 sessions with unique workspaces
	var conns []*websocket.Conn
	for i := 0; i < 3; i++ {
		ws := &models.Workspace{
			Name: fmt.Sprintf("Test Workspace %d", i),
			Slug: fmt.Sprintf("test-workspace-%d", i),
		}
		require.NoError(t, env.DB.Create(ws).Error)
		uid, _ := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent")
		tok := env.IssueTempToken(t, env.User.ID, time.Minute)
		conn, _ := env.DialWS(t, "/api/agent-invocation/"+uid, tok)
		_ = expectFrame(t, conn, agent.FrameStatusResponse)
		conns = append(conns, conn)
	}

	// Verify 3 sessions are active
	count := 0
	env.Runtime.Sessions().Range(func(_, _ any) bool {
		count++
		return true
	})
	require.Equal(t, 3, count)

	// Shutdown should abort all sessions
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := env.Runtime.Shutdown(ctx)
	require.NoError(t, err)

	// All sessions should be gone
	count = 0
	env.Runtime.Sessions().Range(func(_, _ any) bool {
		count++
		return true
	})
	require.Equal(t, 0, count)
}

func TestHandleWS_LLMError_EmitsWSSFailure(t *testing.T) {
	env := newAgentTestEnv(t)
	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{}, // empty → mock returns error on first call
	}
	env.Runtime.SetTestLanguageModelOverride(mock)
	ws := seedWorkspace(t, env.DB)
	uid, _ := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent")
	tok := env.IssueTempToken(t, env.User.ID, time.Minute)
	conn, _ := env.DialWS(t, "/api/agent-invocation/"+uid, tok)

	_ = expectFrame(t, conn, agent.FrameStatusResponse)
	fail := expectFrame(t, conn, agent.FrameWSSFailure)
	require.Contains(t, fail.Content, "mock ran out of replies")

	// wssFailure may be sent twice (pantheon edge case); read until close frame
	for {
		_, _, err := conn.ReadMessage()
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestHandleWS_NoLLMAvailable_RejectsBeforeUpgrade(t *testing.T) {
	env := newAgentTestEnv(t)
	// No override, no API key → buildLanguageModel returns error
	ws := seedWorkspace(t, env.DB)
	uid, _ := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent")

	u, _ := url.Parse(env.Server.URL)
	u.Scheme = "ws"
	u.Path = "/api/agent-invocation/" + uid
	q := u.Query()
	q.Set("token", env.IssueTempToken(t, env.User.ID, time.Minute))
	u.RawQuery = q.Encode()
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.Error(t, err)
	require.Nil(t, conn)
	if resp != nil {
		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	}
}

func TestHandleWS_UnknownUUID_Closes1008(t *testing.T) {
	env := newAgentTestEnv(t)
	u, _ := url.Parse(env.Server.URL)
	u.Scheme = "ws"
	u.Path = "/api/agent-invocation/unknown-uuid"
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.Error(t, err)
	require.Nil(t, conn)
	if resp != nil {
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	}
}

func TestHandleWS_ClosedInvocation_Closes1008(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := seedWorkspace(t, env.DB)
	uid, err := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent hello")
	require.NoError(t, err)
	err = env.Runtime.CloseInvocation(context.Background(), uid)
	require.NoError(t, err)

	u, _ := url.Parse(env.Server.URL)
	u.Scheme = "ws"
	u.Path = "/api/agent-invocation/" + uid
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.Error(t, err)
	require.Nil(t, conn)
	if resp != nil {
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	}
}

func TestHandleWS_SessionExceedsMaxDuration_EmitsWSSFailureAndCloses(t *testing.T) {
	// Custom env with a very short session max duration
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	cfg := &config.Config{StorageDir: t.TempDir(), AgentSessionMaxDuration: 500 * time.Millisecond}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)
	rt := agent.NewRuntime(agent.Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc,
	})
	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{slowReply}, // stalls until timeout
	}
	rt.SetTestLanguageModelOverride(mock)

	eng := gin.New()
	api := eng.Group("/api")
	api.GET("/agent-invocation/:uuid", func(c *gin.Context) { rt.HandleWS(c) })
	srv := httptest.NewServer(eng)
	defer srv.Close()

	ws := seedWorkspace(t, db)
	uid, _ := rt.CreateInvocation(context.Background(), ws, seedAdminUser(t, db), nil, "@agent")
	tok, _ := tempTokenSvc.IssueWithTTL(context.Background(), 1, time.Minute)

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

	// Wait for timeout frame
	var failure agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&failure))
	require.Equal(t, agent.FrameWSSFailure, failure.Type)
	require.Contains(t, failure.Content, "Session reached maximum duration")
}
