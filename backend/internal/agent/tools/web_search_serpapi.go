package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/providers"
)

// serpapiProvider searches via SerpApi, supporting 10 sub-engines.
type serpapiProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("serpapi", &serpapiProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *serpapiProvider) Name() string { return "SerpApi" }

func (p *serpapiProvider) Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error) {
	apiKey, err := providers.ResolveAPIKey("serpapi", settings, cfg)
	if err != nil {
		return nil, fmt.Errorf("SerpApi API key not configured. Set AgentSerpApiKey in settings.")
	}

	engine := firstNonEmptyString(
		settings["AgentSerpApiEngine"],
		cfg.AgentSerpApiEngine,
		"google",
	)

	params := url.Values{
		"engine":  {engine},
		"api_key": {apiKey},
	}
	if engine == "amazon" {
		params.Set("k", query)
	} else {
		params.Set("q", query)
	}

	endpoint := "https://serpapi.com/search.json?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("serpapi: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("serpapi: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("serpapi: read body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("serpapi: http %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed serpapiResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("serpapi: decode response: %w", err)
	}

	return normalizeSerpapiResponse(engine, &parsed), nil
}

// serpapiResponse holds the top-level fields we care about across engines.
type serpapiResponse struct {
	KnowledgeGraph *serpapiKnowledgeGraph `json:"knowledge_graph"`
	AnswerBox      *serpapiAnswerBox       `json:"answer_box"`
	OrganicResults []serpapiOrganic        `json:"organic_results"`
	LocalResults   []serpapiLocal          `json:"local_results"`
	ImagesResults  []serpapiImage          `json:"images_results"`
	ShoppingResults []serpapiShopping      `json:"shopping_results"`
	NewsResults    []serpapiNews           `json:"news_results"`
	JobsResults    []serpapiJob            `json:"jobs_results"`
}

type serpapiKnowledgeGraph struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Subtitle    string `json:"subtitle"`
	Type        string `json:"type"`
}

type serpapiAnswerBox struct {
	Title   string `json:"title"`
	Answer  string `json:"answer"`
	Snippet string `json:"snippet"`
}

type serpapiOrganic struct {
	Title         string `json:"title"`
	Link          string `json:"link"`
	Snippet       string `json:"snippet"`
	PatentLink    string `json:"patent_link"`
	PublicationInfo *serpapiPublicationInfo `json:"publication_info"`
}

type serpapiPublicationInfo struct {
	Summary string `json:"summary"`
}

type serpapiLocal struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Website     string `json:"website"`
	Address     string `json:"address"`
	Description string `json:"description"`
}

type serpapiImage struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Source  string `json:"source"`
	Original string `json:"original"`
}

type serpapiShopping struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	ProductLink string `json:"product_link"`
	Snippet     string `json:"snippet"`
	Price       string `json:"price"`
}

type serpapiNews struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
	Source  string `json:"source"`
	Date    string `json:"date"`
}

type serpapiJob struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description"`
	CompanyName string `json:"company_name"`
	Location    string `json:"location"`
}

func normalizeSerpapiResponse(engine string, resp *serpapiResponse) []SearchResult {
	// Support both "google_images" and "google_images_light" forms.
	engine = strings.TrimSuffix(engine, "_light")

	switch engine {
	case "google":
		return serpapiGoogleResults(resp)
	case "baidu":
		return serpapiBaiduResults(resp)
	case "amazon":
		return serpapiAmazonResults(resp)
	case "google_maps":
		return serpapiGoogleMapsResults(resp)
	case "google_images":
		return serpapiGoogleImagesResults(resp)
	case "google_shopping":
		return serpapiGoogleShoppingResults(resp)
	case "google_news":
		return serpapiGoogleNewsResults(resp)
	case "google_jobs":
		return serpapiGoogleJobsResults(resp)
	case "google_patents":
		return serpapiGooglePatentsResults(resp)
	case "google_scholar":
		return serpapiGoogleScholarResults(resp)
	default:
		return serpapiGoogleResults(resp)
	}
}

func serpapiGoogleResults(resp *serpapiResponse) []SearchResult {
	results := make([]SearchResult, 0)
	if resp.KnowledgeGraph != nil {
		results = append(results, SearchResult{
			Title:   firstNonEmptyString(resp.KnowledgeGraph.Title, resp.KnowledgeGraph.Subtitle, resp.KnowledgeGraph.Type),
			Link:    "",
			Snippet: resp.KnowledgeGraph.Description,
		})
	}
	if resp.AnswerBox != nil {
		results = append(results, SearchResult{
			Title:   firstNonEmptyString(resp.AnswerBox.Title, "Answer Box"),
			Link:    "",
			Snippet: firstNonEmptyString(resp.AnswerBox.Answer, resp.AnswerBox.Snippet),
		})
	}
	for _, r := range resp.OrganicResults {
		if r.Title == "" || r.Link == "" {
			continue
		}
		results = append(results, SearchResult{Title: r.Title, Link: r.Link, Snippet: r.Snippet})
	}
	return results
}

