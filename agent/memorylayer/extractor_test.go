package memorylayer

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaxonomyExtractor_Disabled(t *testing.T) {
	e := NewTaxonomyExtractor(&mockLLM{}, TaxonomyConfig{Enabled: false})
	mems, err := e.Extract(context.Background(), &Boundary{Turns: []Turn{{ID: 1}}})
	require.NoError(t, err)
	assert.Nil(t, mems)
}

func TestTaxonomyExtractor_HappyPath4Types(t *testing.T) {
	resp := `[{"type":"core","content":"allergic to peanuts","confidence":0.9},{"type":"episode","content":"discussed migration","confidence":0.8},{"type":"fact","content":"uses pnpm","confidence":0.95},{"type":"foresight","content":"deadline Friday","confidence":0.7,"expires_at":"2026-12-31T23:59:59Z"}]`
	llm := &mockLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: resp},
	}}}}
	e := NewTaxonomyExtractor(llm, TaxonomyConfig{Enabled: true, MaxOutputs: 8, Timeout: time.Second})
	mems, err := e.Extract(context.Background(), &Boundary{Turns: []Turn{{ID: 42, UserMsg: "hi", Assistant: "hello"}}})
	require.NoError(t, err)
	require.Len(t, mems, 4)

	types := make(map[string]bool)
	for _, m := range mems {
		types[m.MemType] = true
		assert.Equal(t, int64(42), m.ParentTurnID)
	}
	assert.True(t, types["core"])
	assert.True(t, types["episode"])
	assert.True(t, types["fact"])
	assert.True(t, types["foresight"])

	// foresight should have expires_at
	for _, m := range mems {
		if m.MemType == "foresight" {
			assert.False(t, m.ExpiresAt.IsZero())
		}
	}
}

func TestTaxonomyExtractor_DefaultsForesightExpiry(t *testing.T) {
	resp := `[{"type":"foresight","content":"plan X","confidence":0.7}]`
	llm := &mockLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: resp},
	}}}}
	e := NewTaxonomyExtractor(llm, TaxonomyConfig{Enabled: true, MaxOutputs: 8, Timeout: time.Second})
	mems, err := e.Extract(context.Background(), &Boundary{Turns: []Turn{{ID: 1}}})
	require.NoError(t, err)
	require.Len(t, mems, 1)
	assert.Equal(t, "foresight", mems[0].MemType)
	assert.WithinDuration(t, time.Now().UTC().AddDate(0, 0, 7), mems[0].ExpiresAt, 24*time.Hour)
}

func TestTaxonomyExtractor_RejectsUnknownType(t *testing.T) {
	resp := `[{"type":"skill","content":"should be skipped","confidence":0.5}]`
	llm := &mockLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: resp},
	}}}}
	e := NewTaxonomyExtractor(llm, TaxonomyConfig{Enabled: true, MaxOutputs: 8, Timeout: time.Second})
	mems, err := e.Extract(context.Background(), &Boundary{Turns: []Turn{{ID: 1}}})
	require.NoError(t, err)
	assert.Len(t, mems, 0)
}

func TestTaxonomyExtractor_RespectsMaxOutputs(t *testing.T) {
	var items []map[string]any
	for i := 0; i < 20; i++ {
		items = append(items, map[string]any{"type": "fact", "content": fmt.Sprintf("fact %d", i), "confidence": 0.5})
	}
	raw, _ := json.Marshal(items)
	llm := &mockLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: string(raw)},
	}}}}
	e := NewTaxonomyExtractor(llm, TaxonomyConfig{Enabled: true, MaxOutputs: 8, Timeout: time.Second})
	mems, err := e.Extract(context.Background(), &Boundary{Turns: []Turn{{ID: 1}}})
	require.NoError(t, err)
	assert.Len(t, mems, 8)
}

func TestTaxonomyExtractor_BadJSONReturnsEmpty(t *testing.T) {
	llm := &mockLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: "not json"},
	}}}}
	e := NewTaxonomyExtractor(llm, TaxonomyConfig{Enabled: true, MaxOutputs: 8, Timeout: time.Second})
	mems, err := e.Extract(context.Background(), &Boundary{Turns: []Turn{{ID: 1}}})
	require.NoError(t, err)
	assert.Len(t, mems, 0)
}

func TestTaxonomyExtractor_Timeout(t *testing.T) {
	llm := &mockLLM{sleepFor: 200 * time.Millisecond}
	e := NewTaxonomyExtractor(llm, TaxonomyConfig{Enabled: true, MaxOutputs: 8, Timeout: 50 * time.Millisecond})
	_, err := e.Extract(context.Background(), &Boundary{Turns: []Turn{{ID: 1}}})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
