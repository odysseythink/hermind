package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestHandleConfigGet_RedactsSecretFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"tg_main": {
			Enabled: true,
			Type:    "telegram",
			Options: map[string]string{"token": "super-secret-123"},
		},
	}
	srv, err := NewServer(&ServerOpts{
		Config: cfg,
		Token:  "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/config", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body struct {
		Config map[string]any `json:"config"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	gw, ok := body.Config["gateway"].(map[string]any)
	if !ok {
		t.Fatalf("body.config.gateway missing: %v", body.Config)
	}
	pls, ok := gw["platforms"].(map[string]any)
	if !ok {
		t.Fatalf("body.config.gateway.platforms missing: %v", gw)
	}
	inst, ok := pls["tg_main"].(map[string]any)
	if !ok {
		t.Fatalf("tg_main missing: %v", pls)
	}
	options, ok := inst["options"].(map[string]any)
	if !ok {
		t.Fatalf("options missing: %v", inst)
	}
	if got := options["token"]; got != "" {
		t.Errorf("options.token = %v, want \"\" (redacted)", got)
	}
}
