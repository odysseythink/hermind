package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

const braveDefaultURL = "https://api.search.brave.com/res/v1/web/search"

type braveProvider struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func newBraveProvider(apiKey, endpoint string) *braveProvider {
	return &braveProvider{
		apiKey:   apiKey,
		endpoint: endpoint,
		client:   &http.Client{Timeout: httpTimeout},
	}
}

func (p *braveProvider) ID() string { return "brave" }

func (p *braveProvider) Configured() bool {
	return p.resolvedKey() != ""
}

func (p *braveProvider) resolvedKey() string {
	if p.apiKey != "" {
		return p.apiKey
	}
	return os.Getenv("BRAVE_API_KEY")
}

type braveResponse struct {
	Web struct {
		Results []braveResultItem `json:"results"`
	} `json:"web"`
}

type braveResultItem struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	PageAge     string `json:"page_age,omitempty"`
}

func (p *braveProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	key := p.resolvedKey()
	if key == "" {
		return nil, fmt.Errorf("api key missing")
	}
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = braveDefaultURL
	}

	params := url.Values{}
	params.Set("q", q)
	params.Set("count", strconv.Itoa(n))

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("X-Subscription-Token", key)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	var out braveResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	results := make([]SearchResult, 0, len(out.Web.Results))
	for _, r := range out.Web.Results {
		results = append(results, SearchResult{
			Title:         r.Title,
			URL:           r.URL,
			Snippet:       r.Description,
			PublishedDate: r.PageAge,
		})
	}
	return results, nil
}
