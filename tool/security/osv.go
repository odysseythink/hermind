// Package security provides defensive-only tools: OSV vulnerability
// lookup, URL safety checking, website policy enforcement, and an
// MCP OAuth client-credentials helper.
package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/odysseythink/hermind/tool"
)

// OSVClient talks to https://api.osv.dev/v1/query.
type OSVClient struct {
	BaseURL string
	http    *http.Client
}

func NewOSVClient(baseURL string) *OSVClient {
	if baseURL == "" {
		baseURL = "https://api.osv.dev"
	}
	return &OSVClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

type osvRequest struct {
	Package struct {
		Ecosystem string `json:"ecosystem"`
		Name      string `json:"name"`
	} `json:"package"`
	Version string `json:"version,omitempty"`
}

type osvResponse struct {
	Vulns []struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
	} `json:"vulns"`
}

// Query returns the list of vulnerability IDs + summaries for a
// package in the given ecosystem at an optional version.
func (c *OSVClient) Query(ctx context.Context, ecosystem, name, version string) ([]string, error) {
	req := osvRequest{Version: version}
	req.Package.Ecosystem = ecosystem
	req.Package.Name = name
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/query", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("osv: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("osv: status %d: %s", resp.StatusCode, string(errBody))
	}
	var out osvResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(out.Vulns))
	for _, v := range out.Vulns {
		lines = append(lines, v.ID+": "+v.Summary)
	}
	return lines, nil
}

// RegisterOSV adds the osv_check tool.
func RegisterOSV(reg *tool.Registry, c *OSVClient) {
	if c == nil {
		return
	}
	reg.Register(&tool.Entry{
		Name:        "osv_check",
		Toolset:     "security",
		Description: "Query OSV for known vulnerabilities in a package version.",
		Emoji:       "🛡",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "osv_check",
				Description: "Check a package (ecosystem + name + version) against the OSV vulnerability database.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "ecosystem":{"type":"string","description":"e.g. Go, npm, PyPI, crates.io"},
    "name":{"type":"string"},
    "version":{"type":"string"}
  },
  "required":["ecosystem","name"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Ecosystem string `json:"ecosystem"`
				Name      string `json:"name"`
				Version   string `json:"version,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if args.Ecosystem == "" || args.Name == "" {
				return tool.ToolError("ecosystem and name are required"), nil
			}
			vulns, err := c.Query(ctx, args.Ecosystem, args.Name, args.Version)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"vulns": vulns}), nil
		},
	})
}
