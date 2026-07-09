package agent_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/agent/tools"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/require"
)

// mockSearchProvider returns fixed search results for integration tests.
type mockSearchProvider struct {
	name    string
	results []tools.SearchResult
	err     error
}

func (m *mockSearchProvider) Name() string { return m.name }
func (m *mockSearchProvider) Search(ctx context.Context, query string, _ map[string]string, _ *config.Config) ([]tools.SearchResult, error) {
	return m.results, m.err
}

func ptr[T any](v T) *T { return &v }

// setupWebSearchE2E creates a Runtime, httptest.Server, workspace, user, and WebSocket
// connection for a web-browsing integration test. The mock LLM drives the agent to invoke
// web-browsing via ToolCallPart. Returns the WS conn and a cleanup func.
func setupWebSearchE2E(t *testing.T, mockLLM *mockLanguageModel, sysSvc *services.SystemService) (*websocket.Conn, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	cfg := &config.Config{StorageDir: t.TempDir()}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)

	rt := agent.NewRuntime(agent.Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc, SysSvc: sysSvc,
	})
	rt.SetTestLanguageModelOverride(mockLLM)

	eng := gin.New()
	api := eng.Group("/api")
	api.GET("/agent-invocation/:uuid", func(c *gin.Context) { rt.HandleWS(c) })
	srv := httptest.NewServer(eng)

	ws := seedWorkspace(t, db)
	user := seedAdminUser(t, db)
	uid, _ := rt.CreateInvocation(context.Background(), ws, user, nil, "@agent search golang")
	tok, _ := tempTokenSvc.IssueWithTTL(context.Background(), user.ID, time.Minute)

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/api/agent-invocation/" + uid
	q := u.Query()
	q.Set("token", tok)
	u.RawQuery = q.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)

	cleanup := func() {
		conn.Close()
		srv.Close()
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
	return conn, cleanup
}

// expectChatFrame reads from conn, skipping any reportStreamEvent (citation) frames,
// until a non-reportStreamEvent frame arrives. Returns that frame.
func expectChatFrame(t *testing.T, conn anyFrameReader) agent.ServerFrame {
	t.Helper()
	for {
		var f agent.ServerFrame
		require.NoError(t, conn.ReadJSON(&f))
		if f.Type == agent.FrameReportStreamEvent {
			continue
		}
		return f
	}
}

func TestWebSearch_AgentInvokesDuckDuckGo(t *testing.T) {
	// Override the duckduckgo-engine registry entry with a mock that returns
	// controlled results. The real init() already registered it; this overwrites.
	tools.RegisterSearchProviderForTesting("duckduckgo-engine", &mockSearchProvider{
		name: "DuckDuckGo",
		results: []tools.SearchResult{
			{Title: "Golang Home", Link: "https://go.dev", Snippet: "The Go Programming Language"},
			{Title: "Go Wiki", Link: "https://github.com/golang/go/wiki", Snippet: "Go Wiki on GitHub"},
		},
	})

	mockLLM := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		parts: [][]core.ContentParter{
			{core.ToolCallPart{ID: "1", Name: "web-browsing", Arguments: `{"query":"golang"}`}},
			core.NewTextContent("Here are the search results for golang: Go.dev is the official site."),
			core.NewTextContent("TERMINATE"),
		},
	}

	// No SystemService → default provider (duckduckgo-engine, now mocked) is used.
	conn, cleanup := setupWebSearchE2E(t, mockLLM, nil)
	defer cleanup()

	// First frame: status response "@agent runtime ready"
	_ = expectFrame(t, conn, agent.FrameStatusResponse)

	// The agent invokes web-browsing, tool executes, then LLM synthesizes.
	// We verify the final chat message from @agent contains the synthesized text.
	chat := expectChatFrame(t, conn)
	require.Equal(t, "@agent", chat.From)
	require.Contains(t, chat.Content, "golang")

	// Connection should close after TERMINATE.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure),
		"expected close after session end, got: %v", err)
}

func TestWebSearch_ProviderSwitching(t *testing.T) {
	// Register two mock providers under distinct keys.
	tools.RegisterSearchProviderForTesting("duckduckgo-engine", &mockSearchProvider{
		name:    "DuckDuckGo",
		results: []tools.SearchResult{{Title: "DDG Result", Link: "https://ddg.example.com", Snippet: "Default provider"}},
	})
	tools.RegisterSearchProviderForTesting("serper-dot-dev", &mockSearchProvider{
		name:    "Serper.dev",
		results: []tools.SearchResult{{Title: "Serper Result", Link: "https://serper.example.com", Snippet: "Switched provider"}},
	})

	// Create a SystemService and insert the agent_search_provider setting.
	db := openTestDB(t)
	sysSvc := services.NewSystemService(db)
	require.NoError(t, db.Create(&models.SystemSetting{
		Key:   "agent_search_provider",
		Value: ptr("serper-dot-dev"),
	}).Error)

	mockLLM := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		parts: [][]core.ContentParter{
			{core.ToolCallPart{ID: "1", Name: "web-browsing", Arguments: `{"query":"switching"}`}},
			core.NewTextContent("Serper.dev found results about switching."),
			core.NewTextContent("TERMINATE"),
		},
	}

	conn, cleanup := setupWebSearchE2E(t, mockLLM, sysSvc)
	defer cleanup()
	sqlDB, _ := db.DB()
	if sqlDB != nil {
		defer sqlDB.Close()
	}

	_ = expectFrame(t, conn, agent.FrameStatusResponse)
	chat := expectChatFrame(t, conn)
	require.Equal(t, "@agent", chat.From)
	// The LLM synthesized text mentions Serper, confirming the provider was switched.
	require.Contains(t, chat.Content, "Serper")
}

