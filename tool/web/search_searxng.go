package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type searxngProvider struct {
	baseURL string
	client  *http.Client
}

func newSearXNGProvider(baseURL string) *searxngProvider {
	return &searxngProvider{
		baseURL: baseURL,
		client:  &http.Client{Timeout: httpTimeout},
	}
}

func (p *searxngProvider) ID() string      { return "searxng" }
func (p *searxngProvider) Configured() bool { return p.baseURL != "" }

func (p *searxngProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	if p.baseURL == "" {
		return nil, fmt.Errorf("searxng: base URL not configured")
	}

	u, err := url.Parse(strings.TrimSuffix(p.baseURL, "/") + "/search")
	if err != nil {
		return nil, fmt.Errorf("searxng: invalid base URL: %w", err)
	}
	qval := u.Query()
	qval.Set("q", q)
	qval.Set("format", "json")
	u.RawQuery = qval.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("searxng: request error: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searxng: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("searxng: http %d", resp.StatusCode)
	}

	var payload struct {
		Results []struct {
			URL           string `json:"url"`
			Title         string `json:"title"`
			Content       string `json:"content"`
			PublishedDate string `json:"publishedDate"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("searxng: decode error: %w", err)
	}

	results := make([]SearchResult, 0, n)
	for i, r := range payload.Results {
		if i >= n {
			break
		}
		results = append(results, SearchResult{
			Title:         r.Title,
			URL:           r.URL,
			Snippet:       r.Content,
			PublishedDate: r.PublishedDate,
		})
	}

	return results, nil
}
