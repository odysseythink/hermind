package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/scraper"
)

// YouTubeRequest is the payload for the YouTube extension.
type YouTubeRequest struct {
	URL string `json:"url"`
}

// YouTubeExtension implements the Extension interface for YouTube transcripts.
type YouTubeExtension struct {
	scraper *scraper.YouTubeScraper
}

// NewYouTubeExtension creates a new YouTubeExtension.
func NewYouTubeExtension() *YouTubeExtension {
	return &YouTubeExtension{scraper: scraper.NewYouTubeScraper()}
}

// Name returns the extension name.
func (y *YouTubeExtension) Name() string { return "youtube-transcript" }

// Handle routes YouTube extension requests.
func (y *YouTubeExtension) Handle(ctx context.Context, endpoint string, method string, body []byte) (*core.ExtensionResponse, error) {
	if method != http.MethodPost {
		return nil, fmt.Errorf("method %s not supported", method)
	}
	if endpoint != "/ext/youtube-transcript" {
		return nil, fmt.Errorf("unknown endpoint %s", endpoint)
	}
	return y.extractTranscript(ctx, body)
}

func (y *YouTubeExtension) extractTranscript(ctx context.Context, body []byte) (*core.ExtensionResponse, error) {
	var req YouTubeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	if req.URL == "" {
		return nil, fmt.Errorf("url is required")
	}

	transcript, err := y.scraper.ExtractTranscript(ctx, req.URL)
	if err != nil {
		return nil, fmt.Errorf("extract transcript: %w", err)
	}

	return &core.ExtensionResponse{
		Success: true,
		Data:    map[string]interface{}{"transcript": transcript},
	}, nil
}
