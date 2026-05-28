package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEmbedder struct{}

func (m *mockEmbedder) EmbedTexts(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1.0, float32(i)}
	}
	return out, nil
}
func (m *mockEmbedder) EmbedQuery(_ context.Context, _ string) ([]float32, error) { return nil, nil }
func (m *mockEmbedder) Dimensions() int                                           { return 2 }

type mockLLM struct{ text string }

func (m *mockLLM) Complete(_ context.Context, _ []core.Message, _ string, _ *float64) (string, error) {
	return m.text, nil
}
func (m *mockLLM) Stream(_ context.Context, _ []core.Message, _ string, _ *float64) (<-chan providers.LLMChunk, error) {
	ch := make(chan providers.LLMChunk, 2)
	ch <- providers.LLMChunk{TextDelta: m.text}
	ch <- providers.LLMChunk{FinishReason: "stop"}
	close(ch)
	return ch, nil
}
func (m *mockLLM) LanguageModel() core.LanguageModel { return nil }

func newChatSvcWithMock(t *testing.T, env *apiTestEnv, llmText string) *services.ChatService {
	t.Helper()
	cfg := env.Cfg
	vec := services.NewVectorService(cfg)
	return services.NewChatService(env.DB, cfg, vec, &mockLLM{text: llmText}, nil, nil, nil)
}

func TestAPIOpenAI_Models(t *testing.T) {
	env := newAPITestEnv(t, nil)
	require.NoError(t, env.DB.Create(&models.Workspace{Name: "Chat 1", Slug: "chat-1"}).Error)
	require.NoError(t, env.DB.Create(&models.Workspace{Name: "Chat 2", Slug: "chat-2"}).Error)

	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, nil, env.DB, env.Cfg)

	req := httptest.NewRequest("GET", "/api/v1/openai/models", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "list", body.Object)
	require.Len(t, body.Data, 2)
	slugs := []string{body.Data[0].ID, body.Data[1].ID}
	assert.Contains(t, slugs, "chat-1")
	assert.Contains(t, slugs, "chat-2")
	for _, m := range body.Data {
		assert.Equal(t, "model", m.Object)
		assert.NotEmpty(t, m.OwnedBy)
	}
}

func TestAPIOpenAI_VectorStores_Empty(t *testing.T) {
	env := newAPITestEnv(t, nil)
	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, nil, env.DB, env.Cfg)

	req := httptest.NewRequest("GET", "/api/v1/openai/vector_stores", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Data    []any `json:"data"`
		HasMore bool  `json:"has_more"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Empty(t, body.Data)
	assert.False(t, body.HasMore)
}

func TestAPIOpenAI_VectorStores_WithDocs(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "My WS", Slug: "my-ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	require.NoError(t, env.DB.Create(&models.WorkspaceDocument{
		DocId: "d1", Filename: "f1", Docpath: "x/1", WorkspaceID: ws.ID,
	}).Error)
	require.NoError(t, env.DB.Create(&models.WorkspaceDocument{
		DocId: "d2", Filename: "f2", Docpath: "x/2", WorkspaceID: ws.ID,
	}).Error)

	cfg := env.Cfg
	cfg.VectorDB = "lancedb"
	wsSvc := services.NewWorkspaceService(env.DB, cfg, nil)
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, nil, env.DB, cfg)

	req := httptest.NewRequest("GET", "/api/v1/openai/vector_stores", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Data []struct {
			ID         string `json:"id"`
			Object     string `json:"object"`
			Name       string `json:"name"`
			FileCounts struct {
				Total int `json:"total"`
			} `json:"file_counts"`
			Provider string `json:"provider"`
		} `json:"data"`
		HasMore bool `json:"has_more"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	assert.Equal(t, "my-ws", body.Data[0].ID)
	assert.Equal(t, "vector_store", body.Data[0].Object)
	assert.Equal(t, "My WS", body.Data[0].Name)
	assert.Equal(t, 2, body.Data[0].FileCounts.Total)
	assert.Equal(t, "lancedb", body.Data[0].Provider)
}

