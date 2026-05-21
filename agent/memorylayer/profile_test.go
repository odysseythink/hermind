package memorylayer

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// profileStubLLM implements core.LanguageModel for profile tests.
type profileStubLLM struct {
	resp *core.Response
	err  error
}

func (m *profileStubLLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}
func (m *profileStubLLM) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, m.err
}
func (m *profileStubLLM) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, m.err
}
func (m *profileStubLLM) Provider() string { return "stub" }
func (m *profileStubLLM) Model() string    { return "stub-model" }

func TestProfileUpdater_AddOnEmptyProfile(t *testing.T) {
	store, _ := sqlite.Open(":memory:")
	defer store.Close()
	_ = store.Migrate()
	ctx := context.Background()

	llm := &profileStubLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `{"adds":[{"kind":"explicit","key":"diet.restrictions","value":"peanuts","evidence":"I'm allergic to peanuts","source_turns":[1],"confidence":0.9}],"updates":[],"deletes":[]}`},
		}}}}
	up := NewProfileUpdater(store, llm, ProfileConfig{Enabled: true, Timeout: time.Second, DefaultUserID: "default"})
	up.Apply(ctx, &Boundary{Turns: []Turn{{ID: 1, UserMsg: "I'm allergic to peanuts"}}, Reason: "hard_turn"})

	p, err := store.GetProfile(ctx, "default")
	require.NoError(t, err)
	require.Len(t, p.Sections, 1)
	assert.Equal(t, "explicit", p.Sections[0].Kind)
	assert.Equal(t, "diet.restrictions", p.Sections[0].Key)
	assert.Equal(t, "peanuts", p.Sections[0].Value)

	events, _ := store.ListMemoryEvents(ctx, 10, 0, []string{"profile.updated"})
	require.Len(t, events, 1)
	assert.Contains(t, string(events[0].Data), `"adds":1`)
}

func TestProfileUpdater_UpdateExistingSection(t *testing.T) {
	store, _ := sqlite.Open(":memory:")
	defer store.Close()
	_ = store.Migrate()
	ctx := context.Background()

	// Seed one section.
	_, err := store.SaveProfileDelta(ctx, &storage.ProfileDelta{
		UserID: "default",
		Adds: []storage.ProfileSection{
			{Kind: "explicit", Key: "work.role", Value: "engineer", Confidence: 0.8},
		},
	})
	require.NoError(t, err)

	// LLM returns update referencing s1 but omits kind/key (common).
	llm := &profileStubLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `{"adds":[],"updates":[{"id":"s1","value":"senior engineer","evidence":"promoted","source_turns":[2],"confidence":0.95}],"deletes":[]}`},
		}}}}
	up := NewProfileUpdater(store, llm, ProfileConfig{Enabled: true, Timeout: time.Second, DefaultUserID: "default"})
	up.Apply(ctx, &Boundary{Turns: []Turn{{ID: 2, UserMsg: "I got promoted to senior engineer"}}, Reason: "hard_turn"})

	p, err := store.GetProfile(ctx, "default")
	require.NoError(t, err)
	require.Len(t, p.Sections, 1)
	assert.Equal(t, "senior engineer", p.Sections[0].Value)
	assert.Equal(t, "explicit", p.Sections[0].Kind)
	assert.Equal(t, "work.role", p.Sections[0].Key)
}

func TestProfileUpdater_DeleteExistingSection(t *testing.T) {
	store, _ := sqlite.Open(":memory:")
	defer store.Close()
	_ = store.Migrate()
	ctx := context.Background()

	_, err := store.SaveProfileDelta(ctx, &storage.ProfileDelta{
		UserID: "default",
		Adds: []storage.ProfileSection{
			{Kind: "explicit", Key: "temp.location", Value: "Paris", Confidence: 0.6},
		},
	})
	require.NoError(t, err)

	llm := &profileStubLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `{"adds":[],"updates":[],"deletes":[{"id":"s1"}]}`},
		}}}}
	up := NewProfileUpdater(store, llm, ProfileConfig{Enabled: true, Timeout: time.Second, DefaultUserID: "default"})
	up.Apply(ctx, &Boundary{Turns: []Turn{{ID: 3, UserMsg: "I left Paris"}}, Reason: "hard_turn"})

	p, err := store.GetProfile(ctx, "default")
	require.NoError(t, err)
	require.Len(t, p.Sections, 0)
}

