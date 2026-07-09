package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

// mockWebSearchProvider is a test double that never hits the network.
type mockWebSearchProvider struct {
	name    string
	results []SearchResult
	err     error
}

func (m *mockWebSearchProvider) Name() string { return m.name }

func (m *mockWebSearchProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	return m.results, m.err
}

func TestWebBrowsing_UsesConfiguredProvider(t *testing.T) {
	mock := &mockWebSearchProvider{
		name: "MockProvider",
		results: []SearchResult{
			{Title: "Result A", Link: "https://a.example", Snippet: "snippet A"},
			{Title: "Result B", Link: "https://b.example", Snippet: "snippet B"},
		},
	}
	registerSearchProvider("mock-engine", mock)
	defer delete(searchProviderRegistry, "mock-engine")

	var emits []string
	tc := &ToolContext{
		Settings: map[string]string{"agent_search_provider": "mock-engine"},
		Cfg:      &config.Config{},
		Emit:     func(msg string) { emits = append(emits, msg) },
	}

	entry := NewWebBrowsingSkill(tc)
	result, err := entry.Handler(context.Background(), []byte(`{"query":"golang latest"}`))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if !strings.Contains(result, `"title":"Result A"`) || !strings.Contains(result, `"link":"https://b.example"`) {
		t.Fatalf("expected result to contain mock results, got: %s", result)
	}
	if len(emits) != 2 {
		t.Fatalf("expected 2 status emits, got %d: %v", len(emits), emits)
	}
	if !strings.Contains(emits[0], "Searching web via MockProvider for: golang latest") {
		t.Errorf("unexpected first emit: %q", emits[0])
	}
	if !strings.Contains(emits[1], "Found 2 results via MockProvider") {
		t.Errorf("unexpected second emit: %q", emits[1])
	}
}

func TestWebBrowsing_DefaultsToDuckDuckGo(t *testing.T) {
	mock := &mockWebSearchProvider{
		name:    "DDGMock",
		results: []SearchResult{{Title: "Default Result", Link: "https://ddg.example", Snippet: "default snippet"}},
	}
	old := searchProviderRegistry["duckduckgo-engine"]
	searchProviderRegistry["duckduckgo-engine"] = mock
	t.Cleanup(func() {
		if old == nil {
			delete(searchProviderRegistry, "duckduckgo-engine")
		} else {
			searchProviderRegistry["duckduckgo-engine"] = old
		}
	})

	tc := &ToolContext{
		Settings: map[string]string{}, // no agent_search_provider set
		Cfg:      &config.Config{},
		Emit:     func(string) {},
	}

	entry := NewWebBrowsingSkill(tc)
	result, err := entry.Handler(context.Background(), []byte(`{"query":"anything"}`))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(result, "Default Result") {
		t.Fatalf("expected fallback to duckduckgo-engine, got: %s", result)
	}
}

func TestWebBrowsing_UnknownProviderFallsBack(t *testing.T) {
	mock := &mockWebSearchProvider{
		name:    "DDGMock",
		results: []SearchResult{{Title: "Fallback Result", Link: "https://fallback.example", Snippet: "fallback snippet"}},
	}
	old := searchProviderRegistry["duckduckgo-engine"]
	searchProviderRegistry["duckduckgo-engine"] = mock
	t.Cleanup(func() {
		if old == nil {
			delete(searchProviderRegistry, "duckduckgo-engine")
		} else {
			searchProviderRegistry["duckduckgo-engine"] = old
		}
	})

	tc := &ToolContext{
		Settings: map[string]string{"agent_search_provider": "not-a-real-engine"},
		Cfg:      &config.Config{},
		Emit:     func(string) {},
	}

	entry := NewWebBrowsingSkill(tc)
	result, err := entry.Handler(context.Background(), []byte(`{"query":"anything"}`))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(result, "Fallback Result") {
		t.Fatalf("expected fallback to duckduckgo-engine for unknown provider, got: %s", result)
	}
}

