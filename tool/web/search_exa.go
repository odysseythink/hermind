package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// exaEndpoint is the production Exa search endpoint. Named distinct
// from the legacy exaDefaultURL still living in search.go so both
// can coexist until Task 10 strips the legacy block.
const exaEndpoint = "https://api.exa.ai/search"

type exaProvider struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func newExaProvider(apiKey, endpoint string) *exaProvider {
	return &exaProvider{
		apiKey:   apiKey,
		endpoint: endpoint,
		client:   &http.Client{Timeout: httpTimeout},
	}
}

func (p *exaProvider) ID() string { return "exa" }

func (p *exaProvider) Configured() bool {
	return p.resolvedKey() != ""
}

func (p *exaProvider) resolvedKey() string {
	if p.apiKey != "" {
		return p.apiKey
	}
	return os.Getenv("EXA_API_KEY")
}

type exaRequest struct {
	Query      string `json:"query"`
	NumResults int    `json:"numResults"`
}

type exaResponse struct {
	Results []exaResultItem `json:"results"`
}

type exaResultItem struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Text          string  `json:"text,omitempty"`
	PublishedDate string  `json:"publishedDate,omitempty"`
	Score         float64 `json:"score,omitempty"`
}

func (p *exaProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	key := p.resolvedKey()
	if key == "" {
		return nil, fmt.Errorf("api key missing")
	}
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = exaEndpoint
	}

	body, _ := json.Marshal(exaRequest{Query: q, NumResults: n})
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	var out exaResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	results := make([]SearchResult, 0, len(out.Results))
	for _, r := range out.Results {
		item := SearchResult{
			Title:         r.Title,
			URL:           r.URL,
			Snippet:       r.Text,
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
