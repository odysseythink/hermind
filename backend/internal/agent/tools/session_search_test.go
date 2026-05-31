package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockChatSearcher struct {
	results []models.WorkspaceChat
}

func (m *mockChatSearcher) SearchWorkspaceChatsFTS5(ctx context.Context, workspaceID int, query string, limit int) ([]models.WorkspaceChat, error) {
	return m.results, nil
}

func TestSessionSearchSkill_Execute(t *testing.T) {
	tc := &ToolContext{
		Workspace: &models.Workspace{ID: 1, Slug: "test-ws"},
		Emit:      func(string) {},
	}
	mock := &mockChatSearcher{
		results: []models.WorkspaceChat{
			{ID: 1, Prompt: "How to deploy?", Response: `{"text":"Use docker"}`, CreatedAt: time.Now()},
		},
	}

	skill := NewSessionSearchSkill(tc, mock)
	result, err := skill.Handler(context.Background(), json.RawMessage(`{"query":"deploy","limit":5}`))
	require.NoError(t, err)
	assert.NotContains(t, result, `"error"`)
	assert.Contains(t, result, "How to deploy?")
}

func TestSessionSearchSkill_EmptyQuery(t *testing.T) {
	tc := &ToolContext{Workspace: &models.Workspace{ID: 1}}
	mock := &mockChatSearcher{}
	skill := NewSessionSearchSkill(tc, mock)
	result, err := skill.Handler(context.Background(), json.RawMessage(`{"query":""}`))
	require.NoError(t, err)
	assert.Contains(t, result, `"error"`)
	assert.Contains(t, result, "query is required")
}
