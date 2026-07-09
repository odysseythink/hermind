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
	mock := &mockWebSearchProvider{name: "BadProvider", err: context.DeadlineExceeded}
	registerSearchProvider("bad-engine", mock)
	defer delete(searchProviderRegistry, "bad-engine")

	tc := &ToolContext{
		Settings: map[string]string{"agent_search_provider": "bad-engine"},
		Cfg:      &config.Config{},
		Emit:     func(string) {},
	}

	entry := NewWebBrowsingSkill(tc)
	result, err := entry.Handler(context.Background(), []byte(`{"query":"x"}`))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(result, "search error [BadProvider]") {
		t.Fatalf("expected provider-scoped error, got: %s", result)
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
