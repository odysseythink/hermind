package security

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nousresearch/hermes-agent/tool"
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
	c := NewOSVClient(srv.URL)
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

func TestMCPOAuthFetchToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "client_credentials" {
			t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
		}
		if r.Form.Get("client_id") != "app" {
			t.Errorf("client_id = %q", r.Form.Get("client_id"))
		}
		_, _ = w.Write([]byte(`{"access_token":"at_123","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()
	c := NewMCPOAuthClient(srv.URL, "app", "secret", "read")
	tok, err := c.FetchToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tok != "at_123" {
		t.Errorf("token = %q", tok)
	}
}
