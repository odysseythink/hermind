package web

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/odysseythink/hermind/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchCache_GetMissReturnsZero(t *testing.T) {
	c := newSearchCache(4, time.Second)
	v, ok := c.Get("missing")
	assert.False(t, ok)
	assert.Nil(t, v)
}

func TestSearchCache_SetThenGetReturnsValue(t *testing.T) {
	c := newSearchCache(4, time.Second)
	results := []SearchResult{{Title: "A", URL: "https://a"}}
	c.Set("k1", results)
	v, ok := c.Get("k1")
	assert.True(t, ok)
	assert.Equal(t, results, v)
}

func TestSearchCache_ExpiryRemovesEntry(t *testing.T) {
	c := newSearchCache(4, 10*time.Millisecond)
	c.Set("k1", []SearchResult{{Title: "A"}})
	time.Sleep(20 * time.Millisecond)
	v, ok := c.Get("k1")
	assert.False(t, ok)
	assert.Nil(t, v)
}

func TestSearchCache_LRUEviction(t *testing.T) {
	c := newSearchCache(2, time.Second)
	c.Set("k1", []SearchResult{{Title: "A"}})
	c.Set("k2", []SearchResult{{Title: "B"}})
	_, _ = c.Get("k1")
	c.Set("k3", []SearchResult{{Title: "C"}})

	_, ok := c.Get("k1")
	assert.True(t, ok, "k1 should remain (was MRU)")
	_, ok = c.Get("k2")
	assert.False(t, ok, "k2 should be evicted (was LRU)")
	_, ok = c.Get("k3")
	assert.True(t, ok, "k3 should remain")
}

