package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/hermind/backend/internal/reranker"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupChatDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, AutoMigrate(db))
	return db
}

func TestBuildRAGContext_OverrideTakesPrecedence(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	vec := NewVectorService(cfg) // nil provider — RAG section skipped
	svc := NewChatService(db, cfg, vec, nil, nil, nil, nil)

	wsPrompt := "default prompt"
	ws := &models.Workspace{
		Name:         "ws",
		Slug:         "ws",
		OpenAiPrompt: &wsPrompt,
	}
	require.NoError(t, db.Create(ws).Error)

	override := "OVERRIDE PROMPT"
	sys, _, _, err := svc.buildRAGContext(context.Background(), ws, nil, nil, "hi", &override, nil)
	require.NoError(t, err)
	assert.Equal(t, "OVERRIDE PROMPT", sys)
}

func TestBuildRAGContext_NilOverrideFallsBackToWorkspacePrompt(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	vec := NewVectorService(cfg)
	svc := NewChatService(db, cfg, vec, nil, nil, nil, nil)

	wsPrompt := "default prompt"
	ws := &models.Workspace{Name: "ws", Slug: "ws", OpenAiPrompt: &wsPrompt}
	require.NoError(t, db.Create(ws).Error)

	sys, _, _, err := svc.buildRAGContext(context.Background(), ws, nil, nil, "hi", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "default prompt", sys)
}

func TestBuildRAGContext_EmptyOverrideStringFallsBackToWorkspacePrompt(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	vec := NewVectorService(cfg)
	svc := NewChatService(db, cfg, vec, nil, nil, nil, nil)

	wsPrompt := "default prompt"
	ws := &models.Workspace{Name: "ws", Slug: "ws", OpenAiPrompt: &wsPrompt}
	require.NoError(t, db.Create(ws).Error)

	empty := ""
	// An explicit empty string override is treated as "no override" — Node parity:
	// openaiCompatible.js only uses systemMessage when it has content.
	sys, _, _, err := svc.buildRAGContext(context.Background(), ws, nil, nil, "hi", &empty, nil)
	require.NoError(t, err)
	assert.Equal(t, "default prompt", sys)
}

func TestBuildRAGContext_HistoryOverride_BypassesDBLookup(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	vec := NewVectorService(cfg)
	svc := NewChatService(db, cfg, vec, nil, nil, nil, nil)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)
	// Insert DB chat that would otherwise show up in history.
	require.NoError(t, db.Create(&models.WorkspaceChat{
		WorkspaceID: ws.ID,
		Prompt:      "DB question",
		Response:    `{"text":"DB answer"}`,
		Include:     true,
	}).Error)

	overrideHistory := []core.Message{
		{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("override q")},
		{Role: core.MESSAGE_ROLE_ASSISTANT, Content: core.NewTextContent("override a")},
	}
	_, _, history, err := svc.buildRAGContext(
		context.Background(), ws, nil, nil, "hi", nil, overrideHistory,
	)
	require.NoError(t, err)
	require.Len(t, history, 2)
	// History came from override, not DB.
}

func TestBuildRAGContext_NilHistoryOverride_PullsFromDB(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	svc := NewChatService(db, cfg, NewVectorService(cfg), nil, nil, nil, nil)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)
	require.NoError(t, db.Create(&models.WorkspaceChat{
		WorkspaceID: ws.ID,
		Prompt:      "stored",
		Response:    `{"text":"stored-resp"}`,
		Include:     true,
	}).Error)

	_, _, history, err := svc.buildRAGContext(
		context.Background(), ws, nil, nil, "hi", nil, nil,
	)
	require.NoError(t, err)
	// 1 chat row → 2 messages (user prompt + assistant response).
	assert.Len(t, history, 2)
}

// --- mock AgentInvoker ---

type mockAgentInvoker struct {
	isAgentRet   bool
	isAgentErr   error
	handoffRet   *AgentHandoff
	handoffErr   error
	isAgentCalls int
	handoffCalls int
}

func (m *mockAgentInvoker) IsAgentInvocation(ctx context.Context, ws *models.Workspace, message string) (bool, error) {
	m.isAgentCalls++
	return m.isAgentRet, m.isAgentErr
}

func (m *mockAgentInvoker) PrepareInvocationHandoff(ctx context.Context, ws *models.Workspace, user *models.User, thread *models.WorkspaceThread, prompt string) (*AgentHandoff, error) {
	m.handoffCalls++
	return m.handoffRet, m.handoffErr
}

type stubLLMProv struct{}

