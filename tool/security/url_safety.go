package security

import (
	"context"
	"encoding/json"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/security/urlsafety"
)

// URLSafety is a hermind alias for the pantheon URL safety policy.
type URLSafety = urlsafety.Policy

// NewURLSafety delegates to pantheon/security/urlsafety.
func NewURLSafety(denyHosts, allowHosts []string) *URLSafety {
	return urlsafety.New(denyHosts, allowHosts)
}

// RegisterURLCheck adds the url_check tool.
func RegisterURLCheck(reg *tool.Registry, us *URLSafety) {
	reg.Register(&tool.Entry{
		Name:        "url_check",
		Toolset:     "security",
		Description: "Check a URL against the configured allow/deny list.",
		Emoji:       "🚦",
		Schema: core.ToolDefinition{
			Name:        "url_check",
			Description: "Returns {safe: bool, reason: string} for a URL.",
			Parameters: core.MustSchemaFromJSON([]byte(`{
  "type":"object",
  "properties":{"url":{"type":"string"}},
  "required":["url"]
}`)),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct{ URL string `json:"url"` }
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			safe, reason := us.Check(args.URL)
			return tool.ToolResult(map[string]any{"safe": safe, "reason": reason}), nil
		},
	})
}
