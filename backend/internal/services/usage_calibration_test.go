package services

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatService_UsageCache_RoundTrip(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	svc := NewChatService(db, cfg, NewVectorService(cfg), nil, nil, nil, nil, nil, nil, nil, nil)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	// Record usage for (ws, nil)
	svc.recordUsage(ws.ID, nil, core.Usage{PromptTokens: 1500})

	usage, ok := svc.getLastPromptTokens(ws.ID, nil)
	require.True(t, ok)
	assert.Equal(t, 1500, usage)

	// Different workspace should not exist
	_, ok = svc.getLastPromptTokens(999, nil)
	assert.False(t, ok)
}

func TestChatService_UsageCache_ThreadIsolation(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	svc := NewChatService(db, cfg, NewVectorService(cfg), nil, nil, nil, nil, nil, nil, nil, nil)

	threadID := 42
	svc.recordUsage(1, &threadID, core.Usage{PromptTokens: 2000})
	svc.recordUsage(1, nil, core.Usage{PromptTokens: 1000})

	u1, ok1 := svc.getLastPromptTokens(1, &threadID)
	u2, ok2 := svc.getLastPromptTokens(1, nil)

	require.True(t, ok1)
	require.True(t, ok2)
	assert.Equal(t, 2000, u1)
	assert.Equal(t, 1000, u2)
}
