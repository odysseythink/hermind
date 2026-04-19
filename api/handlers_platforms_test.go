package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/config"
)

func TestPlatformsSchema_ContainsAllRegisteredTypes(t *testing.T) {
	srv, err := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/platforms/schema", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body api.PlatformsSchemaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{
		"api_server", "acp", "webhook",
		"telegram", "discord", "discord_bot",
		"slack", "slack_events",
		"mattermost", "mattermost_bot",
		"feishu", "dingtalk", "wecom",
		"matrix", "signal", "whatsapp",
		"homeassistant", "email", "sms",
	}
	have := map[string]bool{}
	for _, d := range body.Descriptors {
		have[d.Type] = true
	}
	for _, t0 := range want {
		if !have[t0] {
			t.Errorf("missing descriptor: %q", t0)
		}
	}
}

func TestPlatformsSchema_TelegramFieldShape(t *testing.T) {
	srv, err := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/platforms/schema", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var body api.PlatformsSchemaResponse
	_ = json.Unmarshal(w.Body.Bytes(), &body)

	var tg *api.SchemaDescriptorDTO
	for i := range body.Descriptors {
		if body.Descriptors[i].Type == "telegram" {
			tg = &body.Descriptors[i]
			break
		}
	}
	if tg == nil {
		t.Fatal("telegram descriptor not in response")
	}
	if len(tg.Fields) != 1 {
		t.Fatalf("telegram fields = %d, want 1", len(tg.Fields))
	}
	if tg.Fields[0].Kind != "secret" {
		t.Errorf("token kind = %q, want secret", tg.Fields[0].Kind)
	}
	if !tg.Fields[0].Required {
		t.Errorf("token.Required = false, want true")
	}
}

func TestPlatformsSchema_RequiresAuth(t *testing.T) {
	srv, err := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/platforms/schema", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestPlatformReveal_ReturnsSecretValue(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"tg_main": {
			Enabled: true, Type: "telegram",
			Options: map[string]string{"token": "live-token"},
		},
	}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/tg_main/reveal", strings.NewReader(`{"field":"token"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body api.RevealResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if body.Value != "live-token" {
		t.Errorf("value = %q, want %q", body.Value, "live-token")
	}
}

func TestPlatformReveal_RejectsNonSecretField(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"slack_ops": {
			Enabled: true, Type: "slack_events",
			Options: map[string]string{"addr": ":9000", "bot_token": "xoxb-y"},
		},
	}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/slack_ops/reveal", strings.NewReader(`{"field":"addr"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestPlatformReveal_404OnUnknownKey(t *testing.T) {
	cfg := &config.Config{}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/missing/reveal", strings.NewReader(`{"field":"token"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
