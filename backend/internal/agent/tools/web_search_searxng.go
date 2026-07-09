package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
)

// searxngProvider searches via a self-hosted SearXNG instance.
// It requires no API key, only a base URL.
type searxngProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("searxng-engine", &searxngProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *searxngProvider) Name() string { return "SearXNG" }

func (p *searxngProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	baseURL := firstNonEmptyString(
		settings["AgentSearXNGApiUrl"],
		cfg.AgentSearXNGApiUrl,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("SearXNG base URL not configured. Set AgentSearXNGApiUrl in settings.")
	}

	baseURL = strings.TrimRight(baseURL, "/")
	endpoint := baseURL + "/search?" + url.Values{
		"format": {"json"},
		"q":      {query},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("searxng: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searxng: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("searxng: read body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("searxng: http %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed searxngResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("searxng: decode response: %w", err)
	}

	results := make([]SearchResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		if r.Title == "" || r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:         r.Title,
			Link:          r.URL,
			Snippet:       r.Content,
			PublishedDate: r.PublishedDate,
		})
	}
	return results, nil
}

type searxngResponse struct {
	Results []searxngResult `json:"results"`
}

type searxngResult struct {
	Title         string `json:"title"`
	URL           string `json:"url"`
	Content       string `json:"content"`
	PublishedDate string `json:"publishedDate"`
}