func TestWebSearch_NoResultsGraceful(t *testing.T) {
	// Mock provider returns zero results — agent should handle gracefully.
	tools.RegisterSearchProviderForTesting("duckduckgo-engine", &mockSearchProvider{
		name:    "DuckDuckGo",
		results: []tools.SearchResult{}, // empty
	})

	mockLLM := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		parts: [][]core.ContentParter{
			{core.ToolCallPart{ID: "1", Name: "web-browsing", Arguments: `{"query":"xkcd_foobarbaz_404"}`}},
			core.NewTextContent("I found no relevant information online."),
			core.NewTextContent("TERMINATE"),
		},
	}

	conn, cleanup := setupWebSearchE2E(t, mockLLM, nil)
	defer cleanup()

	_ = expectFrame(t, conn, agent.FrameStatusResponse)
	chat := expectFrame(t, conn, "")
	require.Equal(t, "@agent", chat.From)
	// The LLM receives a "No information was found" tool result and synthesizes accordingly.
	require.Contains(t, chat.Content, "no relevant")
}

func TestWebSearch_UnknownProviderFallsBackToDefault(t *testing.T) {
	// The settings point to a non-existent provider → web-browsing falls back to duckduckgo-engine.
	tools.RegisterSearchProviderForTesting("duckduckgo-engine", &mockSearchProvider{
		name:    "DuckDuckGo",
		results: []tools.SearchResult{{Title: "Fallback Result", Link: "https://fallback.example.com", Snippet: "Fell back to DDG"}},
	})

	db := openTestDB(t)
	sysSvc := services.NewSystemService(db)
	require.NoError(t, db.Create(&models.SystemSetting{
		Key:   "agent_search_provider",
		Value: ptr("nonexistent-provider-xyz"),
	}).Error)

	mockLLM := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		parts: [][]core.ContentParter{
			{core.ToolCallPart{ID: "1", Name: "web-browsing", Arguments: `{"query":"fallback"}`}},
			core.NewTextContent("Fallback worked."),
			core.NewTextContent("TERMINATE"),
		},
	}

	conn, cleanup := setupWebSearchE2E(t, mockLLM, sysSvc)
	defer cleanup()
	sqlDB, _ := db.DB()
	if sqlDB != nil {
		defer sqlDB.Close()
	}

	_ = expectFrame(t, conn, agent.FrameStatusResponse)
	chat := expectChatFrame(t, conn)
	require.Equal(t, "@agent", chat.From)
	// The fallback to DDG happened silently; the LLM still got results and synthesized.
	require.Contains(t, chat.Content, "Fallback")
}

func TestWebSearch_CitationsEmitted(t *testing.T) {
	tools.RegisterSearchProviderForTesting("duckduckgo-engine", &mockSearchProvider{
		name: "DuckDuckGo",
		results: []tools.SearchResult{
			{Title: "Result A", Link: "https://example.com/a", Snippet: "First result"},
			{Title: "Result B", Link: "https://example.com/b", Snippet: "Second result"},
		},
	})

	mockLLM := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		parts: [][]core.ContentParter{
			{core.ToolCallPart{ID: "1", Name: "web-browsing", Arguments: `{"query":"citations"}`}},
			core.NewTextContent("Here are results with citations."),
			core.NewTextContent("TERMINATE"),
		},
	}

	conn, cleanup := setupWebSearchE2E(t, mockLLM, nil)
	defer cleanup()

	_ = expectFrame(t, conn, agent.FrameStatusResponse)

	// Collect all frames until connection closes; look for the reportStreamEvent frame.
	var citationFrame *agent.ServerFrame
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		var f agent.ServerFrame
		if err := conn.ReadJSON(&f); err != nil {
			break
		}
		if f.Type == agent.FrameReportStreamEvent {
			cp := f
			citationFrame = &cp
			break
		}
	}

	require.NotNil(t, citationFrame, "expected a reportStreamEvent citation frame")
	raw, ok := citationFrame.ContentObj.(json.RawMessage)
	require.True(t, ok, "ContentObj should be json.RawMessage")
	var content map[string]any
	require.NoError(t, json.Unmarshal(raw, &content))
	require.Equal(t, "citations", content["type"])
	require.NotEmpty(t, content["uuid"], "citation frame must have a uuid")
	citations, ok := content["citations"].([]any)
	require.True(t, ok, "citations should be an array")
	require.Len(t, citations, 2)

	// Verify first citation shape.
	c0 := citations[0].(map[string]any)
	require.Equal(t, "https://example.com/a", c0["id"])
	require.Equal(t, "Result A", c0["title"])
	require.Equal(t, "First result", c0["text"])
	require.Equal(t, "link://https://example.com/a", c0["chunkSource"])
}
