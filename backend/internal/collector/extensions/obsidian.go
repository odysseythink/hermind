package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
)

// ObsidianRequest is the payload for the Obsidian extension.
type ObsidianRequest struct {
	VaultPath string `json:"vaultPath"`
}

// ObsidianFile represents a single file from an Obsidian vault.
type ObsidianFile struct {
	Name        string                 `json:"name"`
	Path        string                 `json:"path"`
	Content     string                 `json:"content"`
	FrontMatter map[string]interface{} `json:"frontMatter,omitempty"`
}

// ObsidianExtension implements the Extension interface for Obsidian vaults.
type ObsidianExtension struct{}

// NewObsidianExtension creates a new ObsidianExtension.
func NewObsidianExtension() *ObsidianExtension {
	return &ObsidianExtension{}
}

// Name returns the extension name.
func (o *ObsidianExtension) Name() string { return "obsidian" }

// Handle routes Obsidian extension requests.
func (o *ObsidianExtension) Handle(ctx context.Context, endpoint string, method string, body []byte) (*core.ExtensionResponse, error) {
	if method != http.MethodPost {
		return nil, fmt.Errorf("method %s not supported", method)
	}
	if endpoint != "/ext/obsidian/vault" {
		return nil, fmt.Errorf("unknown endpoint %s", endpoint)
	}
	return o.loadVault(ctx, body)
}

func (o *ObsidianExtension) loadVault(ctx context.Context, body []byte) (*core.ExtensionResponse, error) {
	var req ObsidianRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	if req.VaultPath == "" {
		return nil, fmt.Errorf("vaultPath is required")
	}

	var files []ObsidianFile
	if err := o.walkVault(req.VaultPath, &files); err != nil {
		return nil, err
	}

	return &core.ExtensionResponse{
		Success: true,
		Data:    map[string]interface{}{"files": files},
	}, nil
}

func (o *ObsidianExtension) walkVault(root string, files *[]ObsidianFile) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if info.IsDir() {
			return nil
		}
		if !isSupportedExt(filepath.Ext(info.Name())) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		text := string(content)
		fm, body := extractFrontMatter(text)
		rel, _ := filepath.Rel(root, path)

		*files = append(*files, ObsidianFile{
			Name:        info.Name(),
			Path:        rel,
			Content:     body,
			FrontMatter: fm,
		})
		return nil
	})
}

func extractFrontMatter(content string) (map[string]interface{}, string) {
	if !strings.HasPrefix(content, "---") {
		return nil, content
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, content
	}
	var fm map[string]interface{}
	if err := json.Unmarshal([]byte(parts[1]), &fm); err != nil {
		return nil, content
	}
	return fm, strings.TrimSpace(parts[2])
}
