package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
)

// PaperlessRequest is the payload for the Paperless extension.
type PaperlessRequest struct {
	BaseURL  string `json:"baseUrl"`
	APIToken string `json:"apiToken"`
}

// PaperlessDocument represents a single Paperless document.
type PaperlessDocument struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	CreatedDate string `json:"createdDate"`
}

// PaperlessExtension implements the Extension interface for Paperless-ngx.
type PaperlessExtension struct {
	httpClient *http.Client
}

// NewPaperlessExtension creates a new PaperlessExtension.
func NewPaperlessExtension() *PaperlessExtension {
	return &PaperlessExtension{httpClient: &http.Client{}}
}

// NewPaperlessExtensionWithClient creates a new PaperlessExtension with a custom HTTP client.
func NewPaperlessExtensionWithClient(client *http.Client) *PaperlessExtension {
	return &PaperlessExtension{httpClient: client}
}

// Name returns the extension name.
func (p *PaperlessExtension) Name() string { return "paperless-ngx" }

// Handle routes Paperless extension requests.
func (p *PaperlessExtension) Handle(ctx context.Context, endpoint string, method string, body []byte) (*core.ExtensionResponse, error) {
	if method != http.MethodPost {
		return nil, fmt.Errorf("method %s not supported", method)
	}
	if endpoint != "/ext/paperless-ngx" {
		return nil, fmt.Errorf("unknown endpoint %s", endpoint)
	}
	return p.loadDocuments(ctx, body)
}

func (p *PaperlessExtension) loadDocuments(ctx context.Context, body []byte) (*core.ExtensionResponse, error) {
	var req PaperlessRequest
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

	var docs []PaperlessDocument
	page := 1
	for {
		batch, hasMore, err := p.fetchDocuments(ctx, base, req.APIToken, page)
		if err != nil {
			return nil, err
		}
		docs = append(docs, batch...)
		if !hasMore {
			break
		}
		page++
	}

	return &core.ExtensionResponse{
		Success: true,
		Data:    map[string]interface{}{"documents": docs},
	}, nil
}

func (p *PaperlessExtension) fetchDocuments(ctx context.Context, base *url.URL, token string, page int) ([]PaperlessDocument, bool, error) {
	u := base.ResolveReference(&url.URL{Path: "/api/documents/"})
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()

	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	if token != "" {
		headers.Set("Authorization", "Token "+token)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, false, err
	}
	req.Header = headers

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("paperless API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Results []struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			Content     string `json:"content"`
			CreatedDate string `json:"created"`
		} `json:"results"`
		Next string `json:"next"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, fmt.Errorf("decode response: %w", err)
	}

	var docs []PaperlessDocument
	for _, r := range result.Results {
		docs = append(docs, PaperlessDocument{
			ID:          r.ID,
			Title:       r.Title,
			Content:     r.Content,
			CreatedDate: r.CreatedDate,
		})
	}
	return docs, result.Next != "", nil
}