func TestAPIOpenAI_VectorStores_PaginationQueryShortCircuits(t *testing.T) {
	env := newAPITestEnv(t, nil)
	require.NoError(t, env.DB.Create(&models.Workspace{Name: "x", Slug: "x"}).Error)
	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, nil, env.DB, env.Cfg)

	req := httptest.NewRequest("GET", "/api/v1/openai/vector_stores?after=x", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Data    []any `json:"data"`
		HasMore bool  `json:"has_more"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Empty(t, body.Data)
	assert.False(t, body.HasMore)
}

func TestAPIOpenAI_Embeddings_ArrayInput(t *testing.T) {
	env := newAPITestEnv(t, nil)
	env.Cfg.EmbeddingModel = "text-embedding-3-small"
	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, &mockEmbedder{}, env.DB, env.Cfg)

	payload := []byte(`{"input":["hello","world"],"model":null}`)
	req := httptest.NewRequest("POST", "/api/v1/openai/embeddings", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Object string `json:"object"`
		Data   []struct {
			Object    string    `json:"object"`
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
		Model string `json:"model"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "list", body.Object)
	require.Len(t, body.Data, 2)
	assert.Equal(t, "embedding", body.Data[0].Object)
	assert.Equal(t, 0, body.Data[0].Index)
	assert.Equal(t, 1, body.Data[1].Index)
	assert.Equal(t, "text-embedding-3-small", body.Model)
}

func TestAPIOpenAI_Embeddings_StringInputCoerced(t *testing.T) {
	env := newAPITestEnv(t, nil)
	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, &mockEmbedder{}, env.DB, env.Cfg)

	payload := []byte(`{"input":"hello"}`)
	req := httptest.NewRequest("POST", "/api/v1/openai/embeddings", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Data []any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Data, 1)
}

func TestAPIOpenAI_Embeddings_EmptyInput500(t *testing.T) {
	env := newAPITestEnv(t, nil)
	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, &mockEmbedder{}, env.DB, env.Cfg)

	payload := []byte(`{"input":[]}`)
	req := httptest.NewRequest("POST", "/api/v1/openai/embeddings", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAPIOpenAI_Embeddings_NilEmbedder503(t *testing.T) {
	env := newAPITestEnv(t, nil)
	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, nil, env.DB, env.Cfg)

	payload := []byte(`{"input":"hi"}`)
	req := httptest.NewRequest("POST", "/api/v1/openai/embeddings", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestAPIOpenAI_ChatCompletions_NonStreaming(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)

	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	chatSvc := newChatSvcWithMock(t, env, "Hello from LLM")
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

	payload := []byte(`{
		"model":"ws",
		"messages":[
			{"role":"system","content":"Be helpful."},
			{"role":"user","content":"Hi there"}
		],
		"stream":false
	}`)
	req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "chat.completion", body.Object)
	assert.Equal(t, "ws", body.Model)
	require.Len(t, body.Choices, 1)
	assert.Equal(t, "assistant", body.Choices[0].Message.Role)
	assert.Equal(t, "Hello from LLM", body.Choices[0].Message.Content)
	assert.Equal(t, "stop", body.Choices[0].FinishReason)
}

