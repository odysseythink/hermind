package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/providers"
)

// braveProvider searches the web via Brave Search API.
type braveProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("brave-search", &braveProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *braveProvider) Name() string { return "Brave" }

func (p *braveProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	apiKey, err := providers.ResolveAPIKey("brave", settings, cfg)
	if err != nil {
		return nil, fmt.Errorf("Brave Search API key not configured. Set AgentBraveApiKey in settings.")
	}

	endpoint := "https://api.search.brave.com/res/v1/web/search?q=" + url.QueryEscape(query) + "&count=" + strconv.Itoa(10)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("brave: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("brave: read body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("brave: http %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed braveResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("brave: decode response: %w", err)
	}

	results := make([]SearchResult, 0, len(parsed.Web.Results))
	for _, r := range parsed.Web.Results {
		if r.Title == "" || r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    r.URL,
			Snippet: r.Description,
		})
	}
	return results, nil
}

type braveResponse struct {
	Web braveWeb `json:"web"`
}

type braveWeb struct {
	Results []braveResult `json:"results"`
}

type braveResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}
