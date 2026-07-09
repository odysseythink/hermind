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

// serplyProvider searches the web via Serply.io.
type serplyProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("serply-engine", &serplyProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *serplyProvider) Name() string { return "Serply" }

func (p *serplyProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	apiKey, err := providers.ResolveAPIKey("serply", settings, cfg)
	if err != nil {
		return nil, fmt.Errorf("Serply API key not configured. Set AgentSerplyApiKey in settings.")
	}

	endpoint := "https://api.serply.io/v1/search?" + url.Values{
		"q":    {query},
		"num":  {strconv.Itoa(10)},
		"hl":   {"us"},
		"gl":   {"US"},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("serply: build request: %w", err)
	}
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("serply: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("serply: read body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("serply: http %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed serplyResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("serply: decode response: %w", err)
	}

	if parsed.Message == "Unauthorized" {
		return nil, fmt.Errorf("serply: unauthorized. Please double check your AgentSerplyApiKey")
	}

	results := make([]SearchResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		if r.Title == "" || r.Link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    r.Link,
			Snippet: r.Description,
		})
	}
	return results, nil
}

type serplyResponse struct {
	Message string         `json:"message"`
	Results []serplyResult `json:"results"`
}

type serplyResult struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description"`
}
