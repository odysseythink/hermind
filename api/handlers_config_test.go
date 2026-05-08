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


func TestHandleConfigGet_RedactsSectionSecretFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.Storage.Driver = "postgres"
	cfg.Storage.PostgresURL = "postgres://user:pass@host/db"

	srv, err := NewServer(&ServerOpts{
		Config: cfg,

	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/config", nil)
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

	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// The frontend ships back the redacted blank for postgres_url; the
	// rest of the config round-trips unchanged.
	putBody := strings.NewReader(`{"config":{"storage":{"driver":"postgres","postgres_url":""}}}`)
	req := httptest.NewRequest("PUT", "/api/config", putBody)
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

func TestConfigGet_RedactsListSecrets(t *testing.T) {
	// Seed a ShapeList section so we don't wait on Task 4's fallback_providers.
	const key = "__test_redact_list"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   descriptor.ShapeList,
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Label: "Type", Kind: descriptor.FieldEnum,
				Required: true, Enum: []string{"a"}},
			{Name: "api_key", Label: "API key", Kind: descriptor.FieldSecret},
		},
	})

	blob := map[string]any{
		key: []any{
			map[string]any{"provider": "a", "api_key": "sk-one"},
			map[string]any{"provider": "a", "api_key": "sk-two"},
		},
	}
	RedactSectionSecretsForTest(blob)
	list, _ := blob[key].([]any)
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	for i, raw := range list {
		inner := raw.(map[string]any)
		if inner["api_key"] != "" {
			t.Errorf("[%d] api_key = %q, want blank", i, inner["api_key"])
		}
		if inner["provider"] != "a" {
			t.Errorf("[%d] provider mutated: %q", i, inner["provider"])
		}
	}
}

func TestConfigPut_PreservesListSecretsByIndex(t *testing.T) {
	// Preserve is strictly by index: updated[i].api_key == "" AND
	// current[i] has a non-empty api_key → restore current[i].api_key.
	// If updated has no current counterpart at index i, leave blank.
	const key = "__test_preserve_list"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   descriptor.ShapeList,
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Label: "Type", Kind: descriptor.FieldEnum,
				Required: true, Enum: []string{"a", "b"}},
			{Name: "api_key", Label: "API key", Kind: descriptor.FieldSecret},
		},
	})

	current := map[string]any{
		key: []any{
			map[string]any{"provider": "a", "api_key": "sk-zero"},
			map[string]any{"provider": "b", "api_key": "sk-one"},
		},
	}
	updated := map[string]any{
		key: []any{
			map[string]any{"provider": "a", "api_key": ""},        // present in current → restore
			map[string]any{"provider": "b", "api_key": "sk-new"},  // user retyped → keep
			map[string]any{"provider": "a", "api_key": ""},        // appended, no counterpart → stay blank
		},
	}
	PreserveSectionSecretsForTest(updated, current)
	got, _ := updated[key].([]any)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].(map[string]any)["api_key"] != "sk-zero" {
		t.Errorf("[0] api_key = %q, want %q", got[0].(map[string]any)["api_key"], "sk-zero")
	}
	if got[1].(map[string]any)["api_key"] != "sk-new" {
		t.Errorf("[1] api_key = %q, want preserved", got[1].(map[string]any)["api_key"])
	}
	if got[2].(map[string]any)["api_key"] != "" {
		t.Errorf("[2] api_key = %q, want blank (no current counterpart)", got[2].(map[string]any)["api_key"])
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
	srv, err := NewServer(&ServerOpts{Config: cfg})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	req := httptest.NewRequest("GET", "/api/config", nil)
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

func TestRedactSectionSecrets_DottedPath(t *testing.T) {
	const key = "dotted_redact_test"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Dotted Redact",
		GroupID: "runtime",
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Label: "Provider", Kind: descriptor.FieldEnum,
				Enum: []string{"", "a"}},
			{Name: "a.api_key", Label: "A API key", Kind: descriptor.FieldSecret},
		},
	})
	t.Cleanup(func() { descriptor.Unregister(key) })

	blob := map[string]any{
		key: map[string]any{
			"provider": "a",
			"a": map[string]any{
				"api_key": "sk-dotted-secret",
			},
		},
	}
	RedactSectionSecretsForTest(blob)
	outer, _ := blob[key].(map[string]any)
	inner, _ := outer["a"].(map[string]any)
	if inner["api_key"] != "" {
		t.Errorf("%s.a.api_key = %q, want \"\" (redacted)", key, inner["api_key"])
	}
}

func TestPreserveSectionSecrets_DottedPath(t *testing.T) {
	const key = "dotted_preserve_test"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Dotted Preserve",
		GroupID: "runtime",
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Label: "Provider", Kind: descriptor.FieldEnum,
				Enum: []string{"", "a"}},
			{Name: "a.api_key", Label: "A API key", Kind: descriptor.FieldSecret},
		},
	})
	t.Cleanup(func() { descriptor.Unregister(key) })

	current := map[string]any{
		key: map[string]any{
			"provider": "a",
			"a":        map[string]any{"api_key": "sk-real"},
		},
	}
	updated := map[string]any{
		key: map[string]any{
			"provider": "a",
			"a":        map[string]any{"api_key": ""}, // blanked by redact
		},
	}
	PreserveSectionSecretsForTest(updated, current)
	outer, _ := updated[key].(map[string]any)
	inner, _ := outer["a"].(map[string]any)
	if inner["api_key"] != "sk-real" {
		t.Errorf("%s.a.api_key = %q, want %q (restored)", key, inner["api_key"], "sk-real")
	}
}

