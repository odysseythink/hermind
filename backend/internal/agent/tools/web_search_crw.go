package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/providers"
)

// crwProvider searches the web via fastCRW.
type crwProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("crw-search", &crwProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *crwProvider) Name() string { return "fastCRW" }

func (p *crwProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	apiKey, err := providers.ResolveAPIKey("crw", settings, cfg)
	if err != nil {
		return nil, fmt.Errorf("fastCRW API key not configured. Set AgentCrwApiKey in settings.")
	}

	baseURL := firstNonEmptyString(
		settings["AgentCrwApiUrl"],
		cfg.AgentCrwApiUrl,
		"https://fastcrw.com/api",
	)
	baseURL = strings.TrimRight(baseURL, "/")
	endpoint := baseURL + "/v1/search"

	body, err := json.Marshal(map[string]any{
		"query":       query,
		"search_type": "web",
		"num":         10,
	})
	if err != nil {
		return nil, fmt.Errorf("crw: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("crw: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("crw: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("crw: read body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("crw: http %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed crwResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("crw: decode response: %w", err)
	}

	if parsed.Success != nil && !*parsed.Success {
		return nil, fmt.Errorf("crw: %s", firstNonEmptyString(parsed.Error, "unknown error"))
	}

	results := make([]SearchResult, 0, len(parsed.Data))
	for _, r := range parsed.Data {
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

type crwResponse struct {
	Success *bool      `json:"success"`
	Error   string     `json:"error"`
	Data    []crwResult `json:"data"`
}

type crwResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}
