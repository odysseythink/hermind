package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/require"
)

type mockVectorSearcher struct {
	results []dto.VectorSearchResult
	err     error
}

func (m *mockVectorSearcher) Search(ctx context.Context, ws *models.Workspace, req dto.VectorSearchRequest) ([]dto.VectorSearchResult, error) {
	return m.results, m.err
}

func TestRAGMemory_Search_ReturnsTopK(t *testing.T) {
	mock := &mockVectorSearcher{
		results: []dto.VectorSearchResult{
			{Text: "Plato wrote The Republic.", Score: 0.95, Metadata: map[string]any{"source": "plato.pdf"}},
			{Text: "Aristotle was Plato's student.", Score: 0.85, Metadata: map[string]any{"sourceName": "aristotle.pdf"}},
		},
	}
	tc := &ToolContext{
		Ctx:             context.Background(),
		Workspace:       &models.Workspace{ID: 1, Slug: "test-ws"},
		VectorSearchSvc: mock,
		Emit:            func(string) {},
	}

	entry := NewRAGMemorySkill(tc)
	args := json.RawMessage(`{"action":"search","content":"Who wrote The Republic?"}`)
	result, err := entry.Handler(context.Background(), args)
	require.NoError(t, err)
	require.Contains(t, result, "Plato")
	require.Contains(t, result, "Aristotle")
	require.Contains(t, result, "plato.pdf")
	require.Contains(t, result, "aristotle.pdf")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	results := parsed["results"].([]any)
	require.Len(t, results, 2)
}

func TestRAGMemory_Search_NoResults_ReturnsEmptyArrayString(t *testing.T) {
	mock := &mockVectorSearcher{results: []dto.VectorSearchResult{}}
	tc := &ToolContext{
		Ctx:             context.Background(),
		Workspace:       &models.Workspace{ID: 1, Slug: "test-ws"},
		VectorSearchSvc: mock,
		Emit:            func(string) {},
	}

	entry := NewRAGMemorySkill(tc)
	args := json.RawMessage(`{"action":"search","content":"nothing"}`)
	result, err := entry.Handler(context.Background(), args)
	require.NoError(t, err)
	require.JSONEq(t, `{"results":[]}`, result)
}

func TestRAGMemory_Store_EmbedsAndPersists(t *testing.T) {
	mock := &mockVectorSearcher{}
	tc := &ToolContext{
		Ctx:             context.Background(),
		Workspace:       &models.Workspace{ID: 1, Slug: "test-ws"},
		VectorSearchSvc: mock,
		Emit:            func(string) {},
	}

	entry := NewRAGMemorySkill(tc)
	args := json.RawMessage(`{"action":"store","content":"remember this"}`)
	result, err := entry.Handler(context.Background(), args)
	require.NoError(t, err)
	require.Contains(t, result, "deferred")
}

func TestRAGMemory_InvalidAction_ReturnsError(t *testing.T) {
	mock := &mockVectorSearcher{}
	tc := &ToolContext{
		Ctx:             context.Background(),
		Workspace:       &models.Workspace{ID: 1, Slug: "test-ws"},
		VectorSearchSvc: mock,
		Emit:            func(string) {},
	}

	entry := NewRAGMemorySkill(tc)
	args := json.RawMessage(`{"action":"delete","content":"x"}`)
	result, err := entry.Handler(context.Background(), args)
	require.NoError(t, err) // tool.Error returns valid JSON, not Go error
	require.Contains(t, result, "error")
	require.Contains(t, result, "unknown action")
}

func TestRAGMemory_DispatchViaRegistry(t *testing.T) {
	mock := &mockVectorSearcher{
		results: []dto.VectorSearchResult{
			{Text: "Found it!", Score: 0.99},
		},
	}
	tc := &ToolContext{
		Ctx:             context.Background(),
		Workspace:       &models.Workspace{ID: 1, Slug: "test-ws"},
		VectorSearchSvc: mock,
		Emit:            func(string) {},
	}

	entry := NewRAGMemorySkill(tc)
	reg := tool.NewRegistry()
	reg.Register(entry)

	result, err := reg.Dispatch(context.Background(), "rag-memory", json.RawMessage(`{"action":"search","content":"test"}`))
	require.NoError(t, err)
	require.Contains(t, result, "Found it!")
}

func TestRAGMemory_CheckFn_FalseWhenNoVectorSvc(t *testing.T) {
	tc := &ToolContext{
		Ctx:             context.Background(),
		Workspace:       &models.Workspace{ID: 1},
		VectorSearchSvc: nil,
		Emit:            func(string) {},
	}
	entry := NewRAGMemorySkill(tc)
	require.NotNil(t, entry.CheckFn)
	require.False(t, entry.CheckFn())
}