func TestRedactAndPreserve_HonorsSubkey(t *testing.T) {
	// Ad-hoc ShapeKeyedMap descriptor with Subkey="servers" + NoDiscriminator,
	// single secret field "api_key". Mirrors the future mcp shape.
	const key = "__test_subkey_redact"
	descriptor.Register(descriptor.Section{
		Key: key, Label: "Test", GroupID: "runtime",
		Shape: descriptor.ShapeKeyedMap, Subkey: "servers",
		NoDiscriminator: true,
		Fields: []descriptor.FieldSpec{
			{Name: "api_key", Label: "k", Kind: descriptor.FieldSecret},
		},
	})
	t.Cleanup(func() { descriptor.Unregister(key) })

	// Redact: secrets under servers.<inst> must be blanked.
	blob := map[string]any{
		key: map[string]any{
			"servers": map[string]any{
				"foo": map[string]any{"api_key": "real-secret-1"},
				"bar": map[string]any{"api_key": "real-secret-2"},
			},
		},
	}
	RedactSectionSecretsForTest(blob)
	foo := blob[key].(map[string]any)["servers"].(map[string]any)["foo"].(map[string]any)
	bar := blob[key].(map[string]any)["servers"].(map[string]any)["bar"].(map[string]any)
	if foo["api_key"] != "" {
		t.Errorf("after redact: foo.api_key = %v, want blank", foo["api_key"])
	}
	if bar["api_key"] != "" {
		t.Errorf("after redact: bar.api_key = %v, want blank", bar["api_key"])
	}

	// Preserve: foo comes back blanked (user didn't touch it in UI), bar
	// comes back with a new value. foo should be restored from current,
	// bar should keep the new value.
	current := map[string]any{
		key: map[string]any{
			"servers": map[string]any{
				"foo": map[string]any{"api_key": "real-secret-1"},
				"bar": map[string]any{"api_key": "real-secret-2"},
			},
		},
	}
	updated := map[string]any{
		key: map[string]any{
			"servers": map[string]any{
				"foo": map[string]any{"api_key": ""},
				"bar": map[string]any{"api_key": "new-bar-secret"},
			},
		},
	}
	PreserveSectionSecretsForTest(updated, current)
	fooAfter := updated[key].(map[string]any)["servers"].(map[string]any)["foo"].(map[string]any)
	barAfter := updated[key].(map[string]any)["servers"].(map[string]any)["bar"].(map[string]any)
	if fooAfter["api_key"] != "real-secret-1" {
		t.Errorf("after preserve: foo.api_key = %v, want real-secret-1 (restored)", fooAfter["api_key"])
	}
	if barAfter["api_key"] != "new-bar-secret" {
		t.Errorf("after preserve: bar.api_key = %v, want new-bar-secret (kept)", barAfter["api_key"])
	}
}

func TestRedactAndPreserve_HonorsSubkeyForList(t *testing.T) {
	// Ad-hoc ShapeList descriptor with Subkey="jobs" + NoDiscriminator,
	// single secret field "token". Mirrors a future list-of-secrets shape.
	const key = "__test_subkey_list"
	descriptor.Register(descriptor.Section{
		Key: key, Label: "Test", GroupID: "runtime",
		Shape: descriptor.ShapeList, Subkey: "jobs",
		NoDiscriminator: true,
		Fields: []descriptor.FieldSpec{
			{Name: "name", Label: "N", Kind: descriptor.FieldString},
			{Name: "token", Label: "T", Kind: descriptor.FieldSecret},
		},
	})
	t.Cleanup(func() { descriptor.Unregister(key) })

	blob := map[string]any{
		key: map[string]any{
			"jobs": []any{
				map[string]any{"name": "a", "token": "tok-a"},
				map[string]any{"name": "b", "token": "tok-b"},
			},
		},
	}
	RedactSectionSecretsForTest(blob)
	jobs := blob[key].(map[string]any)["jobs"].([]any)
	if jobs[0].(map[string]any)["token"] != "" {
		t.Errorf("redacted jobs[0].token = %v, want blank", jobs[0].(map[string]any)["token"])
	}
	if jobs[1].(map[string]any)["token"] != "" {
		t.Errorf("redacted jobs[1].token = %v, want blank", jobs[1].(map[string]any)["token"])
	}

	current := map[string]any{
		key: map[string]any{
			"jobs": []any{
				map[string]any{"name": "a", "token": "tok-a"},
				map[string]any{"name": "b", "token": "tok-b"},
			},
		},
	}
	updated := map[string]any{
		key: map[string]any{
			"jobs": []any{
				map[string]any{"name": "a", "token": ""},            // user untouched
				map[string]any{"name": "b", "token": "tok-b-fresh"}, // user retyped
			},
		},
	}
	PreserveSectionSecretsForTest(updated, current)
	after := updated[key].(map[string]any)["jobs"].([]any)
	if after[0].(map[string]any)["token"] != "tok-a" {
		t.Errorf("preserved jobs[0].token = %v, want tok-a (restored)", after[0].(map[string]any)["token"])
	}
	if after[1].(map[string]any)["token"] != "tok-b-fresh" {
		t.Errorf("preserved jobs[1].token = %v, want tok-b-fresh (kept)", after[1].(map[string]any)["token"])
	}
}
