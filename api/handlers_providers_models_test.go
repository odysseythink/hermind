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
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
)

// fakeProvider implements provider.Provider with optional ModelLister behavior.
// Injected into factory.primary via the SetConstructorForTest hook below.
type fakeProvider struct {
	models []string
	err    error
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Complete(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeProvider) Stream(_ context.Context, _ *provider.Request) (provider.Stream, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeProvider) ModelInfo(_ string) *provider.ModelInfo       { return nil }
func (f *fakeProvider) EstimateTokens(_ string, text string) (int, error) {
	return len(text) / 4, nil
}
func (f *fakeProvider) Available() bool { return true }
func (f *fakeProvider) ListModels(_ context.Context) ([]string, error) {
	return f.models, f.err
}

// fakeProviderNoLister returns a provider that does NOT implement ModelLister.
type fakeProviderNoLister struct{}

func (f *fakeProviderNoLister) Name() string { return "fake-no-lister" }
func (f *fakeProviderNoLister) Complete(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeProviderNoLister) Stream(_ context.Context, _ *provider.Request) (provider.Stream, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeProviderNoLister) ModelInfo(_ string) *provider.ModelInfo       { return nil }
func (f *fakeProviderNoLister) EstimateTokens(_ string, text string) (int, error) {
	return len(text) / 4, nil
}
func (f *fakeProviderNoLister) Available() bool { return true }

func TestProvidersModels_HappyPath(t *testing.T) {
	factory.SetConstructorForTest("__fake_happy", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &fakeProvider{models: []string{"m1", "m2"}}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__fake_happy") })

	cfg := &config.Config{Providers: map[string]config.ProviderConfig{
		"p1": {Provider: "__fake_happy", APIKey: "k"},
	}}
	srv, err := api.NewServer(&api.ServerOpts{Config: cfg})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/providers/p1/models", nil)
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
	if len(body.Models) != 2 || body.Models[0] != "m1" || body.Models[1] != "m2" {
		t.Errorf("models = %v, want [m1 m2]", body.Models)
	}
}

func TestProvidersModels_UnknownName(t *testing.T) {
	cfg := &config.Config{Providers: map[string]config.ProviderConfig{}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/providers/does-not-exist/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestProvidersModels_FactoryError(t *testing.T) {
	factory.SetConstructorForTest("__fake_factory_err", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return nil, errors.New("bad config")
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__fake_factory_err") })

	cfg := &config.Config{Providers: map[string]config.ProviderConfig{
		"p1": {Provider: "__fake_factory_err"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/providers/p1/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestProvidersModels_NotListModelsCapable(t *testing.T) {
	factory.SetConstructorForTest("__fake_no_lister", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &fakeProviderNoLister{}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__fake_no_lister") })

	cfg := &config.Config{Providers: map[string]config.ProviderConfig{
		"p1": {Provider: "__fake_no_lister"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/providers/p1/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", w.Code)
	}
}

func TestProvidersModels_UpstreamError(t *testing.T) {
	factory.SetConstructorForTest("__fake_upstream_err", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &fakeProvider{err: errors.New("auth failed")}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__fake_upstream_err") })

	cfg := &config.Config{Providers: map[string]config.ProviderConfig{
		"p1": {Provider: "__fake_upstream_err"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/providers/p1/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}
