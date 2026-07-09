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

// baiduProvider searches the web via Baidu Qianfan.
type baiduProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("baidu-search", &baiduProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *baiduProvider) Name() string { return "Baidu" }

func (p *baiduProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	apiKey, err := providers.ResolveAPIKey("baidu", settings, cfg)
	if err != nil {
		return nil, fmt.Errorf("Baidu Search API key not configured. Set AgentBaiduSearchApiKey in settings.")
	}

	body, err := json.Marshal(map[string]any{
		"messages": []map[string]string{
			{"role": "user", "content": query},
		},
		"resource_type_filter": []map[string]any{
			{"type": "web", "top_k": 10},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("baidu: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://qianfan.baidubce.com/v2/ai_search/web_search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("baidu: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Appbuilder-Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("baidu: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("baidu: read body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("baidu: http %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed baiduResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("baidu: decode response: %w", err)
	}

	if (parsed.Code != "" || parsed.Message != "") && len(parsed.References) == 0 {
		return nil, fmt.Errorf("baidu: %s", firstNonEmptyString(parsed.Message, parsed.Code, "unknown error"))
	}

	return normalizeBaiduReferences(parsed.References), nil
}

type baiduResponse struct {
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	References []baiduReference `json:"references"`
}

type baiduReference struct {
	Title         string `json:"title"`
	WebAnchor     string `json:"web_anchor"`
	URL           string `json:"url"`
	Snippet       string `json:"snippet"`
	Content       string `json:"content"`
	Type          string `json:"type"`
	ResourceType  string `json:"resource_type"`
}

func normalizeBaiduReferences(refs []baiduReference) []SearchResult {
	seen := make(map[string]struct{})
	results := make([]SearchResult, 0, len(refs))

	for _, r := range refs {
		refType := strings.ToLower(firstNonEmptyString(r.Type, r.ResourceType, "web"))
		if refType != "web" {
			continue
		}

		title := strings.TrimSpace(firstNonEmptyString(r.Title, r.WebAnchor))
		link := strings.TrimSpace(r.URL)
		snippet := strings.TrimSpace(firstNonEmptyString(r.Snippet, r.Content))

		if title == "" || link == "" {
			continue
		}
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}

		results = append(results, SearchResult{
			Title:   title,
			Link:    link,
			Snippet: snippet,
		})
	}

	return results
}
