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

// bingProvider searches the web via Bing Web Search API.
type bingProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("bing-search", &bingProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *bingProvider) Name() string { return "Bing" }

func (p *bingProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	apiKey, err := providers.ResolveAPIKey("bing", settings, cfg)
	if err != nil {
		return nil, fmt.Errorf("Bing Search API key not configured. Set AgentBingSearchApiKey in settings.")
	}

	endpoint := "https://api.bing.microsoft.com/v7.0/search?q=" + url.QueryEscape(query) +
		"&count=" + strconv.Itoa(15) + "&mkt=en-US"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("bing: build request: %w", err)
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", apiKey)
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bing: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("bing: read body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("bing: http %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed bingResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("bing: decode response: %w", err)
	}

	results := make([]SearchResult, 0, len(parsed.WebPages.Value))
	for _, r := range parsed.WebPages.Value {
		if r.Name == "" || r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Name,
			Link:    r.URL,
			Snippet: r.Snippet,
		})
	}
	return results, nil
}

type bingResponse struct {
	WebPages bingWebPages `json:"webPages"`
}

type bingWebPages struct {
	Value []bingResult `json:"value"`
}

type bingResult struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}
