package agent

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/mlog"
)

// originMatcher encapsulates the allowed-origin logic for WebSocket CORS.
type originMatcher struct {
	exact     map[string]bool
	suffix    []string // "example.com" matches "a.example.com"
	anyOrigin bool
}

func parseAllowedOrigins(raw string) *originMatcher {
	raw = strings.TrimSpace(raw)
	if raw == "*" {
		return &originMatcher{anyOrigin: true}
	}
	m := &originMatcher{exact: map[string]bool{}}
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if strings.HasPrefix(o, "*.") {
			m.suffix = append(m.suffix, strings.TrimPrefix(o, "*."))
		} else {
			m.exact[o] = true
		}
	}
	return m
}

func (m *originMatcher) match(origin, requestHost string) bool {
	if origin == "" {
		return true // non-browser client
	}
	if m.anyOrigin {
		return true
	}
	if m.exact[origin] {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	for _, suf := range m.suffix {
		// strict: host must END with "." + suf (no prefix injection)
		if strings.HasSuffix(u.Host, "."+suf) {
			return true
		}
	}
	// empty config → same-host fallback
	if len(m.exact) == 0 && len(m.suffix) == 0 && !m.anyOrigin {
		return u.Host == requestHost
	}
	return false
}

// buildCheckOrigin returns a CheckOrigin function for the WebSocket upgrader
// based on the config.AgentAllowedOrigins CSV.
//
// Behavior:
//   - "" (default): allow only when Origin matches Host header (same-host)
//   - "*": allow any origin (logs a startup warning)
//   - "https://a.com,https://b.com": exact match against the list
//   - "*.example.com": suffix match — any subdomain of example.com
//   - Missing Origin header: allowed (non-browser clients like curl/CI)
func buildCheckOrigin(cfg *config.Config) func(*http.Request) bool {
	raw := strings.TrimSpace(cfg.AgentAllowedOrigins)
	if raw == "*" {
		mlog.Warning("agent: AGENT_ALLOWED_ORIGINS=* — allowing any origin. Tighten in production.")
	}
	m := parseAllowedOrigins(raw)
	return func(r *http.Request) bool {
		return m.match(r.Header.Get("Origin"), r.Host)
	}
}
