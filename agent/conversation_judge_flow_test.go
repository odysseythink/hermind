package agent

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recStore struct {
	bumps []struct {
		ID   string
		Used bool
	}
}

func (r *recStore) BumpMemoryUsage(_ context.Context, id string, used bool) error {
	r.bumps = append(r.bumps, struct {
		ID   string
		Used bool
	}{id, used})
	return nil
}
func (r *recStore) AppendMessage(_ context.Context, _ *storage.StoredMessage) error { return nil }
func (r *recStore) GetHistory(_ context.Context, _, _ int) ([]*storage.StoredMessage, error) {
	return nil, nil
}
func (r *recStore) SearchMessages(_ context.Context, _ string, _ *storage.SearchOptions) ([]*storage.SearchResult, error) {
	return nil, nil
}
func (r *recStore) UpdateSystemPromptCache(_ context.Context, _ string) error { return nil }
func (r *recStore) UpdateUsage(_ context.Context, _ *storage.UsageUpdate) error { return nil }
func (r *recStore) SaveMemory(_ context.Context, _ *storage.Memory) error      { return nil }
func (r *recStore) GetMemory(_ context.Context, _ string) (*storage.Memory, error) {
	return nil, storage.ErrNotFound
}
func (r *recStore) SearchMemories(_ context.Context, _ string, _ *storage.MemorySearchOptions) ([]*storage.Memory, error) {
	return nil, nil
}
func (r *recStore) DeleteMemory(_ context.Context, _ string) error { return nil }
func (r *recStore) ListMemoriesByType(_ context.Context, _ string, _ int) ([]*storage.Memory, error) {
	return nil, nil
}
func (r *recStore) MarkMemorySuperseded(_ context.Context, _, _ string) error { return nil }
func (r *recStore) WithTx(ctx context.Context, fn func(storage.Tx) error) error { return fn(nopTx{}) }
func (r *recStore) Close() error   { return nil }
func (r *recStore) Migrate() error { return nil }

type nopTx struct{}

func (nopTx) AppendMessage(_ context.Context, _ *storage.StoredMessage) error { return nil }
func (nopTx) UpdateUsage(_ context.Context, _ *storage.UsageUpdate) error     { return nil }
func (nopTx) UpdateSystemPromptCache(_ context.Context, _ string) error       { return nil }

type flowStubProvider struct {
	reply string
}

func (p *flowStubProvider) Name() string { return "stub" }
func (p *flowStubProvider) Available() bool { return true }
func (p *flowStubProvider) Complete(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	return nil, nil
}
func (p *flowStubProvider) Stream(_ context.Context, _ *provider.Request) (provider.Stream, error) {
	return &flowStubStream{reply: p.reply}, nil
}
func (p *flowStubProvider) ModelInfo(_ string) *provider.ModelInfo { return nil }
func (p *flowStubProvider) EstimateTokens(_ string, _ string) (int, error) { return 0, nil }

type flowStubStream struct {
	reply string
	done  bool
}

func (s *flowStubStream) Recv() (*provider.StreamEvent, error) {
	if s.done {
		return nil, nil
	}
	s.done = true
	return &provider.StreamEvent{
		Type: provider.EventDone,
		Response: &provider.Response{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: message.TextContent(s.reply),
			},
		},
	}, nil
}
func (s *flowStubStream) Close() error { return nil }

type fixedJudge struct{ v *Verdict }

func (f *fixedJudge) Run(_ context.Context, _ JudgeInput) (*Verdict, error) { return f.v, nil }

func TestRunConversation_JudgeDispatchBumpsUsage(t *testing.T) {
	store := &recStore{}
	p := &flowStubProvider{reply: "here is alpha fact done"}
	eng := NewEngineWithToolsAndAux(p, nil, store, nil, config.AgentConfig{MaxTurns: 2}, "cli")
	eng.SetActiveMemoriesProvider(func(_ context.Context, _ string) []memprovider.InjectedMemory {
		return []memprovider.InjectedMemory{
			{ID: "a", Content: "alpha fact"},
			{ID: "b", Content: "beta fact"},
		}
	})
	eng.SetConversationJudge(&fixedJudge{v: &Verdict{
		Outcome:      "success",
		MemoriesUsed: []string{"b"},
	}})

	_, err := eng.RunConversation(context.Background(), &RunOptions{UserMessage: "hi"})
	require.NoError(t, err)

	bumps := map[string]bool{}
	for _, b := range store.bumps {
		bumps[b.ID] = b.Used
	}
	assert.True(t, bumps["a"], "memory a matched by substring")
	assert.True(t, bumps["b"], "memory b matched by verdict")
}

func TestRunConversation_NoJudgeSkipsBumps(t *testing.T) {
	store := &recStore{}
	p := &flowStubProvider{reply: "reply"}
	eng := NewEngineWithToolsAndAux(p, nil, store, nil, config.AgentConfig{MaxTurns: 2}, "cli")
	eng.SetActiveMemoriesProvider(func(_ context.Context, _ string) []memprovider.InjectedMemory {
		return []memprovider.InjectedMemory{{ID: "x", Content: "xfact"}}
	})
	// No SetConversationJudge.

	_, err := eng.RunConversation(context.Background(), &RunOptions{UserMessage: "hi"})
	require.NoError(t, err)
	assert.Empty(t, store.bumps, "no judge → no feedback calls")
}
