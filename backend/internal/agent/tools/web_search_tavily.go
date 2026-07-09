package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/providers"
)

// tavilyProvider searches the web via Tavily.
type tavilyProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("tavily-search", &tavilyProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *tavilyProvider) Name() string { return "Tavily" }

func (p *tavilyProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	apiKey, err := providers.ResolveAPIKey("tavily", settings, cfg)
	if err != nil {
		return nil, fmt.Errorf("Tavily API key not configured. Set AgentTavilyApiKey in settings.")
	}

	body, err := json.Marshal(map[string]any{
		"api_key":       apiKey,
		"query":         query,
		"search_depth":  "basic",
		"max_results":   10,
	})
	if err != nil {
		return nil, fmt.Errorf("tavily: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("tavily: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("tavily: read body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("tavily: http %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed tavilyResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("tavily: decode response: %w", err)
	}

	if parsed.Detail != nil && parsed.Detail.Error != "" {
		return nil, fmt.Errorf("tavily: %s", parsed.Detail.Error)
	}

	results := make([]SearchResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		if r.Title == "" || r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    r.URL,
			Snippet: r.Content,
		})
	}
	return results, nil
}

type tavilyResponse struct {
	Results []tavilyResult `json:"results"`
	Detail  *tavilyDetail  `json:"detail"`
}

type tavilyResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

type tavilyDetail struct {
	Error string `json:"error"`
}
