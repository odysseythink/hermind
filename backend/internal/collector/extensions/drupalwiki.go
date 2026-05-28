package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
)

// DrupalWikiRequest is the payload for the DrupalWiki extension.
type DrupalWikiRequest struct {
	BaseURL     string `json:"baseUrl"`
	AccessToken string `json:"accessToken,omitempty"`
}

// DrupalWikiPage represents a single DrupalWiki page.
type DrupalWikiPage struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// DrupalWikiExtension implements the Extension interface for DrupalWiki.
type DrupalWikiExtension struct {
	httpClient *http.Client
}

// NewDrupalWikiExtension creates a new DrupalWikiExtension.
func NewDrupalWikiExtension() *DrupalWikiExtension {
	return &DrupalWikiExtension{httpClient: &http.Client{}}
}

// NewDrupalWikiExtensionWithClient creates a new DrupalWikiExtension with a custom HTTP client.
func NewDrupalWikiExtensionWithClient(client *http.Client) *DrupalWikiExtension {
	return &DrupalWikiExtension{httpClient: client}
}

// Name returns the extension name.
func (d *DrupalWikiExtension) Name() string { return "drupalwiki" }

// Handle routes DrupalWiki extension requests.
func (d *DrupalWikiExtension) Handle(ctx context.Context, endpoint string, method string, body []byte) (*core.ExtensionResponse, error) {
	if method != http.MethodPost {
		return nil, fmt.Errorf("method %s not supported", method)
	}
	if endpoint != "/ext/drupalwiki" {
		return nil, fmt.Errorf("unknown endpoint %s", endpoint)
	}
	return d.loadPages(ctx, body)
}

func (d *DrupalWikiExtension) loadPages(ctx context.Context, body []byte) (*core.ExtensionResponse, error) {
	var req DrupalWikiRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	if req.BaseURL == "" {
		return nil, fmt.Errorf("baseUrl is required")
	}

	base, err := url.Parse(req.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid baseUrl: %w", err)
	}

	pages, err := d.fetchPages(ctx, base, req.AccessToken)
	if err != nil {
		return nil, err
	}

	return &core.ExtensionResponse{
		Success: true,
		Data:    map[string]interface{}{"pages": pages},
	}, nil
}

func (d *DrupalWikiExtension) fetchPages(ctx context.Context, base *url.URL, token string) ([]DrupalWikiPage, error) {
	// Drupal JSON:API endpoint for wiki pages
	u := base.ResolveReference(&url.URL{Path: "/jsonapi/node/wiki_page"})

	headers := make(http.Header)
	headers.Set("Accept", "application/vnd.api+json")
	if token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header = headers

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("drupalwiki API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Title string `json:"title"`
				Body  struct {
					Value string `json:"value"`
				} `json:"body"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var pages []DrupalWikiPage
	for _, item := range result.Data {
		pages = append(pages, DrupalWikiPage{
			ID:      item.ID,
			Title:   item.Attributes.Title,
			Content: item.Attributes.Body.Value,
		})
	}
	return pages, nil
}
