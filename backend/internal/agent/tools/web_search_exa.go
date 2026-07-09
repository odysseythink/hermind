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

// exaProvider searches the web via Exa.
type exaProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("exa-search", &exaProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *exaProvider) Name() string { return "Exa" }

func (p *exaProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	apiKey, err := providers.ResolveAPIKey("exa", settings, cfg)
	if err != nil {
		return nil, fmt.Errorf("Exa API key not configured. Set AgentExaApiKey in settings.")
	}

	body, err := json.Marshal(map[string]any{
		"query":      query,
		"type":       "auto",
		"numResults": 10,
		"contents": map[string]bool{
			"text": true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("exa: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.exa.ai/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("exa: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exa: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("exa: read body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("exa: http %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed exaResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("exa: decode response: %w", err)
	}

	results := make([]SearchResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		if r.Title == "" || r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:         r.Title,
			Link:          r.URL,
			Snippet:       r.Text,
			PublishedDate: r.PublishedDate,
		})
	}
	return results, nil
}

type exaResponse struct {
	Results []exaResult `json:"results"`
}

type exaResult struct {
	Title         string `json:"title"`
	URL           string `json:"url"`
	Text          string `json:"text"`
	PublishedDate string `json:"publishedDate"`
}
