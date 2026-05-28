package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/scraper"
)

// ResyncRequest is the payload for the resync extension.
type ResyncRequest struct {
	Type    string                 `json:"type"`
	Options map[string]interface{} `json:"options"`
}

// ResyncExtension dispatches resync requests to the appropriate extension.
type ResyncExtension struct {
	scraperManager *scraper.Manager
	extensions     map[string]Extension
}

// NewResyncExtension creates a new ResyncExtension.
func NewResyncExtension(manager *scraper.Manager, exts map[string]Extension) *ResyncExtension {
	return &ResyncExtension{
		scraperManager: manager,
		extensions:     exts,
	}
}

// Name returns the extension name.
func (r *ResyncExtension) Name() string { return "resync" }

// Handle routes resync extension requests.
func (r *ResyncExtension) Handle(ctx context.Context, endpoint string, method string, body []byte) (*core.ExtensionResponse, error) {
	if method != http.MethodPost {
		return nil, fmt.Errorf("method %s not supported", method)
	}
	if endpoint != "/ext/resync-source-document" {
		return nil, fmt.Errorf("unknown endpoint %s", endpoint)
	}
	return r.resync(ctx, body)
}

func (r *ResyncExtension) resync(ctx context.Context, body []byte) (*core.ExtensionResponse, error) {
	var req ResyncRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	if req.Type == "" {
		return nil, fmt.Errorf("type is required")
	}

	switch req.Type {
	case "link":
		return r.resyncLink(ctx, req.Options)
	case "youtube":
		return r.resyncYouTube(ctx, req.Options)
	case "confluence":
		return r.resyncExtension(ctx, "confluence", req.Options)
	case "github":
		return r.resyncExtension(ctx, "github", req.Options)
	case "drupalwiki":
		return r.resyncExtension(ctx, "drupalwiki", req.Options)
	case "paperless-ngx":
		return r.resyncExtension(ctx, "paperless-ngx", req.Options)
	default:
		return nil, fmt.Errorf("unsupported resync type: %s", req.Type)
	}
}

func (r *ResyncExtension) resyncLink(ctx context.Context, options map[string]interface{}) (*core.ExtensionResponse, error) {
	url, _ := options["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("url is required for link resync")
	}
	if r.scraperManager == nil {
		return nil, fmt.Errorf("scraper manager not available")
	}
	result, err := r.scraperManager.GetLinkText(ctx, url, "")
	if err != nil {
		return nil, err
	}
	return &core.ExtensionResponse{
		Success: result.Success,
		Data:    map[string]interface{}{"content": result.Content, "url": result.URL},
		Reason:  "",
	}, nil
}

func (r *ResyncExtension) resyncYouTube(ctx context.Context, options map[string]interface{}) (*core.ExtensionResponse, error) {
	url, _ := options["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("url is required for youtube resync")
	}
	ext, ok := r.extensions["youtube-transcript"]
	if !ok {
		ext = NewYouTubeExtension()
	}
	body, err := json.Marshal(YouTubeRequest{URL: url})
	if err != nil {
		return nil, fmt.Errorf("marshal youtube request: %w", err)
	}
	return ext.Handle(ctx, "/ext/youtube-transcript", http.MethodPost, body)
}

func (r *ResyncExtension) resyncExtension(ctx context.Context, name string, options map[string]interface{}) (*core.ExtensionResponse, error) {
	ext, ok := r.extensions[name]
	if !ok {
		return nil, fmt.Errorf("extension %s not available", name)
	}
	body, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", name, err)
	}
	return ext.Handle(ctx, "/ext/"+name, http.MethodPost, body)
}
