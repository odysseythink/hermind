package services

import (
	"context"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/reranker"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
)

func TestMemoryInjector_NoMemories_ReturnsBase(t *testing.T) {
	memSvc := NewMemoryService(newMemTestDB(t))
	inj := NewMemoryInjector(memSvc, &fakeSettings{enabled: true}, &reranker.NoopReranker{})

	out := inj.PromptWithMemories(context.Background(), "base prompt", nil, 1, "q", nil)
	assert.Equal(t, "base prompt", out)
}

func TestMemoryInjector_DisabledReturnsBase(t *testing.T) {
	db := newMemTestDB(t)
	memSvc := NewMemoryService(db)
	uid := 1
	_, _ = memSvc.Create(context.Background(), &uid, intPtr(1), models.MemoryScopeWorkspace, "x")

	inj := NewMemoryInjector(memSvc, &fakeSettings{enabled: false}, &reranker.NoopReranker{})
	out := inj.PromptWithMemories(context.Background(), "base", &uid, 1, "q", nil)
	assert.Equal(t, "base", out)
}

func TestMemoryInjector_EnabledRenders(t *testing.T) {
	db := newMemTestDB(t)
	memSvc := NewMemoryService(db)
	uid := 1
	_, _ = memSvc.Create(context.Background(), &uid, intPtr(1), models.MemoryScopeWorkspace, "ws fact")
	_, _ = memSvc.Create(context.Background(), &uid, nil, models.MemoryScopeGlobal, "global fact")

	inj := NewMemoryInjector(memSvc, &fakeSettings{enabled: true}, &reranker.NoopReranker{})
	out := inj.PromptWithMemories(context.Background(), "base", &uid, 1, "q", nil)
	assert.True(t, strings.Contains(out, "## Things I Remember About You"))
	assert.True(t, strings.Contains(out, "- global fact"))
	assert.True(t, strings.Contains(out, "- ws fact"))
}

func TestMemoryInjector_RerankCapsAtMaxInjected(t *testing.T) {
	db := newMemTestDB(t)
	memSvc := NewMemoryService(db)
	uid := 1
	for i := 0; i < models.MaxInjectedWorkspaceLimit+3; i++ {
		_, _ = memSvc.Create(context.Background(), &uid, intPtr(1), models.MemoryScopeWorkspace, "ws"+string(rune('a'+i)))
	}
	inj := NewMemoryInjector(memSvc, &fakeSettings{enabled: true}, &reranker.NoopReranker{})
	out := inj.PromptWithMemories(context.Background(), "base", &uid, 1, "q", []core.Message{})
	bullets := strings.Count(out, "\n- ")
	assert.Equal(t, models.MaxInjectedWorkspaceLimit, bullets)
}

type fakeSettings struct{ enabled bool }

func (f *fakeSettings) MemoriesEnabled(ctx context.Context) bool { return f.enabled }

func intPtr(i int) *int { return &i }
