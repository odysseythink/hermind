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

// perplexityProvider searches the web via Perplexity.
type perplexityProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("perplexity-search", &perplexityProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *perplexityProvider) Name() string { return "Perplexity" }

func (p *perplexityProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	apiKey, err := providers.ResolveAPIKey("perplexity-search", settings, cfg)
	if err != nil {
		return nil, fmt.Errorf("Perplexity API key not configured. Set AgentPerplexityApiKey in settings.")
	}

	body, err := json.Marshal(map[string]any{
		"query":               query,
		"max_results":         5,
		"max_tokens_per_page": 2048,
	})
	if err != nil {
		return nil, fmt.Errorf("perplexity: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.perplexity.ai/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("perplexity: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perplexity: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("perplexity: read body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("perplexity: http %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed perplexityResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("perplexity: decode response: %w", err)
	}

	results := make([]SearchResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		title := firstNonEmptyString(r.Title, r.Name)
		link := firstNonEmptyString(r.URL, r.Link)
		if title == "" || link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   title,
			Link:    link,
			Snippet: firstNonEmptyString(r.Snippet, r.Text, r.Description),
		})
	}
	return results, nil
}

type perplexityResponse struct {
	Results []perplexityResult `json:"results"`
}

type perplexityResult struct {
	Title       string `json:"title"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Link        string `json:"link"`
	Snippet     string `json:"snippet"`
	Text        string `json:"text"`
	Description string `json:"description"`
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
