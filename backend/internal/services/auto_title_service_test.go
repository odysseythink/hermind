package services

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAutoTitleDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Workspace{}, &models.WorkspaceThread{}, &models.WorkspaceChat{}))
	// Limit to single connection so in-memory SQLite is shared across goroutines.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	return db
}

type mockLLMForTitle struct {
	lastPrompt string
	returnText string
}

func (m *mockLLMForTitle) Stream(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (<-chan providers.LLMChunk, error) {
	return nil, nil
}

func (m *mockLLMForTitle) Complete(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (string, error) {
	if len(messages) > 0 && len(messages[0].Content) > 0 {
		if tp, ok := messages[0].Content[0].(core.TextPart); ok {
			m.lastPrompt = tp.Text
		}
	}
	return m.returnText, nil
}

func (m *mockLLMForTitle) LanguageModel() core.LanguageModel { return nil }

func TestAutoTitleService_MaybeGenerate_SkipsWhenCustomTitleExists(t *testing.T) {
	db := setupAutoTitleDB(t)
	mockLLM := &mockLLMForTitle{returnText: "Should Not Run"}
	svc := NewAutoTitleService(db, mockLLM)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	thread := &models.WorkspaceThread{
		Name:        "Custom Title",
		Slug:        "custom-slug",
		WorkspaceID: ws.ID,
	}
	require.NoError(t, db.Create(thread).Error)

	// Insert one chat so it would otherwise qualify.
	require.NoError(t, db.Create(&models.WorkspaceChat{
		WorkspaceID: ws.ID,
		ThreadID:    &thread.ID,
		Prompt:      "hello",
		Response:    `{"text":"hi"}`,
		Include:     true,
	}).Error)

	svc.MaybeGenerate(context.Background(), thread.ID, "hello", "hi")

	// Verify title was NOT changed.
	var updated models.WorkspaceThread
	require.NoError(t, db.First(&updated, thread.ID).Error)
	assert.Equal(t, "Custom Title", updated.Name)
	assert.Empty(t, mockLLM.lastPrompt)
}

func TestAutoTitleService_MaybeGenerate_SkipsAfterSecondExchange(t *testing.T) {
	db := setupAutoTitleDB(t)
	mockLLM := &mockLLMForTitle{returnText: "Should Not Run"}
	svc := NewAutoTitleService(db, mockLLM)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	thread := &models.WorkspaceThread{
		Name:        "Thread",
		Slug:        "thread-slug",
		WorkspaceID: ws.ID,
	}
	require.NoError(t, db.Create(thread).Error)

	// Insert 3 user chats (>2 threshold).
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&models.WorkspaceChat{
			WorkspaceID: ws.ID,
			ThreadID:    &thread.ID,
			Prompt:      "msg",
			Response:    `{"text":"resp"}`,
			Include:     true,
		}).Error)
	}

	svc.MaybeGenerate(context.Background(), thread.ID, "third", "resp")

	var updated models.WorkspaceThread
	require.NoError(t, db.First(&updated, thread.ID).Error)
	assert.Equal(t, "Thread", updated.Name)
	assert.Empty(t, mockLLM.lastPrompt)
}

func TestAutoTitleService_MaybeGenerate_GeneratesOnFirstExchange(t *testing.T) {
	db := setupAutoTitleDB(t)
	mockLLM := &mockLLMForTitle{returnText: "Kubernetes Deployment Guide"}
	svc := NewAutoTitleService(db, mockLLM)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	thread := &models.WorkspaceThread{
		Name:        "Thread",
		Slug:        "thread-slug",
		WorkspaceID: ws.ID,
	}
	require.NoError(t, db.Create(thread).Error)

	// Insert 1 user chat (≤2 threshold).
	require.NoError(t, db.Create(&models.WorkspaceChat{
		WorkspaceID: ws.ID,
		ThreadID:    &thread.ID,
		Prompt:      "how do I deploy to k8s?",
		Response:    `{"text":"use kubectl apply"}`,
		Include:     true,
	}).Error)

	svc.MaybeGenerate(context.Background(), thread.ID, "how do I deploy to k8s?", "use kubectl apply")

	// Wait for goroutine to finish (small sleep is acceptable in integration-style tests).
	// In production this is fire-and-forget.
	// For the test we can poll the DB.
	require.Eventually(t, func() bool {
		var updated models.WorkspaceThread
		db.First(&updated, thread.ID)
		return updated.Name == "Kubernetes Deployment Guide"
	}, 2*time.Second, 100*time.Millisecond)

	assert.Contains(t, mockLLM.lastPrompt, "how do I deploy to k8s?")
	assert.Contains(t, mockLLM.lastPrompt, "use kubectl apply")
}

func TestSanitizeTitle(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"  Hello World  ", "Hello World"},
		{`"Quoted Title"`, "Quoted Title"},
		{`'Single Quotes'`, "Single Quotes"},
		{"Title: My Title", "My Title"},
		{"title: lowercase", "lowercase"},
		{
			"This is a very long title that should be truncated because it exceeds the maximum allowed length",
			"This is a very long title that should be truncated because it exceeds the max...",
		},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			assert.Equal(t, c.expected, sanitizeTitle(c.input))
		})
	}
}

func TestBuildTitlePrompt_TruncatesLongInputs(t *testing.T) {
	longUser := make([]byte, 600)
	for i := range longUser {
		longUser[i] = 'a'
	}
	longResp := make([]byte, 600)
	for i := range longResp {
		longResp[i] = 'b'
	}

	prompt := buildTitlePrompt(string(longUser), string(longResp))
	assert.Contains(t, prompt, "aaa...")
	assert.Contains(t, prompt, "bbb...")
	assert.NotContains(t, prompt, string(longUser))
	assert.NotContains(t, prompt, string(longResp))
}