func serpapiBaiduResults(resp *serpapiResponse) []SearchResult {
	results := make([]SearchResult, 0)
	if resp.AnswerBox != nil {
		results = append(results, SearchResult{
			Title:   firstNonEmptyString(resp.AnswerBox.Title, "Answer Box"),
			Link:    "",
			Snippet: firstNonEmptyString(resp.AnswerBox.Answer, resp.AnswerBox.Snippet),
		})
	}
	for _, r := range resp.OrganicResults {
		if r.Title == "" || r.Link == "" {
			continue
		}
		results = append(results, SearchResult{Title: r.Title, Link: r.Link, Snippet: r.Snippet})
	}
	return results
}

func serpapiAmazonResults(resp *serpapiResponse) []SearchResult {
	results := make([]SearchResult, 0, len(resp.OrganicResults))
	for _, r := range resp.OrganicResults {
		if r.Title == "" {
			continue
		}
		link := firstNonEmptyString(r.Link)
		if link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title: r.Title,
			Link:  link,
			Snippet: r.Snippet,
		})
	}
	return results
}

func serpapiGoogleMapsResults(resp *serpapiResponse) []SearchResult {
	results := make([]SearchResult, 0, len(resp.LocalResults))
	for _, r := range resp.LocalResults {
		if r.Title == "" {
			continue
		}
		link := firstNonEmptyString(r.Link, r.Website)
		snippet := firstNonEmptyString(r.Description, r.Address)
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    link,
			Snippet: snippet,
		})
	}
	return results
}

func serpapiGoogleImagesResults(resp *serpapiResponse) []SearchResult {
	results := make([]SearchResult, 0, len(resp.ImagesResults))
	for _, r := range resp.ImagesResults {
		if r.Title == "" {
			continue
		}
		link := firstNonEmptyString(r.Link, r.Original)
		if link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    link,
			Snippet: firstNonEmptyString(r.Source),
		})
	}
	return results
}

func serpapiGoogleShoppingResults(resp *serpapiResponse) []SearchResult {
	results := make([]SearchResult, 0, len(resp.ShoppingResults))
	for _, r := range resp.ShoppingResults {
		if r.Title == "" {
			continue
		}
		link := firstNonEmptyString(r.Link, r.ProductLink)
		if link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    link,
			Snippet: firstNonEmptyString(r.Snippet, r.Price),
		})
	}
	return results
}

func serpapiGoogleNewsResults(resp *serpapiResponse) []SearchResult {
	results := make([]SearchResult, 0, len(resp.NewsResults))
	for _, r := range resp.NewsResults {
		if r.Title == "" || r.Link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:         r.Title,
			Link:          r.Link,
			Snippet:       r.Snippet,
			PublishedDate: r.Date,
		})
	}
	return results
}

func serpapiGoogleJobsResults(resp *serpapiResponse) []SearchResult {
	results := make([]SearchResult, 0, len(resp.JobsResults))
	for _, r := range resp.JobsResults {
		if r.Title == "" {
			continue
		}
		snippet := firstNonEmptyString(r.Description, r.CompanyName, r.Location)
		results = append(results, SearchResult{
			Title: r.Title,
			Link:  r.Link,
			Snippet: snippet,
		})
	}
	return results
}

func serpapiGooglePatentsResults(resp *serpapiResponse) []SearchResult {
	results := make([]SearchResult, 0, len(resp.OrganicResults))
	for _, r := range resp.OrganicResults {
		if r.Title == "" {
			continue
		}
		link := firstNonEmptyString(r.PatentLink, r.Link)
		if link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    link,
			Snippet: r.Snippet,
		})
	}
	return results
}

func serpapiGoogleScholarResults(resp *serpapiResponse) []SearchResult {
	results := make([]SearchResult, 0, len(resp.OrganicResults))
	for _, r := range resp.OrganicResults {
		if r.Title == "" || r.Link == "" {
			continue
		}
		snippet := r.Snippet
		if snippet == "" && r.PublicationInfo != nil {
			snippet = r.PublicationInfo.Summary
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    r.Link,
			Snippet: snippet,
		})
	}
	return results
}
