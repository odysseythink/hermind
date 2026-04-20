package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/config/descriptor"
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

func TestHandleConfigPut_PreservesSectionSecretOnBlank(t *testing.T) {
	cfg := &config.Config{}
	cfg.Storage.Driver = "postgres"
	cfg.Storage.PostgresURL = "postgres://user:pass@host/db"

	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")

	srv, err := NewServer(&ServerOpts{
		Config:     cfg,
		ConfigPath: path,
		Token:      "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// The frontend ships back the redacted blank for postgres_url; the
	// rest of the config round-trips unchanged.
	putBody := strings.NewReader(`{"config":{"storage":{"driver":"postgres","postgres_url":""}}}`)
	req := httptest.NewRequest("PUT", "/api/config", putBody)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", w.Code, w.Body.String())
	}

	if got := cfg.Storage.PostgresURL; got != "postgres://user:pass@host/db" {
		t.Errorf("PostgresURL = %q, want preserved secret", got)
	}
}

func TestConfigGet_RedactsKeyedMapSecrets(t *testing.T) {
	// Seed a ShapeKeyedMap section so we don't wait on Task 4's providers.
	const key = "__test_redact_keyed_map"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   descriptor.ShapeKeyedMap,
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Label: "Type", Kind: descriptor.FieldEnum,
				Required: true, Enum: []string{"a"}},
			{Name: "api_key", Label: "API key", Kind: descriptor.FieldSecret},
		},
	})

	// The descriptor has yaml key __test_redact_keyed_map but config.Config has
	// no matching struct field. To simulate a populated instance, inject it
	// directly into the map that the GET handler receives. We do that by
	// passing a Config whose yaml-marshaled shape contains the key — easiest
	// is to bolt it onto config.Config.Providers since we're only exercising
	// the redaction walk, not the yaml round-trip. But Providers isn't
	// ShapeKeyedMap yet. Instead, seed the tested handler via a custom Config
	// struct — which we can't define here. Fall back to exercising redaction
	// through yaml.Marshal of a map[string]any we construct ourselves.
	// The redactSectionSecrets function operates on a map[string]any — we
	// call it directly.
	blob := map[string]any{
		key: map[string]any{
			"anthropic_main": map[string]any{
				"provider": "a",
				"api_key":  "sk-real-secret",
			},
			"openai_bot": map[string]any{
				"provider": "a",
				"api_key":  "sk-other-secret",
			},
		},
	}
	RedactSectionSecretsForTest(blob)
	inst1, _ := blob[key].(map[string]any)["anthropic_main"].(map[string]any)
	inst2, _ := blob[key].(map[string]any)["openai_bot"].(map[string]any)
	if inst1["api_key"] != "" {
		t.Errorf("anthropic_main.api_key = %q, want blank", inst1["api_key"])
	}
	if inst2["api_key"] != "" {
		t.Errorf("openai_bot.api_key = %q, want blank", inst2["api_key"])
	}
	if inst1["provider"] != "a" {
		t.Errorf("anthropic_main.provider = %q, want untouched", inst1["provider"])
	}
}

func TestConfigPut_PreservesKeyedMapSecrets(t *testing.T) {
	// Same seeding story. Exercise preserveSectionSecrets via a test hook.
	const key = "__test_preserve_keyed_map"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   descriptor.ShapeKeyedMap,
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Label: "Type", Kind: descriptor.FieldEnum,
				Required: true, Enum: []string{"a"}},
			{Name: "api_key", Label: "API key", Kind: descriptor.FieldSecret},
		},
	})

	current := map[string]any{
		key: map[string]any{
			"anthropic_main": map[string]any{
				"provider": "a",
				"api_key":  "sk-real-secret",
			},
		},
	}
	updated := map[string]any{
		key: map[string]any{
			"anthropic_main": map[string]any{
				"provider": "a",
				"api_key":  "", // blanked — should be restored from current
			},
			"new_instance": map[string]any{
				"provider": "a",
				"api_key":  "sk-freshly-typed",
			},
		},
	}
	PreserveSectionSecretsForTest(updated, current)
	inst1, _ := updated[key].(map[string]any)["anthropic_main"].(map[string]any)
	if inst1["api_key"] != "sk-real-secret" {
		t.Errorf("anthropic_main.api_key = %q, want %q (restored from current)",
			inst1["api_key"], "sk-real-secret")
	}
	inst2, _ := updated[key].(map[string]any)["new_instance"].(map[string]any)
	if inst2["api_key"] != "sk-freshly-typed" {
		t.Errorf("new_instance.api_key = %q, want %q (preserved from updated)",
			inst2["api_key"], "sk-freshly-typed")
	}
}

func TestConfigGet_RedactsProvidersApiKey_Integration(t *testing.T) {
	// Integration test: drive through the real HTTP pipeline (handleConfigGet
	// → yaml.Marshal → yaml.Unmarshal → redactSectionSecrets → writeJSON)
	// rather than the map-walk wrapper used in TestConfigGet_RedactsKeyedMapSecrets.
	// Ensures the yaml round-trip produces the map shape the redact loop assumes.
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic_main": {
				Provider: "anthropic",
				APIKey:   "sk-real-secret",
				Model:    "claude-opus-4-7",
			},
		},
	}
	srv, err := NewServer(&ServerOpts{Config: cfg, Token: "t"})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	req := httptest.NewRequest("GET", "/api/config", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body struct {
		Config map[string]any `json:"config"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	providers, _ := body.Config["providers"].(map[string]any)
	if providers == nil {
		t.Fatalf("providers missing from response: %+v", body.Config)
	}
	inst, _ := providers["anthropic_main"].(map[string]any)
	if inst == nil {
		t.Fatalf("anthropic_main instance missing from response: %+v", providers)
	}
	if ak := inst["api_key"]; ak != "" {
		t.Errorf("anthropic_main.api_key = %v, want blank (redacted)", ak)
	}
	// Non-secret fields must be preserved.
	if inst["provider"] != "anthropic" {
		t.Errorf("anthropic_main.provider = %v, want anthropic", inst["provider"])
	}
	if inst["model"] != "claude-opus-4-7" {
		t.Errorf("anthropic_main.model = %v, want claude-opus-4-7", inst["model"])
	}
}
