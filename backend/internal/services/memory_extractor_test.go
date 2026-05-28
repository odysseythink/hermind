package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockExtractLLM struct {
	observerResp  string // raw JSON for tool-call arguments
	reflectorResp string
	callCount     int
}

func (m *mockExtractLLM) Generate(_ context.Context, req *core.Request) (*core.Response, error) {
	m.callCount++
	if len(req.Tools) == 0 {
		return &core.Response{}, nil
	}
	switch req.Tools[0].Name {
	case "extract_candidate_facts":
		return &core.Response{
			Message: core.Message{
				Role: core.MESSAGE_ROLE_ASSISTANT,
				Content: []core.ContentParter{
					core.ToolCallPart{ID: "1", Name: "extract_candidate_facts", Arguments: m.observerResp},
				},
			},
		}, nil
	case "decide_memory_actions":
		return &core.Response{
			Message: core.Message{
				Role: core.MESSAGE_ROLE_ASSISTANT,
				Content: []core.ContentParter{
					core.ToolCallPart{ID: "2", Name: "decide_memory_actions", Arguments: m.reflectorResp},
				},
			},
		}, nil
	}
	return &core.Response{}, nil
}

func TestMemoryExtractor_RoundTrip(t *testing.T) {
	db := newMemTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.WorkspaceChat{}, &models.Workspace{}))
	memSvc := NewMemoryService(db)
	uid, wid := 1, 1
	chats := []models.WorkspaceChat{
		{WorkspaceID: wid, UserID: &uid, Prompt: "I'm a Go dev", Response: "noted"},
		{WorkspaceID: wid, UserID: &uid, Prompt: "I work at Acme", Response: "noted"},
	}
	for i := range chats {
		require.NoError(t, db.Create(&chats[i]).Error)
	}

	llm := &mockExtractLLM{
		observerResp:  `{"facts":[{"content":"User is a Go developer","confidence":0.9,"reasoning":"explicit"}]}`,
		reflectorResp: `{"memories":[{"content":"User is a Go developer","scope":"GLOBAL","action":"create","reasoning":"durable"}]}`,
	}
	ext := NewMemoryExtractor(memSvc, llm, "obs prompt {{CONVERSATION}}", "ref prompt {{CANDIDATES}}")

	err := ext.ProcessGroup(context.Background(), &uid, wid, chats)
	require.NoError(t, err)

	globals, _ := memSvc.ListGlobal(context.Background(), &uid)
	require.Len(t, globals, 1)
	assert.Equal(t, "User is a Go developer", globals[0].Content)
}
