package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/config"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		Model: "anthropic/claude-opus-4-6",
		Providers: map[string]config.ProviderConfig{
			"anthropic": {Provider: "anthropic", Model: "claude-opus-4-6", APIKey: "k"},
		},
	}
	s, err := NewServer(&ServerOpts{
		Config:  cfg,
		Storage: nil,
		Token:   "t",
		Version: "dev-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func authedReq(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestStatus_PublicAccess(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/status", nil)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code = %d, body = %s", rr.Code, rr.Body.String())
	}
	var resp StatusResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Version != "dev-test" {
		t.Errorf("version = %q", resp.Version)
	}
	if resp.StorageDriver != "none" {
		t.Errorf("storage_driver = %q, want none", resp.StorageDriver)
	}
}

func TestModelInfo_PublicAccess(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/model/info", nil)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	var resp ModelInfoResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Model != "anthropic/claude-opus-4-6" {
		t.Errorf("model = %q", resp.Model)
	}
	if !resp.SupportsTools {
		t.Errorf("supports_tools false despite configured provider")
	}
}

func TestConfigGet(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, authedReq("GET", "/api/config", "t"))
	if rr.Code != 200 {
		t.Fatalf("code = %d, body = %s", rr.Code, rr.Body.String())
	}
	var resp ConfigResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Config["model"] != "anthropic/claude-opus-4-6" {
		t.Errorf("config.model = %v", resp.Config["model"])
	}
}

func TestConfigGet_RequiresAuth(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, httptest.NewRequest("GET", "/api/config", nil))
	if rr.Code != 401 {
		t.Errorf("code = %d, want 401", rr.Code)
	}
}

func TestConfigPut_501WithoutPath(t *testing.T) {
	s := newTestServer(t)
	req := authedReq("PUT", "/api/config", "t")
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("code = %d, want 501", rr.Code)
	}
}

func TestIndex_RendersToken(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct == "" || !contains(ct, "html") {
		t.Errorf("content-type = %q", ct)
	}
	if !contains(rr.Body.String(), "t") {
		t.Errorf("token not rendered")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestToolsList_EmptyDefault(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, authedReq("GET", "/api/tools", "t"))
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	var resp ToolsResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Tools == nil {
		t.Errorf("tools is nil (should be empty slice)")
	}
}

func TestProvidersList(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, authedReq("GET", "/api/providers", "t"))
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	var resp ProvidersResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Providers) != 1 || resp.Providers[0].Name != "anthropic" {
		t.Errorf("providers = %+v", resp.Providers)
	}
	// Ensure API keys never leak.
	for _, p := range resp.Providers {
		body, _ := json.Marshal(p)
		if contains(string(body), "api_key") || contains(string(body), `"k"`) {
			t.Errorf("api key leaked: %s", body)
		}
	}
}

func TestSkillsList(t *testing.T) {
	s := newTestServer(t)
	s.opts.Config.Skills.Disabled = []string{"shell"}
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, authedReq("GET", "/api/skills", "t"))
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	var resp SkillsResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Skills) != 1 || resp.Skills[0].Name != "shell" || resp.Skills[0].Enabled {
		t.Errorf("skills = %+v", resp.Skills)
	}
}
