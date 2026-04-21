package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

const tavilyDefaultURL = "https://api.tavily.com/search"

type tavilyProvider struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func newTavilyProvider(apiKey, endpoint string) *tavilyProvider {
	return &tavilyProvider{
		apiKey:   apiKey,
		endpoint: endpoint,
		client:   &http.Client{Timeout: httpTimeout},
	}
}

func (p *tavilyProvider) ID() string { return "tavily" }

func (p *tavilyProvider) Configured() bool {
	return p.resolvedKey() != ""
}

func (p *tavilyProvider) resolvedKey() string {
	if p.apiKey != "" {
		return p.apiKey
	}
	return os.Getenv("TAVILY_API_KEY")
}

type tavilyRequest struct {
	APIKey            string `json:"api_key"`
	Query             string `json:"query"`
	MaxResults        int    `json:"max_results"`
	IncludeAnswer     bool   `json:"include_answer"`
	IncludeRawContent bool   `json:"include_raw_content"`
}

type tavilyResponse struct {
	Query   string             `json:"query"`
	Results []tavilyResultItem `json:"results"`
}

type tavilyResultItem struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Content       string  `json:"content"`
	Score         float64 `json:"score,omitempty"`
	PublishedDate string  `json:"published_date,omitempty"`
}

func (p *tavilyProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	key := p.resolvedKey()
	if key == "" {
		return nil, fmt.Errorf("api key missing")
	}
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = tavilyDefaultURL
	}

	body, _ := json.Marshal(tavilyRequest{
		APIKey:            key,
		Query:             q,
		MaxResults:        n,
		IncludeAnswer:     false,
		IncludeRawContent: false,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	var out tavilyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	results := make([]SearchResult, 0, len(out.Results))
	for _, r := range out.Results {
		item := SearchResult{
			Title:         r.Title,
			URL:           r.URL,
			Snippet:       r.Content,
			PublishedDate: r.PublishedDate,
		}
		if r.Score != 0 {
			s := r.Score
			item.Score = &s
		}
		results = append(results, item)
	}
	return results, nil
}