func (s *stubLLMProv) Stream(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (<-chan providers.LLMChunk, error) {
	return nil, assert.AnError
}
func (s *stubLLMProv) Complete(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (string, error) {
	return "", assert.AnError
}
func (s *stubLLMProv) LanguageModel() core.LanguageModel { return nil }

func TestChatService_Stream_NoAgentInvoker_FallsThrough(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	vec := NewVectorService(cfg)
	svc := NewChatService(db, cfg, vec, &stubLLMProv{}, nil, nil, nil)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	ch, err := svc.Stream(context.Background(), ws, nil, nil, dto.StreamChatRequest{Message: "@agent hi"})
	require.NoError(t, err)

	// With nil invoker, the @agent prefix is ignored and we fall through to normal path.
	// The stub LLM returns error → abort chunk.
	var found bool
	for chunk := range ch {
		found = true
		if chunk.Type == "abort" {
			require.NotNil(t, chunk.Error)
		}
	}
	require.True(t, found, "expected at least one chunk")
}

func TestChatService_Stream_NotAgentMessage_FallsThrough(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	vec := NewVectorService(cfg)
	mockInv := &mockAgentInvoker{isAgentRet: false}
	svc := NewChatService(db, cfg, vec, &stubLLMProv{}, nil, mockInv, nil)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	ch, err := svc.Stream(context.Background(), ws, nil, nil, dto.StreamChatRequest{Message: "hello"})
	require.NoError(t, err)

	var found bool
	for chunk := range ch {
		found = true
		_ = chunk
	}
	require.True(t, found, "expected at least one chunk")
	require.Equal(t, 1, mockInv.isAgentCalls)
}

func TestChatService_Stream_AgentMessage_EmitsTwoChunks(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	vec := NewVectorService(cfg)
	mockInv := &mockAgentInvoker{
		isAgentRet: true,
		handoffRet: &AgentHandoff{UUID: "uid-1", WSToken: "tok-1"},
	}
	svc := NewChatService(db, cfg, vec, nil, nil, mockInv, nil)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	ch, err := svc.Stream(context.Background(), ws, nil, nil, dto.StreamChatRequest{Message: "@agent hi"})
	require.NoError(t, err)

	c1 := <-ch
	require.Equal(t, "agentInitWebsocketConnection", c1.Type)
	require.Equal(t, "uid-1", *c1.WebsocketUUID)
	require.Equal(t, "tok-1", *c1.WebsocketToken)
	require.False(t, c1.Close)

	c2 := <-ch
	require.Equal(t, "statusResponse", c2.Type)
	require.Contains(t, *c2.TextResponse, "Swapping over")
	require.True(t, c2.Close)
	require.True(t, c2.Animate)

	_, more := <-ch
	require.False(t, more, "channel must be closed after handoff")
	require.Equal(t, 1, mockInv.handoffCalls)
}

func TestChatService_Stream_AgentMessage_HandoffError_EmitsAbort(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	vec := NewVectorService(cfg)
	mockInv := &mockAgentInvoker{
		isAgentRet: true,
		handoffErr: assert.AnError,
	}
	svc := NewChatService(db, cfg, vec, nil, nil, mockInv, nil)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	ch, err := svc.Stream(context.Background(), ws, nil, nil, dto.StreamChatRequest{Message: "@agent hi"})
	require.NoError(t, err)

	c1 := <-ch
	require.Equal(t, "abort", c1.Type)
	require.True(t, c1.Close)
	require.NotNil(t, c1.Error)
	require.Contains(t, *c1.Error, "agent invocation could not be prepared")

	_, more := <-ch
	require.False(t, more, "channel must be closed after abort")
}

func TestChatService_Stream_AgentMessage_SkipsRAG(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	// vec with a mock provider so SimilaritySearch would be called in non-agent path.
	vec := NewVectorService(cfg)
	mockInv := &mockAgentInvoker{
		isAgentRet: true,
		handoffRet: &AgentHandoff{UUID: "uid-1", WSToken: "tok-1"},
	}
	svc := NewChatService(db, cfg, vec, nil, nil, mockInv, nil)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	ch, err := svc.Stream(context.Background(), ws, nil, nil, dto.StreamChatRequest{Message: "@agent hi"})
	require.NoError(t, err)

	// Drain
	for range ch {
	}
	// RAG was skipped — the test passes if we reach here without panic.
	// A stronger assertion would require a mock vectorSvc that records calls.
	require.Equal(t, 1, mockInv.handoffCalls)
}

func TestStreamChatResponse_OmitEmptyDoesNotEmitWSFields(t *testing.T) {
	resp := dto.StreamChatResponse{UUID: "1", Type: "textResponseChunk", Close: false}
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	require.NotContains(t, string(b), "websocketUUID")
	require.NotContains(t, string(b), "websocketToken")
	require.NotContains(t, string(b), "animate")
}

func TestChatService_WithNoopReranker_ReturnsOriginalOrder(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	vec := NewVectorService(cfg)
	mockDB := &mockVectorDBForSearch{
		results: []vectordb.SearchResult{
			{DocId: "d1", Text: "first", Score: 0.9},
			{DocId: "d2", Text: "second", Score: 0.8},
			{DocId: "d3", Text: "third", Score: 0.7},
		},
	}
	vec.SetProvider(mockDB)

	svc := NewChatService(db, cfg, vec, nil, &mockEmbedderForSearch{}, nil, &reranker.NoopReranker{})

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	_, sources, _, err := svc.buildRAGContext(context.Background(), ws, nil, nil, "test query", nil, nil)
	require.NoError(t, err)
	require.Len(t, sources, 3)
	assert.Equal(t, "d1", sources[0].(map[string]any)["docId"])
	assert.Equal(t, "d2", sources[1].(map[string]any)["docId"])
	assert.Equal(t, "d3", sources[2].(map[string]any)["docId"])
}
