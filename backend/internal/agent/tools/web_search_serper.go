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

// serperProvider searches Google via Serper.dev.
// It requires an API key stored in settings["AgentSerperApiKey"] or the
// AGENT_SERPER_API_KEY environment variable.
type serperProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("serper-dot-dev", &serperProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *serperProvider) Name() string { return "Serper.dev" }

func (p *serperProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	apiKey, err := providers.ResolveAPIKey("serper", settings, cfg)
	if err != nil {
		return nil, fmt.Errorf("Serper.dev API key not configured. Set AgentSerperApiKey in settings.")
	}

	body, err := json.Marshal(map[string]string{
		"q":  query,
		"gl": "us",
		"hl": "en",
	})
	if err != nil {
		return nil, fmt.Errorf("serper: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://google.serper.dev/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("serper: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("serper: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("serper: http %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("serper: read body: %w", err)
	}

	var parsed serperResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("serper: decode response: %w", err)
	}

	return normalizeSerperResponse(&parsed), nil
}

type serperResponse struct {
	Organic        []serperOrganic        `json:"organic"`
	KnowledgeGraph *serperKnowledgeGraph  `json:"knowledgeGraph"`
}

type serperOrganic struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}

type serperKnowledgeGraph struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func normalizeSerperResponse(resp *serperResponse) []SearchResult {
	results := make([]SearchResult, 0, len(resp.Organic)+1)

	if resp.KnowledgeGraph != nil && resp.KnowledgeGraph.Description != "" {
		results = append(results, SearchResult{
			Title:   resp.KnowledgeGraph.Title,
			Link:    "",
			Snippet: resp.KnowledgeGraph.Description,
		})
	}

	for _, o := range resp.Organic {
		if o.Title == "" || o.Link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   o.Title,
			Link:    o.Link,
			Snippet: o.Snippet,
		})
	}

	return results
}