func TestWebBrowsing_QueryRequired(t *testing.T) {
	tc := &ToolContext{Settings: map[string]string{}, Cfg: &config.Config{}, Emit: func(string) {}}
	entry := NewWebBrowsingSkill(tc)

	result, err := entry.Handler(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(result, "query is required") {
		t.Fatalf("expected query required error, got: %s", result)
	}
}

func TestWebBrowsing_SearchError(t *testing.T) {
	// When the DEFAULT provider itself fails, no fallback is possible -> error returned.
	mock := &mockWebSearchProvider{name: "BadDefault", err: context.DeadlineExceeded}
	old := searchProviderRegistry[defaultSearchProvider]
	searchProviderRegistry[defaultSearchProvider] = mock
	t.Cleanup(func() {
		if old == nil {
			delete(searchProviderRegistry, defaultSearchProvider)
		} else {
			searchProviderRegistry[defaultSearchProvider] = old
		}
	})

	tc := &ToolContext{
		Settings: map[string]string{}, // empty -> uses default
		Cfg:      &config.Config{},
		Emit:     func(string) {},
	}

	entry := NewWebBrowsingSkill(tc)
	result, err := entry.Handler(context.Background(), []byte(`{"query":"x"}`))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(result, "Web search is currently unavailable") {
		t.Fatalf("expected 'Web search is currently unavailable', got: %s", result)
	}
}

func TestWebBrowsing_NoResults(t *testing.T) {
	mock := &mockWebSearchProvider{name: "EmptyProvider", results: []SearchResult{}}
	registerSearchProvider("empty-engine", mock)
	defer delete(searchProviderRegistry, "empty-engine")

	tc := &ToolContext{
		Settings: map[string]string{"agent_search_provider": "empty-engine"},
		Cfg:      &config.Config{},
		Emit:     func(string) {},
	}

	entry := NewWebBrowsingSkill(tc)
	result, err := entry.Handler(context.Background(), []byte(`{"query":"xyzxyzxyz"}`))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(result, "No information was found online for the search query.") {
		t.Fatalf("expected no-results message, got: %s", result)
	}
}

func TestWebBrowsing_EmitsCitations(t *testing.T) {
	registerSearchProvider("mock-emit-test", &mockEmittingProvider{})

	var capturedCitations []Citation
	emitter := func(citations []Citation) {
		capturedCitations = citations
	}

	tc := &ToolContext{
		Ctx:           context.Background(),
		Settings:      map[string]string{"agent_search_provider": "mock-emit-test"},
		Emit:          func(string) {},
		EmitCitations: emitter,
		Cfg:           &config.Config{},
	}

	entry := NewWebBrowsingSkill(tc)
	handler := entry.Handler

	result, err := handler(context.Background(), []byte(`{"query":"test query"}`))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	if len(capturedCitations) != 2 {
		t.Fatalf("expected 2 citations, got %d", len(capturedCitations))
	}
	if capturedCitations[0].ID != "https://example.com/a" {
		t.Errorf("expected ID https://example.com/a, got %s", capturedCitations[0].ID)
	}
	if capturedCitations[0].Title != "Result A" {
		t.Errorf("expected title Result A, got %s", capturedCitations[0].Title)
	}
	if capturedCitations[0].Text != "Snippet A" {
		t.Errorf("expected text Snippet A, got %s", capturedCitations[0].Text)
	}
	if capturedCitations[0].ChunkSource != "link://https://example.com/a" {
		t.Errorf("expected chunkSource link://example.com/a, got %s", capturedCitations[0].ChunkSource)
	}
	if capturedCitations[0].Score != nil {
		t.Error("expected nil Score")
	}
	if capturedCitations[1].ID != "https://example.com/b" {
		t.Errorf("expected ID https://example.com/b, got %s", capturedCitations[1].ID)
	}
}

func TestWebBrowsing_SkipsResultWithoutURL(t *testing.T) {
	registerSearchProvider("mock-skip-test", &mockSkippingProvider{})

	var emitted int
	emitter := func(citations []Citation) {
		emitted = len(citations)
	}

	tc := &ToolContext{
		Ctx:           context.Background(),
		Settings:      map[string]string{"agent_search_provider": "mock-skip-test"},
		Emit:          func(string) {},
		EmitCitations: emitter,
		Cfg:           &config.Config{},
	}

	entry := NewWebBrowsingSkill(tc)

	_, err := entry.Handler(context.Background(), []byte(`{"query":"skip"}`))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if emitted != 0 {
		t.Fatalf("expected 0 citations when all results have empty URLs, got %d", emitted)
	}
}

func TestWebBrowsing_NilEmitterDoesNotPanic(t *testing.T) {
	registerSearchProvider("mock-nil-emit", &mockEmittingProvider{})

	tc := &ToolContext{
		Ctx:           context.Background(),
		Settings:      map[string]string{"agent_search_provider": "mock-nil-emit"},
		Emit:          func(string) {},
		EmitCitations: nil, // explicitly nil
		Cfg:           &config.Config{},
	}

	entry := NewWebBrowsingSkill(tc)

	_, err := entry.Handler(context.Background(), []byte(`{"query":"any"}`))
	if err != nil {
		t.Fatalf("nil EmitCitations should not cause panic: %v", err)
	}
}

type mockEmittingProvider struct{}

func (p *mockEmittingProvider) Name() string { return "MockEmit" }
func (p *mockEmittingProvider) Search(ctx context.Context, query string, _ map[string]string, _ *config.Config) ([]SearchResult, error) {
	return []SearchResult{
		{Title: "Result A", Link: "https://example.com/a", Snippet: "Snippet A"},
		{Title: "Result B", Link: "https://example.com/b", Snippet: "Snippet B"},
	}, nil
}

type mockSkippingProvider struct{}

func (p *mockSkippingProvider) Name() string { return "MockSkip" }
func (p *mockSkippingProvider) Search(ctx context.Context, query string, _ map[string]string, _ *config.Config) ([]SearchResult, error) {
	return []SearchResult{{Title: "No URL", Link: "", Snippet: "No link here"}}, nil
}

func TestWebBrowsing_FallbackOnProviderError(t *testing.T) {
	// Primary provider returns an error -> fallback to DuckDuckGo.
	primary := &mockWebSearchProvider{
		name: "BadSerper",
		err:  &SearchError{Provider: "Serper.dev", Message: "timeout", Cause: context.DeadlineExceeded},
	}
	fallback := &mockWebSearchProvider{
		name:    "DDGFallback",
		results: []SearchResult{{Title: "Fallback OK", Link: "https://ddg.example", Snippet: "ddg worked"}},
	}
	// Override both registry entries.
	registerSearchProvider("serper-dot-dev", primary)
	registerSearchProvider("duckduckgo-engine", fallback)
	t.Cleanup(func() {
		delete(searchProviderRegistry, "serper-dot-dev")
		delete(searchProviderRegistry, "duckduckgo-engine")
	})

	var emits []string
	tc := &ToolContext{
		Settings: map[string]string{"agent_search_provider": "serper-dot-dev"},
		Cfg:      &config.Config{},
		Emit:     func(msg string) { emits = append(emits, msg) },
	}

	entry := NewWebBrowsingSkill(tc)
	result, err := entry.Handler(context.Background(), []byte(`{"query":"fallback test"}`))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(result, "Fallback OK") {
		t.Fatalf("expected fallback to DDG results, got: %s", result)
	}
	// Verify the warning status emit mentions the fallback.
	found := false
	for _, e := range emits {
		if strings.Contains(e, "Falling back to DuckDuckGo") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a 'Falling back to DuckDuckGo' status emit, got: %v", emits)
	}
}

func TestWebBrowsing_AllProvidersFail(t *testing.T) {
	// Both primary (non-default) and fallback (DDG) fail -> "unavailable" error.
	primary := &mockWebSearchProvider{
		name: "DeadSerper",
		err:  &SearchError{Provider: "Serper.dev", Message: "timeout after 10s", Cause: context.DeadlineExceeded},
	}
	ddg := &mockWebSearchProvider{
		name: "DeadDDG",
		err:  &SearchError{Provider: "DuckDuckGo", Message: "network unreachable", Cause: context.DeadlineExceeded},
	}
	registerSearchProvider("serper-dot-dev", primary)
	registerSearchProvider(defaultSearchProvider, ddg)
	t.Cleanup(func() {
		delete(searchProviderRegistry, "serper-dot-dev")
		delete(searchProviderRegistry, defaultSearchProvider)
	})

	tc := &ToolContext{
		Settings: map[string]string{"agent_search_provider": "serper-dot-dev"},
		Cfg:      &config.Config{},
		Emit:     func(string) {},
	}

	entry := NewWebBrowsingSkill(tc)
	result, err := entry.Handler(context.Background(), []byte(`{"query":"dead"}`))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(result, "Web search is currently unavailable") {
		t.Fatalf("expected 'Web search is currently unavailable', got: %s", result)
	}
	if !strings.Contains(result, "All search providers failed") {
		t.Fatalf("expected 'All search providers failed', got: %s", result)
	}
}