func TestSearchCache_ConcurrentAccess(t *testing.T) {
	c := newSearchCache(100, time.Second)
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 50; j++ {
				key := string(rune('a' + (id+j)%26))
				c.Set(key, []SearchResult{{Title: key}})
				_, _ = c.Get(key)
			}
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// fakeProvider is a stub SearchProvider used to test dispatcher logic
// without hitting the network.
type fakeProvider struct {
	id         string
	configured bool
	results    []SearchResult
	err        error
	calls      int
}

func (f *fakeProvider) ID() string       { return f.id }
func (f *fakeProvider) Configured() bool { return f.configured }
func (f *fakeProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

func dispatcherWith(providers map[string]SearchProvider, explicit string) *searchDispatcher {
	return &searchDispatcher{
		providers: providers,
		explicit:  explicit,
		cache:     newSearchCache(8, time.Minute),
	}
}

func TestDispatcher_ExplicitProviderWins(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "")
	t.Setenv("BRAVE_API_KEY", "")
	t.Setenv("EXA_API_KEY", "")
	providers := map[string]SearchProvider{
		"tavily":     &fakeProvider{id: "tavily", configured: true},
		"brave":      &fakeProvider{id: "brave", configured: true},
		"exa":        &fakeProvider{id: "exa", configured: true},
		"searxng":    &fakeProvider{id: "searxng", configured: true},
		"bing":       &fakeProvider{id: "bing", configured: true},
		"DuckDuckGo": &fakeProvider{id: "DuckDuckGo", configured: true},
	}
	d := dispatcherWith(providers, "brave")
	p, err := d.resolveProvider()
	require.NoError(t, err)
	assert.Equal(t, "brave", p.ID())
}

func TestDispatcher_ExplicitUnknownErrors(t *testing.T) {
	providers := map[string]SearchProvider{
		"DuckDuckGo": &fakeProvider{id: "DuckDuckGo", configured: true},
	}
	d := dispatcherWith(providers, "bogus")
	_, err := d.resolveProvider()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestDispatcher_ExplicitUnconfiguredErrors(t *testing.T) {
	providers := map[string]SearchProvider{
		"tavily":     &fakeProvider{id: "tavily", configured: false},
		"DuckDuckGo": &fakeProvider{id: "DuckDuckGo", configured: true},
	}
	d := dispatcherWith(providers, "tavily")
	_, err := d.resolveProvider()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestDispatcher_AutoPriority(t *testing.T) {
	providers := map[string]SearchProvider{
		"tavily":     &fakeProvider{id: "tavily", configured: false},
		"brave":      &fakeProvider{id: "brave", configured: false},
		"exa":        &fakeProvider{id: "exa", configured: false},
		"searxng":    &fakeProvider{id: "searxng", configured: true},
		"bing":       &fakeProvider{id: "bing", configured: true},
		"DuckDuckGo": &fakeProvider{id: "DuckDuckGo", configured: true},
	}
	d := dispatcherWith(providers, "")
	p, err := d.resolveProvider()
	require.NoError(t, err)
	assert.Equal(t, "searxng", p.ID())
}

func TestDispatcher_AutoFallsBackToDDG(t *testing.T) {
	providers := map[string]SearchProvider{
		"tavily":     &fakeProvider{id: "tavily", configured: false},
		"brave":      &fakeProvider{id: "brave", configured: false},
		"exa":        &fakeProvider{id: "exa", configured: false},
		"searxng":    &fakeProvider{id: "searxng", configured: false},
		"bing":       &fakeProvider{id: "bing", configured: false},
		"DuckDuckGo": &fakeProvider{id: "DuckDuckGo", configured: true},
	}
	d := dispatcherWith(providers, "")
	p, err := d.resolveProvider()
	require.NoError(t, err)
	assert.Equal(t, "DuckDuckGo", p.ID())
}

func TestDispatcher_HandlerCachesRepeatedQueries(t *testing.T) {
	fake := &fakeProvider{
		id:         "DuckDuckGo",
		configured: true,
		results:    []SearchResult{{Title: "T", URL: "https://x", Snippet: "S"}},
	}
	providers := map[string]SearchProvider{"DuckDuckGo": fake}
	d := dispatcherWith(providers, "DuckDuckGo")
	h := d.Handler()

	out1, err := h(context.Background(), json.RawMessage(`{"query":"golang"}`))
	require.NoError(t, err)
	out2, err := h(context.Background(), json.RawMessage(`{"query":"golang"}`))
	require.NoError(t, err)

	assert.Equal(t, out1, out2, "cached output must match")
	assert.Equal(t, 1, fake.calls, "second call should hit cache")
}

func TestDispatcher_HandlerDifferentQueriesBypassCache(t *testing.T) {
	fake := &fakeProvider{id: "DuckDuckGo", configured: true, results: []SearchResult{{Title: "T"}}}
	providers := map[string]SearchProvider{"DuckDuckGo": fake}
	d := dispatcherWith(providers, "DuckDuckGo")
	h := d.Handler()

	_, _ = h(context.Background(), json.RawMessage(`{"query":"a"}`))
	_, _ = h(context.Background(), json.RawMessage(`{"query":"b"}`))

	assert.Equal(t, 2, fake.calls)
}

func TestDispatcher_HandlerSerializesResult(t *testing.T) {
	score := 0.7
	fake := &fakeProvider{
		id:         "tavily",
		configured: true,
		results: []SearchResult{
			{Title: "T1", URL: "https://x", Snippet: "S1", Score: &score},
			{Title: "T2", URL: "https://y", Snippet: "S2"},
		},
	}
	providers := map[string]SearchProvider{"tavily": fake}
	d := dispatcherWith(providers, "tavily")
	h := d.Handler()

	out, err := h(context.Background(), json.RawMessage(`{"query":"golang","num_results":2}`))
	require.NoError(t, err)

	var result struct {
		Query    string         `json:"query"`
		Provider string         `json:"provider"`
		Results  []SearchResult `json:"results"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "golang", result.Query)
	assert.Equal(t, "tavily", result.Provider)
	require.Len(t, result.Results, 2)
	assert.Equal(t, "T1", result.Results[0].Title)
	require.NotNil(t, result.Results[0].Score)
	assert.Nil(t, result.Results[1].Score)
}

func TestDispatcher_HandlerRejectsEmptyQuery(t *testing.T) {
	d := dispatcherWith(map[string]SearchProvider{"DuckDuckGo": &fakeProvider{id: "DuckDuckGo", configured: true}}, "DuckDuckGo")
	out, err := d.Handler()(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "query")
}

func TestDispatcher_HandlerWrapsProviderError(t *testing.T) {
	fake := &fakeProvider{id: "DuckDuckGo", configured: true, err: errors.New("http 500")}
	d := dispatcherWith(map[string]SearchProvider{"DuckDuckGo": fake}, "DuckDuckGo")
	out, err := d.Handler()(context.Background(), json.RawMessage(`{"query":"q"}`))
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "DuckDuckGo")
	assert.Contains(t, out, "http 500")
}

func TestDispatcher_HandlerClampsNumResults(t *testing.T) {
	fake := &fakeProvider{id: "DuckDuckGo", configured: true, results: []SearchResult{{Title: "T"}}}
	d := dispatcherWith(map[string]SearchProvider{"DuckDuckGo": fake}, "DuckDuckGo")

	_, _ = d.Handler()(context.Background(), json.RawMessage(`{"query":"q"}`))
	_, _ = d.Handler()(context.Background(), json.RawMessage(`{"query":"q2","num_results":30}`))
	assert.Equal(t, 2, fake.calls)
}

func TestRegisterAll_RegistersSearchWhenEnabled(t *testing.T) {
	t.Setenv("EXA_API_KEY", "")
	t.Setenv("TAVILY_API_KEY", "")
	t.Setenv("BRAVE_API_KEY", "")
	reg := tool.NewRegistry()
	RegisterAll(reg, Options{})
	out, err := reg.Dispatch(context.Background(), "web_search", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, "query", "handler rejected empty query (tool IS registered)")
	assert.NotContains(t, out, "unknown tool")
}

func TestRegisterAll_RegistersFetchAlways(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterAll(reg, Options{})
	out, err := reg.Dispatch(context.Background(), "web_fetch", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "url")
}

func TestRegisterAll_RegistersExtractWhenKeyPresent(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterAll(reg, Options{FirecrawlAPIKey: "test"})
	out, err := reg.Dispatch(context.Background(), "web_extract", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.NotContains(t, out, "unknown tool")
}

func TestRegisterAll_SkipsExtractWithoutKey(t *testing.T) {
	t.Setenv("FIRECRAWL_API_KEY", "")
	reg := tool.NewRegistry()
	RegisterAll(reg, Options{})
	out, err := reg.Dispatch(context.Background(), "web_extract", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, "unknown tool")
}
