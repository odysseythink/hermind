package agent

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/stretchr/testify/assert"
)

func TestEngineSetConversationJudge(t *testing.T) {
	e := NewEngine(nil, nil, config.AgentConfig{}, "cli")
	j := NewLLMJudge(nil)
	e.SetConversationJudge(j)
	assert.NotNil(t, e.conversationJudge)
}

func TestEngineSetActiveMemoriesProvider(t *testing.T) {
	e := NewEngine(nil, nil, config.AgentConfig{}, "cli")
	called := false
	e.SetActiveMemoriesProvider(func(_ context.Context, _ string) []memprovider.InjectedMemory {
		called = true
		return nil
	})
	if e.activeMemories != nil {
		e.activeMemories(context.Background(), "q")
	}
	assert.True(t, called)
}
