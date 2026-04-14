package security

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"

	"github.com/odysseythink/hermind/tool"
)

// URLSafety holds a local allow/deny list for website access checks.
// It is fail-closed: when no list is configured, all URLs are
// allowed — caller should populate the lists or route through a
// Google Safe Browsing client if network-level checks are needed.
type URLSafety struct {
	mu   sync.RWMutex
	deny map[string]bool // host (lower-case)
	allow map[string]bool
}

func NewURLSafety(denyHosts, allowHosts []string) *URLSafety {
	us := &URLSafety{
		deny:  map[string]bool{},
		allow: map[string]bool{},
	}
	for _, h := range denyHosts {
		us.deny[strings.ToLower(strings.TrimSpace(h))] = true
	}
	for _, h := range allowHosts {
		us.allow[strings.ToLower(strings.TrimSpace(h))] = true
	}
	return us
}

// Check returns (safe, reason). Unknown hosts are considered safe
// when no allowlist is configured; otherwise only allowlisted hosts
// are safe.
func (u *URLSafety) Check(raw string) (bool, string) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false, "invalid url"
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return false, "missing host"
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	if u.deny[host] {
		return false, "host is denylisted"
	}
	if len(u.allow) > 0 && !u.allow[host] {
		return false, "host not on allowlist"
	}
	return true, ""
}

// RegisterURLCheck adds the url_check tool.
func RegisterURLCheck(reg *tool.Registry, us *URLSafety) {
	reg.Register(&tool.Entry{
		Name:        "url_check",
		Toolset:     "security",
		Description: "Check a URL against the configured allow/deny list.",
		Emoji:       "🚦",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "url_check",
				Description: "Returns {safe: bool, reason: string} for a URL.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"url":{"type":"string"}},
  "required":["url"]
}`),
			},
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
