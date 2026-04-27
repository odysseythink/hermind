package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/config"
	_ "github.com/odysseythink/hermind/config/descriptor"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
)

// pingableProvider extends fakeProvider with a working Complete so the
// /test endpoints have something to ping. The default fakeProvider's
// Complete returns "not implemented" which is fine for /models tests but
// hides the success path we need here.
type pingableProvider struct {
	models     []string
	listErr    error
	completeOK bool
	completeEr error
}

func (p *pingableProvider) Name() string { return "pingable" }
func (p *pingableProvider) Complete(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	if p.completeEr != nil {
		return nil, p.completeEr
	}
	if !p.completeOK {
		return nil, errors.New("not implemented")
	}
	return &provider.Response{
		Message:      message.Message{Role: message.RoleAssistant, Content: message.TextContent("pong")},
		FinishReason: "stop",
	}, nil
}
func (p *pingableProvider) Stream(_ context.Context, _ *provider.Request) (provider.Stream, error) {
	return nil, errors.New("not implemented")
}
func (p *pingableProvider) ModelInfo(_ string) *provider.ModelInfo { return nil }
func (p *pingableProvider) EstimateTokens(_ string, text string) (int, error) {
	return len(text) / 4, nil
}
func (p *pingableProvider) Available() bool { return true }
func (p *pingableProvider) ListModels(_ context.Context) ([]string, error) {
	return p.models, p.listErr
}

func TestAuxiliaryModels_HappyPath_AuxConfigured(t *testing.T) {
	factory.SetConstructorForTest("__aux_happy", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &pingableProvider{models: []string{"a1", "a2"}}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__aux_happy") })

	cfg := &config.Config{
		Auxiliary: config.AuxiliaryConfig{Provider: "__aux_happy", APIKey: "k"},
	}
	srv, err := api.NewServer(&api.ServerOpts{Config: cfg})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/auxiliary/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body struct {
		Models []string `json:"models"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(body.Models) != 2 || body.Models[0] != "a1" {
		t.Errorf("models = %v, want [a1 a2]", body.Models)
	}
}

// When auxiliary is blank the endpoint must transparently fall back to the
// main provider config — this matches engine_deps.go:123 so the UI shows the
// right model list whether or not the user filled in the auxiliary block.
func TestAuxiliaryModels_FallsBackToMainProvider(t *testing.T) {
	factory.SetConstructorForTest("__aux_main_fb", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &pingableProvider{models: []string{"main-1"}}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__aux_main_fb") })

	cfg := &config.Config{
		Model: "__aux_main_fb/x",
		Providers: map[string]config.ProviderConfig{
			"__aux_main_fb": {Provider: "__aux_main_fb", APIKey: "k"},
		},
	}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/auxiliary/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestAuxiliaryModels_NoAuxAndNoMain(t *testing.T) {
	cfg := &config.Config{}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/auxiliary/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAuxiliaryModels_UpstreamError(t *testing.T) {
	factory.SetConstructorForTest("__aux_upstream_err", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &pingableProvider{listErr: errors.New("auth failed")}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__aux_upstream_err") })

	cfg := &config.Config{
		Auxiliary: config.AuxiliaryConfig{Provider: "__aux_upstream_err", APIKey: "k"},
	}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/auxiliary/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestAuxiliaryTest_HappyPath(t *testing.T) {
	factory.SetConstructorForTest("__aux_test_ok", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &pingableProvider{completeOK: true}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__aux_test_ok") })

	cfg := &config.Config{
		Auxiliary: config.AuxiliaryConfig{Provider: "__aux_test_ok", APIKey: "k", Model: "test-m"},
	}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/auxiliary/test", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body struct {
		OK        bool  `json:"ok"`
		LatencyMS int64 `json:"latency_ms"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !body.OK {
		t.Errorf("ok = false, want true")
	}
	if body.LatencyMS < 0 {
		t.Errorf("latency_ms = %d, want >= 0", body.LatencyMS)
	}
}

func TestAuxiliaryTest_UpstreamError(t *testing.T) {
	factory.SetConstructorForTest("__aux_test_err", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &pingableProvider{completeEr: errors.New("model not available")}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__aux_test_err") })

	cfg := &config.Config{
		Auxiliary: config.AuxiliaryConfig{Provider: "__aux_test_err", APIKey: "k"},
	}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/auxiliary/test", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestProvidersTest_HappyPath(t *testing.T) {
	factory.SetConstructorForTest("__prov_test_ok", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &pingableProvider{completeOK: true}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__prov_test_ok") })

	cfg := &config.Config{Providers: map[string]config.ProviderConfig{
		"p1": {Provider: "__prov_test_ok", APIKey: "k", Model: "m"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/providers/p1/test", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestProvidersTest_UnknownName(t *testing.T) {
	cfg := &config.Config{Providers: map[string]config.ProviderConfig{}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/providers/missing/test", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestProvidersTest_UpstreamError(t *testing.T) {
	factory.SetConstructorForTest("__prov_test_err", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &pingableProvider{completeEr: errors.New("network down")}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__prov_test_err") })

	cfg := &config.Config{Providers: map[string]config.ProviderConfig{
		"p1": {Provider: "__prov_test_err", APIKey: "k"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/providers/p1/test", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}
