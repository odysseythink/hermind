package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

func TestHandleConfigPut_PreservesUnchangedSecret(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	cfg := &config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"tg_main": {
			Enabled: true,
			Type:    "telegram",
			Options: map[string]string{"token": "live-token"},
		},
	}
	if err := config.SaveToPath(path, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	srv, err := NewServer(&ServerOpts{
		Config:     cfg,
		ConfigPath: path,
		Token:      "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	put := `{"config":{"gateway":{"platforms":{"tg_main":{"enabled":true,"type":"telegram","options":{"token":""}}}}}}`
	req := httptest.NewRequest("PUT", "/api/config", strings.NewReader(put))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := cfg.Gateway.Platforms["tg_main"].Options["token"]; got != "live-token" {
		t.Errorf("in-memory token = %q, want %q (preserved)", got, "live-token")
	}
}

func TestHandleConfigPut_OverwritesSecretWhenProvided(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	cfg := &config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"tg_main": {
			Enabled: true,
			Type:    "telegram",
			Options: map[string]string{"token": "old-token"},
		},
	}
	if err := config.SaveToPath(path, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	srv, err := NewServer(&ServerOpts{
		Config:     cfg,
		ConfigPath: path,
		Token:      "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	put := `{"config":{"gateway":{"platforms":{"tg_main":{"enabled":true,"type":"telegram","options":{"token":"new-token"}}}}}}`
	req := httptest.NewRequest("PUT", "/api/config", strings.NewReader(put))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := cfg.Gateway.Platforms["tg_main"].Options["token"]; got != "new-token" {
		t.Errorf("in-memory token = %q, want %q (overwritten)", got, "new-token")
	}
}

func TestHandleConfigGet_RedactsSectionSecretFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.Storage.Driver = "postgres"
	cfg.Storage.PostgresURL = "postgres://user:pass@host/db"

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
		t.Fatalf("status = %d", w.Code)
	}
	var body struct {
		Config map[string]any `json:"config"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	storage, ok := body.Config["storage"].(map[string]any)
	if !ok {
		t.Fatalf("storage section missing: %+v", body.Config)
	}
	if got := storage["postgres_url"]; got != "" {
		t.Errorf("postgres_url = %v, want blank (redacted)", got)
	}
	// Sanity check: non-secret fields are NOT blanked.
	if got := storage["driver"]; got != "postgres" {
		t.Errorf("driver = %v, want \"postgres\"", got)
	}
}
