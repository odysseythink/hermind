// Package security provides defensive-only tools: OSV vulnerability
// lookup, URL safety checking, website policy enforcement, and an
// MCP OAuth client-credentials helper.
package security

import (
	"context"
	"encoding/json"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/security/osv"
)

// RegisterOSV adds the osv_check tool.
func RegisterOSV(reg *tool.Registry, c *osv.Client) {
	if c == nil {
		return
	}
	reg.Register(&tool.Entry{
		Name:        "osv_check",
		Toolset:     "security",
		Description: "Query OSV for known vulnerabilities in a package version.",
		Emoji:       "🛡",
		Schema: core.ToolDefinition{
			Name:        "osv_check",
			Description: "Check a package (ecosystem + name + version) against the OSV vulnerability database.",
			Parameters: core.MustSchemaFromJSON([]byte(`{
  "type":"object",
  "properties":{
    "ecosystem":{"type":"string","description":"e.g. Go, npm, PyPI, crates.io"},
    "name":{"type":"string"},
    "version":{"type":"string"}
  },
  "required":["ecosystem","name"]
}`)),
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