func TestProfileUpdater_InvalidJSONIsNoOp(t *testing.T) {
	store, _ := sqlite.Open(":memory:")
	defer store.Close()
	_ = store.Migrate()
	ctx := context.Background()

	llm := &profileStubLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `this is not json`},
		}}}}
	up := NewProfileUpdater(store, llm, ProfileConfig{Enabled: true, Timeout: time.Second, DefaultUserID: "default"})
	up.Apply(ctx, &Boundary{Turns: []Turn{{ID: 1, UserMsg: "hello"}}, Reason: "hard_turn"})

	_, err := store.GetProfile(ctx, "default")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestProfileUpdater_EmptyDeltaIsNoOp(t *testing.T) {
	store, _ := sqlite.Open(":memory:")
	defer store.Close()
	_ = store.Migrate()
	ctx := context.Background()

	llm := &profileStubLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `{"adds":[],"updates":[],"deletes":[]}`},
		}}}}
	up := NewProfileUpdater(store, llm, ProfileConfig{Enabled: true, Timeout: time.Second, DefaultUserID: "default"})
	up.Apply(ctx, &Boundary{Turns: []Turn{{ID: 1, UserMsg: "hello"}}, Reason: "hard_turn"})

	_, err := store.GetProfile(ctx, "default")
	assert.ErrorIs(t, err, storage.ErrNotFound)

	events, _ := store.ListMemoryEvents(ctx, 10, 0, []string{"profile.updated"})
	assert.Len(t, events, 0)
}

func TestProfileUpdater_DisabledIsNoOp(t *testing.T) {
	store, _ := sqlite.Open(":memory:")
	defer store.Close()
	_ = store.Migrate()
	ctx := context.Background()

	llm := &profileStubLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `{"adds":[{"kind":"explicit","key":"x","value":"y"}],"updates":[],"deletes":[]}`},
		}}}}
	up := NewProfileUpdater(store, llm, ProfileConfig{Enabled: false, Timeout: time.Second})
	up.Apply(ctx, &Boundary{Turns: []Turn{{ID: 1, UserMsg: "hello"}}, Reason: "hard_turn"})

	_, err := store.GetProfile(ctx, "default")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestProfileUpdater_NilBoundary(t *testing.T) {
	store, _ := sqlite.Open(":memory:")
	defer store.Close()
	_ = store.Migrate()
	ctx := context.Background()

	llm := &profileStubLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `{"adds":[{"kind":"explicit","key":"x","value":"y"}],"updates":[],"deletes":[]}`},
		}}}}
	up := NewProfileUpdater(store, llm, ProfileConfig{Enabled: true, Timeout: time.Second})
	up.Apply(ctx, nil)

	_, err := store.GetProfile(ctx, "default")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestParseProfileDelta_MarkdownWrapping(t *testing.T) {
	text := `Some preamble before JSON
{"adds":[{"kind":"explicit","key":"foo","value":"bar"}],"updates":[],"deletes":[]}
Some trailing text`
	delta := parseProfileDelta(text, nil, "u")
	require.NotNil(t, delta)
	require.Len(t, delta.Adds, 1)
	assert.Equal(t, "foo", delta.Adds[0].Key)
}

func TestRenderProfilePrompt_MaxSections(t *testing.T) {
	cur := &storage.Profile{
		Sections: []storage.ProfileSection{
			{Kind: "explicit", Key: "a", Value: "1", Confidence: 0.5},
			{Kind: "explicit", Key: "b", Value: "2", Confidence: 0.6},
			{Kind: "explicit", Key: "c", Value: "3", Confidence: 0.7},
		},
	}
	b := &Boundary{Turns: []Turn{{ID: 1, UserMsg: "hi", Assistant: "hello"}}}
	p := renderProfilePrompt(cur, b, 2)
	assert.Contains(t, p, "s1")
	assert.Contains(t, p, "s2")
	assert.NotContains(t, p, "s3")
}

func TestRenderProfilePrompt_EmptyProfile(t *testing.T) {
	b := &Boundary{Turns: []Turn{{ID: 1, UserMsg: "hi", Assistant: "hello"}}}
	p := renderProfilePrompt(nil, b, 24)
	assert.Contains(t, p, "(empty)")
}

func TestIsValidKind(t *testing.T) {
	assert.True(t, isValidKind("explicit"))
	assert.True(t, isValidKind("implicit"))
	assert.False(t, isValidKind("other"))
	assert.False(t, isValidKind(""))
}

func TestProfileUpdater_FallbackUpdateWithoutID(t *testing.T) {
	// When the LLM emits an update without a short ID but with valid kind+key,
	// parseProfileDelta should accept it as a fallback.
	store, _ := sqlite.Open(":memory:")
	defer store.Close()
	_ = store.Migrate()
	ctx := context.Background()

	_, err := store.SaveProfileDelta(ctx, &storage.ProfileDelta{
		UserID: "default",
		Adds: []storage.ProfileSection{
			{Kind: "explicit", Key: "color", Value: "blue", Confidence: 0.8},
		},
	})
	require.NoError(t, err)

	// LLM omits the short ID but provides kind+key directly.
	llm := &profileStubLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `{"adds":[],"updates":[{"kind":"explicit","key":"color","value":"red","evidence":"changed","confidence":0.9}],"deletes":[]}`},
		}}}}
	up := NewProfileUpdater(store, llm, ProfileConfig{Enabled: true, Timeout: time.Second, DefaultUserID: "default"})
	up.Apply(ctx, &Boundary{Turns: []Turn{{ID: 1, UserMsg: "I prefer red now"}}, Reason: "hard_turn"})

	p, err := store.GetProfile(ctx, "default")
	require.NoError(t, err)
	require.Len(t, p.Sections, 1)
	assert.Equal(t, "red", p.Sections[0].Value)
}