func TestAPIOpenAI_ChatCompletions_UnknownModelReturns401(t *testing.T) {
	env := newAPITestEnv(t, nil)
	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	chatSvc := newChatSvcWithMock(t, env, "n/a")
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

	payload := []byte(`{"model":"ghost","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code) // Node parity: returns 401 on missing workspace
}

func TestAPIOpenAI_ChatCompletions_ThreadScopedModel(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	thread := &models.WorkspaceThread{Name: "t", Slug: "t1", WorkspaceID: ws.ID}
	require.NoError(t, env.DB.Create(thread).Error)

	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	chatSvc := newChatSvcWithMock(t, env, "thread-scoped")
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

	payload := []byte(`{"model":"ws:t1","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code) // succeeds with thread context
}

func TestAPIOpenAI_ChatCompletions_NoUserMessageReturns400(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	chatSvc := newChatSvcWithMock(t, env, "")
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

	// Last message is "assistant", not "user".
	payload := []byte(`{"model":"ws","messages":[{"role":"user","content":"q"},{"role":"assistant","content":"a"}]}`)
	req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPIOpenAI_ChatCompletions_Stream_EmitsDataFrames(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)

	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	chatSvc := newChatSvcWithMock(t, env, "Hello world")
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

	payload := []byte(`{"model":"ws","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Contains(t, body, "data: ")
	assert.Contains(t, body, `"object":"chat.completion.chunk"`)
	assert.Contains(t, body, `"delta":{"content":"Hello world"}`)
	assert.Contains(t, body, `"finish_reason":"stop"`)
	assert.Contains(t, body, "data: [DONE]")
	// No "event: " prefix anywhere — OpenAI clients reject named events.
	assert.NotContains(t, body, "event: ")
}

func TestAPIOpenAI_ChatCompletions_ModelSlugWithColons(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	// Thread slug contains a colon.
	thread := &models.WorkspaceThread{Name: "t", Slug: "t1:extra", WorkspaceID: ws.ID}
	require.NoError(t, env.DB.Create(thread).Error)

	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	chatSvc := newChatSvcWithMock(t, env, "thread-with-colon")
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

	payload := []byte(`{"model":"ws:t1:extra","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAPIOpenAI_VectorStores_Pagination(t *testing.T) {
	env := newAPITestEnv(t, nil)
	for i := 0; i < 5; i++ {
		require.NoError(t, env.DB.Create(&models.Workspace{Name: fmt.Sprintf("WS %d", i), Slug: fmt.Sprintf("ws-%d", i)}).Error)
	}
	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, nil, env.DB, env.Cfg)

	// Request limit=2.
	req := httptest.NewRequest("GET", "/api/v1/openai/vector_stores?limit=2", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Data    []any  `json:"data"`
		HasMore bool   `json:"has_more"`
		FirstID string `json:"first_id"`
		LastID  string `json:"last_id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data, 2)
	assert.True(t, body.HasMore)
	assert.Equal(t, "ws-0", body.FirstID)
	assert.Equal(t, "ws-1", body.LastID)

	// Request with after cursor.
	req2 := httptest.NewRequest("GET", "/api/v1/openai/vector_stores?limit=2&after=ws-1", nil)
	req2.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec2 := httptest.NewRecorder()
	env.Router.ServeHTTP(rec2, req2)

	require.Equal(t, http.StatusOK, rec2.Code)
	var body2 struct {
		Data    []any  `json:"data"`
		HasMore bool   `json:"has_more"`
		FirstID string `json:"first_id"`
		LastID  string `json:"last_id"`
	}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &body2))
	require.Len(t, body2.Data, 2)
	assert.True(t, body2.HasMore)
	assert.Equal(t, "ws-2", body2.FirstID)
	assert.Equal(t, "ws-3", body2.LastID)
}

func TestAPIOpenAI_ChatCompletions_NonStreaming_Usage(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)

	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	chatSvc := newChatSvcWithMock(t, env, "Hello from LLM")
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

	payload := []byte(`{
		"model":"ws",
		"messages":[
			{"role":"system","content":"Be helpful."},
			{"role":"user","content":"Hi there"}
		],
		"stream":false
	}`)
	req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Greater(t, body.Usage.PromptTokens, 0, "prompt_tokens should be > 0")
	assert.Greater(t, body.Usage.CompletionTokens, 0, "completion_tokens should be > 0")
	assert.Equal(t, body.Usage.PromptTokens+body.Usage.CompletionTokens, body.Usage.TotalTokens)
}

func TestAPIOpenAI_ChatCompletions_ImageURL(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)

	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	chatSvc := newChatSvcWithMock(t, env, "I see the image")
	api := env.Router.Group("/api")
	RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

	payload := []byte(`{
		"model":"ws",
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"Describe this"},
				{"type":"image_url","image_url":{"url":"https://example.com/img.png"}}
			]}
		],
		"stream":false
	}`)
	req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Choices, 1)
	assert.Equal(t, "I see the image", body.Choices[0].Message.Content)
}
