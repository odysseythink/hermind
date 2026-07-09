package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/providers"
)

// searchapiProvider searches via SearchApi.io.
type searchapiProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("searchapi", &searchapiProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *searchapiProvider) Name() string { return "SearchApi" }

func (p *searchapiProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	apiKey, err := providers.ResolveAPIKey("searchapi", settings, cfg)
	if err != nil {
		return nil, fmt.Errorf("SearchApi API key not configured. Set AgentSearchApiKey in settings.")
	}

	engine := firstNonEmptyString(
		settings["AgentSearchApiEngine"],
		cfg.AgentSearchApiEngine,
		"google",
	)

	params := url.Values{
		"engine": {engine},
		"q":      {query},
	}
	endpoint := "https://www.searchapi.io/api/v1/search?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("searchapi: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-SearchApi-Source", "AnythingLLM")
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searchapi: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("searchapi: read body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("searchapi: http %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed searchapiResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("searchapi: decode response: %w", err)
	}

	return normalizeSearchapiResponse(&parsed), nil
}

type searchapiResponse struct {
	KnowledgeGraph *searchapiKnowledgeGraph `json:"knowledge_graph"`
	AnswerBox      *searchapiAnswerBox       `json:"answer_box"`
	OrganicResults []searchapiOrganic        `json:"organic_results"`
}

type searchapiKnowledgeGraph struct {
	Description string `json:"description"`
}

type searchapiAnswerBox struct {
	Answer string `json:"answer"`
}

type searchapiOrganic struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}

func normalizeSearchapiResponse(resp *searchapiResponse) []SearchResult {
	results := make([]SearchResult, 0)

	if resp.KnowledgeGraph != nil && resp.KnowledgeGraph.Description != "" {
		results = append(results, SearchResult{
			Title:   "Knowledge Graph",
			Link:    "",
			Snippet: resp.KnowledgeGraph.Description,
		})
	}
	if resp.AnswerBox != nil && resp.AnswerBox.Answer != "" {
		results = append(results, SearchResult{
			Title:   "Answer Box",
			Link:    "",
			Snippet: resp.AnswerBox.Answer,
		})
	}
	for _, r := range resp.OrganicResults {
		if r.Title == "" || r.Link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    r.Link,
			Snippet: r.Snippet,
		})
	}
	return results
}
