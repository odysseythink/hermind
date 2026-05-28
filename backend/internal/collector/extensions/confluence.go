package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
)

// ConfluenceRequest is the payload for the Confluence extension.
type ConfluenceRequest struct {
	BaseURL     string `json:"baseUrl"`
	SpaceKey    string `json:"spaceKey"`
	AccessToken string `json:"accessToken"`
	Username    string `json:"username,omitempty"`
}

// ConfluencePage represents a single Confluence page.
type ConfluencePage struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// ConfluenceExtension implements the Extension interface for Confluence.
type ConfluenceExtension struct {
	httpClient *http.Client
}

// NewConfluenceExtension creates a new ConfluenceExtension.
func NewConfluenceExtension() *ConfluenceExtension {
	return &ConfluenceExtension{httpClient: &http.Client{}}
}

// NewConfluenceExtensionWithClient creates a new ConfluenceExtension with a custom HTTP client.
func NewConfluenceExtensionWithClient(client *http.Client) *ConfluenceExtension {
	return &ConfluenceExtension{httpClient: client}
}

// Name returns the extension name.
func (c *ConfluenceExtension) Name() string { return "confluence" }

// Handle routes Confluence extension requests.
func (c *ConfluenceExtension) Handle(ctx context.Context, endpoint string, method string, body []byte) (*core.ExtensionResponse, error) {
	if method != http.MethodPost {
		return nil, fmt.Errorf("method %s not supported", method)
	}
	if endpoint != "/ext/confluence" {
		return nil, fmt.Errorf("unknown endpoint %s", endpoint)
	}
	return c.loadSpace(ctx, body)
}

func (c *ConfluenceExtension) loadSpace(ctx context.Context, body []byte) (*core.ExtensionResponse, error) {
	var req ConfluenceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	if req.BaseURL == "" || req.SpaceKey == "" {
		return nil, fmt.Errorf("baseUrl and spaceKey are required")
	}

	base, err := url.Parse(req.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid baseUrl: %w", err)
	}

	var pages []ConfluencePage
	start := 0
	limit := 25

	for {
		batch, next, err := c.fetchPages(ctx, base, req.SpaceKey, req.Username, req.AccessToken, start, limit)
		if err != nil {
			return nil, err
		}
		pages = append(pages, batch...)
		if next < 0 {
			break
		}
		start = next
	}

	return &core.ExtensionResponse{
		Success: true,
		Data:    map[string]interface{}{"pages": pages},
	}, nil
}

func (c *ConfluenceExtension) fetchPages(ctx context.Context, base *url.URL, spaceKey, username, token string, start, limit int) ([]ConfluencePage, int, error) {
	// Confluence REST API v2: GET /wiki/api/v2/spaces/{spaceKey}/pages
	u := base.ResolveReference(&url.URL{Path: "/wiki/api/v2/spaces/" + spaceKey + "/pages"})
	q := u.Query()
	q.Set("limit", strconv.Itoa(limit))
	q.Set("start", strconv.Itoa(start))
	u.RawQuery = q.Encode()

	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	if username != "" && token != "" {
		headers.Set("Authorization", "Basic "+basicAuth(username, token))
	} else if token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, -1, err
	}
	req.Header = headers

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, -1, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, -1, fmt.Errorf("confluence API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Results []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"results"`
		Links struct {
			Next string `json:"next"`
		} `json:"_links"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, -1, fmt.Errorf("decode response: %w", err)
	}

	var pages []ConfluencePage
	for _, r := range result.Results {
		content, err := c.fetchPageContent(ctx, base, r.ID, username, token)
		if err != nil {
			continue
		}
		pages = append(pages, ConfluencePage{
			ID:      r.ID,
			Title:   r.Title,
			Content: content,
		})
	}

	nextStart := -1
	if result.Links.Next != "" {
		nextStart = start + limit
	}
	return pages, nextStart, nil
}

func (c *ConfluenceExtension) fetchPageContent(ctx context.Context, base *url.URL, pageID, username, token string) (string, error) {
	u := base.ResolveReference(&url.URL{Path: "/wiki/api/v2/pages/" + pageID})
	q := u.Query()
	q.Set("body-format", "storage")
	u.RawQuery = q.Encode()

	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	if username != "" && token != "" {
		headers.Set("Authorization", "Basic "+basicAuth(username, token))
	} else if token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header = headers

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result struct {
		Body struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return stripHTMLTags(result.Body.Storage.Value), nil
}

func basicAuth(username, password string) string {
	return base64Encode(username + ":" + password)
}

func base64Encode(s string) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var out strings.Builder
	var buf [3]byte
	var n int
	for i := 0; i < len(s); i++ {
		buf[n] = s[i]
		n++
		if n == 3 {
			b := uint(buf[0])<<16 | uint(buf[1])<<8 | uint(buf[2])
			out.WriteByte(charset[b>>18&0x3F])
			out.WriteByte(charset[b>>12&0x3F])
			out.WriteByte(charset[b>>6&0x3F])
			out.WriteByte(charset[b&0x3F])
			n = 0
		}
	}
	if n > 0 {
		b := uint(buf[0]) << 16
		if n == 2 {
			b |= uint(buf[1]) << 8
		}
		out.WriteByte(charset[b>>18&0x3F])
		out.WriteByte(charset[b>>12&0x3F])
		if n == 2 {
			out.WriteByte(charset[b>>6&0x3F])
		} else {
			out.WriteByte('=')
		}
		out.WriteByte('=')
	}
	return out.String()
}

func stripHTMLTags(html string) string {
	var out strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
}
