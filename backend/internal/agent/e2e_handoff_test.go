package agent_test

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
)

type fullAgentE2EEnv struct {
	*agentTestEnv
	ChatSvc *services.ChatService
}

func newFullAgentE2EEnv(t *testing.T) *fullAgentE2EEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	cfg := &config.Config{StorageDir: t.TempDir()}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)
	rt := agent.NewRuntime(agent.Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc,
	})
	vec := services.NewVectorService(cfg)
	chatSvc := services.NewChatService(db, cfg, vec, nil, nil, rt, nil, nil, nil)

	eng := gin.New()
	api := eng.Group("/api")

	// Minimal stream-chat handler (no auth middleware — test-only).
	api.POST("/workspace/:slug/stream-chat", func(c *gin.Context) {
		var ws models.Workspace
		if err := db.Where("slug = ?", c.Param("slug")).First(&ws).Error; err != nil {
			c.JSON(404, gin.H{"error": "workspace not found"})
			return
		}
		var req dto.StreamChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		stream, err := chatSvc.Stream(c.Request.Context(), &ws, nil, nil, req)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		for chunk := range stream {
			data, _ := json.Marshal(chunk)
			c.Writer.Write([]byte("data: " + string(data) + "\n\n"))
			if f, ok := c.Writer.(http.Flusher); ok {
				f.Flush()
			}
		}
	})

	// Agent-invocation WS route (bypass auth for direct e2e).
	api.GET("/agent-invocation/:uuid", func(c *gin.Context) {
		rt.HandleWS(c)
	})

	srv := httptest.NewServer(eng)
	t.Cleanup(srv.Close)

	return &fullAgentE2EEnv{
		agentTestEnv: &agentTestEnv{
			DB:           db,
			Cfg:          cfg,
			TempTokenSvc: tempTokenSvc,
			AuthSvc:      authSvc,
			Runtime:      rt,
			Server:       srv,
			User:         seedAdminUser(t, db),
		},
		ChatSvc: chatSvc,
	}
}

func TestE2E_AgentChat_FullHandoff(t *testing.T) {
	env := newFullAgentE2EEnv(t)
	ws := seedWorkspace(t, env.DB)
	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{"Hello back!", "TERMINATE"},
	}
	env.Runtime.SetTestLanguageModelOverride(mock)

	// Phase 1: POST stream-chat with @agent prefix
	reqBody := `{"message":"@agent hi"}`
	req, _ := http.NewRequest("POST", env.Server.URL+"/api/workspace/"+ws.Slug+"/stream-chat", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Parse first two SSE chunks
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	chunks := parseSSEBody(t, string(bodyBytes))
	require.Len(t, chunks, 2)
	initChunk := chunks[0]
	statusChunk := chunks[1]
	require.Equal(t, "agentInitWebsocketConnection", initChunk.Type)
	require.NotNil(t, initChunk.WebsocketUUID)
	require.NotNil(t, initChunk.WebsocketToken)
	require.Equal(t, "statusResponse", statusChunk.Type)
	require.True(t, statusChunk.Close)

	// Phase 2: dial WS with the issued token
	u, _ := url.Parse(env.Server.URL)
	u.Scheme = "ws"
	u.Path = "/api/agent-invocation/" + *initChunk.WebsocketUUID
	q := u.Query()
	q.Set("token", *initChunk.WebsocketToken)
	u.RawQuery = q.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	// Phase 3: expect welcome + assistant reply + close
	var welcome agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&welcome))
	require.Equal(t, agent.FrameStatusResponse, welcome.Type)

	var chat agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&chat))
	require.Equal(t, "@agent", chat.From)
	require.Equal(t, "Hello back!", chat.Content)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure))
}

// parseSSEBody parses all `data: <json>` lines from an SSE response body.
func parseSSEBody(t *testing.T, body string) []dto.StreamChatResponse {
	t.Helper()
	var chunks []dto.StreamChatResponse
	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		var chunk dto.StreamChatResponse
		require.NoError(t, json.Unmarshal([]byte(payload), &chunk))
		chunks = append(chunks, chunk)
	}
	return chunks
}
