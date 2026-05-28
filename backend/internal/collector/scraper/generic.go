package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/external"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

// GenericScraper handles general web page scraping.
type GenericScraper struct {
	chromedpAdapter *external.ChromedpAdapter
}

// NewGenericScraper creates a new GenericScraper.
func NewGenericScraper(adapter *external.ChromedpAdapter) *GenericScraper {
	return &GenericScraper{chromedpAdapter: adapter}
}

// Fetch attempts to retrieve the page content. It first tries a simple HTTP
// request with goquery text extraction, then falls back to chromedp if the
// content is empty or the request fails.
func (g *GenericScraper) Fetch(ctx context.Context, link string, captureAs string, headers map[string]string) (string, error) {
	content, err := g.fetchHTTP(ctx, link, captureAs, headers)
	if err == nil && strings.TrimSpace(content) != "" {
		return content, nil
	}

	// Fallback to chromedp.
	content, err = g.chromedpAdapter.FetchText(ctx, link, headers)
	if err != nil {
		return "", fmt.Errorf("generic scraper failed for %s: %w", link, err)
	}
	if captureAs == "html" {
		// chromedp FetchText returns innerText; we cannot get raw HTML from it
		// in this fallback, so return what we have.
		return content, nil
	}
	return content, nil
}

func (g *GenericScraper) fetchHTTP(ctx context.Context, link string, captureAs string, headers map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", defaultUserAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if captureAs == "html" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// Remove script and style elements.
	doc.Find("script, style, noscript").Remove()

	text := doc.Find("body").Text()
	// Normalize whitespace.
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n"), nil
}

// ScrapeAndSave fetches the link, builds a Document, computes statistics,
// and persists it to the server documents folder.
func (g *GenericScraper) ScrapeAndSave(ctx context.Context, link string, headers map[string]string, metadata map[string]string, storageDir string, tokenizer *utils.Tokenizer) (*core.ProcessResponse, error) {
	content, err := g.Fetch(ctx, link, "text", headers)
	if err != nil {
		return nil, err
	}

	title := metadata["title"]
	if title == "" {
		title = link
	}

	doc := &core.Document{
		URL:         link,
		Title:       title,
		DocSource:   "URL link uploaded by the user.",
		ChunkSource: "link://" + link,
		PageContent: content,
	}

	// Local enrichment.
	enrichDocument(doc, content, tokenizer)

	filename := utils.SlugifyFilename(title)
	if filename == "" {
		filename = "link"
	}

	doc, err = utils.WriteToServerDocuments(storageDir, doc, filename, false)
	if err != nil {
		return nil, fmt.Errorf("save document: %w", err)
	}

	return &core.ProcessResponse{
		Filename:  doc.Location,
		Success:   true,
		Reason:    "",
		Documents: []core.Document{*doc},
	}, nil
}
