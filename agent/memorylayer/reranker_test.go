package memorylayer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLLM struct {
	resp      *core.Response
	err       error
	sleepFor  time.Duration
}

func (m *mockLLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	if m.sleepFor > 0 {
		select {
		case <-time.After(m.sleepFor):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func (m *mockLLM) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockLLM) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockLLM) Provider() string { return "mock" }
func (m *mockLLM) Model() string    { return "mock-model" }

func TestLLMReranker_Disabled(t *testing.T) {
	r := NewLLMReranker(&mockLLM{}, RerankerConfig{Enabled: false})
	cands := []Candidate{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	out, err := r.Rerank(context.Background(), "q", cands, 2)
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "a", out[0].ID)
}

func TestLLMReranker_HappyPath(t *testing.T) {
	llm := &mockLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `["m_2","m_3","m_1"]`},
	}}}}
	r := NewLLMReranker(llm, RerankerConfig{Enabled: true, BatchSize: 20, Timeout: time.Second})
	cands := []Candidate{{ID: "m_1"}, {ID: "m_2"}, {ID: "m_3"}}
	out, err := r.Rerank(context.Background(), "q", cands, 2)
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "m_2", out[0].ID)
	assert.Equal(t, "m_3", out[1].ID)
}

func TestLLMReranker_LLMFails(t *testing.T) {
	llm := &mockLLM{err: errors.New("model down")}
	r := NewLLMReranker(llm, RerankerConfig{Enabled: true, BatchSize: 20, Timeout: time.Second})
	cands := []Candidate{{ID: "a"}, {ID: "b"}}
	out, err := r.Rerank(context.Background(), "q", cands, 2)
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "a", out[0].ID)
}

func TestLLMReranker_LLMDropsCandidates(t *testing.T) {
	llm := &mockLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `["m_2"]`},
	}}}}
	r := NewLLMReranker(llm, RerankerConfig{Enabled: true, BatchSize: 20, Timeout: time.Second})
	cands := []Candidate{{ID: "m_1"}, {ID: "m_2"}, {ID: "m_3"}}
	out, err := r.Rerank(context.Background(), "q", cands, 3)
	require.NoError(t, err)
	require.Len(t, out, 3)
	assert.Equal(t, "m_2", out[0].ID)
	// dropped IDs appended at the end
	assert.Equal(t, "m_1", out[1].ID)
	assert.Equal(t, "m_3", out[2].ID)
}

func TestLLMReranker_Timeout(t *testing.T) {
	llm := &mockLLM{sleepFor: 200 * time.Millisecond}
	r := NewLLMReranker(llm, RerankerConfig{Enabled: true, BatchSize: 20, Timeout: 50 * time.Millisecond})
	cands := []Candidate{{ID: "a"}, {ID: "b"}}
	out, err := r.Rerank(context.Background(), "q", cands, 2)
	require.NoError(t, err)
	require.Len(t, out, 2)
}

func TestParseRerankIDs_Robust(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{`["a","b"]`, []string{"a", "b"}},
		{"```json\n[\"a\",\"b\"]\n```", []string{"a", "b"}},
		{"some text before [\"a\",\"b\"] after", []string{"a", "b"}},
		{"garbage", nil},
		{"", nil},
	}
	for _, tc := range cases {
		got := parseRerankIDs(tc.input)
		assert.Equal(t, tc.want, got, "input: %q", tc.input)
	}
}
