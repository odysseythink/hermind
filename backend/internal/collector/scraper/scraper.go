package scraper

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/external"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
)

// Manager orchestrates link scraping.
type Manager struct {
	chromedpAdapter *external.ChromedpAdapter
	tokenizer       *utils.Tokenizer
	generic         *GenericScraper
	youtube         *YouTubeScraper
}

// NewManager creates a new scraping Manager.
func NewManager(adapter *external.ChromedpAdapter, tokenizer *utils.Tokenizer) *Manager {
	return &Manager{
		chromedpAdapter: adapter,
		tokenizer:       tokenizer,
		generic:         NewGenericScraper(adapter),
		youtube:         NewYouTubeScraper(),
	}
}

// Scrape validates the URL, routes to the appropriate scraper, and optionally
// persists the result as a Document.
func (m *Manager) Scrape(ctx context.Context, link string, captureAs string, headers map[string]string, saveAsDocument bool, metadata map[string]string, storageDir string) (*core.ProcessResponse, error) {
	link = validateURL(link)
	if !validURL(link) {
		return nil, fmt.Errorf("invalid URL: %s", link)
	}

	var content string
	var err error
	isYouTube := isYouTubeURL(link)

	if isYouTube {
		content, err = m.youtube.ExtractTranscript(ctx, link)
	} else {
		content, err = m.generic.Fetch(ctx, link, captureAs, headers)
	}
	if err != nil {
		return nil, err
	}

	title := metadata["title"]
	if title == "" {
		title = link
	}

	doc := core.Document{
		URL:         link,
		Title:       title,
		DocSource:   "URL link uploaded by the user.",
		ChunkSource: "link://" + link,
		PageContent: content,
	}

	enrichDocument(&doc, content, m.tokenizer)

	if saveAsDocument {
		filename := utils.SlugifyFilename(title)
		if filename == "" {
			if isYouTube {
				filename = "youtube-link"
			} else {
				filename = "link"
			}
		}
		docPtr, err := utils.WriteToServerDocuments(storageDir, &doc, filename, false)
		if err != nil {
			return nil, fmt.Errorf("save document: %w", err)
		}
		doc = *docPtr
	}

	return &core.ProcessResponse{
		Filename:  doc.Location,
		Success:   true,
		Reason:    "",
		Documents: []core.Document{doc},
	}, nil
}

// GetLinkText fetches the raw content (text or html) for a link.
func (m *Manager) GetLinkText(ctx context.Context, link string, captureAs string) (*core.LinkContentResponse, error) {
	link = validateURL(link)
	if !validURL(link) {
		return nil, fmt.Errorf("invalid URL: %s", link)
	}

	var content string
	var err error

	if isYouTubeURL(link) {
		content, err = m.youtube.ExtractTranscript(ctx, link)
	} else {
		content, err = m.generic.Fetch(ctx, link, captureAs, nil)
	}
	if err != nil {
		return &core.LinkContentResponse{
			URL:     link,
			Success: false,
			Content: err.Error(),
		}, nil
	}

	return &core.LinkContentResponse{
		URL:     link,
		Success: true,
		Content: content,
	}, nil
}

// enrichDocument populates word and token counts on a Document.
func enrichDocument(doc *core.Document, content string, tokenizer *utils.Tokenizer) {
	doc.WordCount = len(strings.Fields(content))
	doc.TokenCountEstimate = tokenizer.Count(content)
}
