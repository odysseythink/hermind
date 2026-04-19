package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
