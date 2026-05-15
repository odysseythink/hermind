package security

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/security/osv"
)

func TestOSVQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"name":"lodash"`) {
			t.Errorf("bad body: %s", body)
		}
		_, _ = w.Write([]byte(`{"vulns":[{"id":"CVE-2020-1","summary":"issue"}]}`))
	}))
	defer srv.Close()
	c := osv.New(srv.URL)
	reg := tool.NewRegistry()
	RegisterOSV(reg, c)
	args, _ := json.Marshal(map[string]string{"ecosystem": "npm", "name": "lodash", "version": "4.17.15"})
	res, err := reg.Dispatch(context.Background(), "osv_check", args)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(res, "CVE-2020-1") {
		t.Errorf("missing cve: %s", res)
	}
}

func TestURLCheck(t *testing.T) {
	us := NewURLSafety([]string{"evil.example.com"}, nil)
	reg := tool.NewRegistry()
	RegisterURLCheck(reg, us)
	// Denylisted host.
	args, _ := json.Marshal(map[string]string{"url": "https://evil.example.com/foo"})
	res, _ := reg.Dispatch(context.Background(), "url_check", args)
	if !strings.Contains(res, `"safe":false`) {
		t.Errorf("expected unsafe, got %s", res)
	}
	// Unknown host with no allowlist → safe.
	args2, _ := json.Marshal(map[string]string{"url": "https://example.com"})
	res2, _ := reg.Dispatch(context.Background(), "url_check", args2)
	if !strings.Contains(res2, `"safe":true`) {
		t.Errorf("expected safe, got %s", res2)
	}
}

func TestURLCheckAllowlist(t *testing.T) {
	us := NewURLSafety(nil, []string{"example.com"})
	ok, _ := us.Check("https://example.com/x")
	if !ok {
		t.Error("example.com should be on allowlist")
	}
	ok, _ = us.Check("https://other.com/x")
	if ok {
		t.Error("other.com should be blocked when allowlist is set")
	}
}

func TestOSVNoVulns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"vulns":[]}`))
	}))
	defer srv.Close()
	c := osv.New(srv.URL)
	vulns, err := c.Query(context.Background(), "npm", "safe-pkg", "1.0.0")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(vulns) != 0 {
		t.Errorf("expected no vulns, got %v", vulns)
	}
}

func TestOSVInvalidArgs(t *testing.T) {
	c := osv.New("")
	reg := tool.NewRegistry()
	RegisterOSV(reg, c)
	out, _ := reg.Dispatch(context.Background(), "osv_check", json.RawMessage(`{}`))
	if !strings.Contains(out, "ecosystem and name are required") {
		t.Errorf("expected validation error, got %s", out)
	}
}

func TestURLCheckInvalidURL(t *testing.T) {
	us := NewURLSafety(nil, nil)
	safe, reason := us.Check("://not-a-url")
	if safe {
		t.Error("invalid URL should not be safe")
	}
	if !strings.Contains(reason, "invalid") {
		t.Errorf("reason = %q", reason)
	}
}

func TestURLCheckMissingHost(t *testing.T) {
	us := NewURLSafety(nil, nil)
	safe, reason := us.Check("file:///no-host")
	if safe {
		t.Error("URL without host should not be safe")
	}
	if !strings.Contains(reason, "missing host") {
		t.Errorf("reason = %q", reason)
	}
}
