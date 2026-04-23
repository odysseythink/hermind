package api_test

import (
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

func TestFallbackProvidersModels_HappyPath(t *testing.T) {
	factory.SetConstructorForTest("__fake_fb_happy", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &fakeProvider{models: []string{"fb1", "fb2"}}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__fake_fb_happy") })

	cfg := &config.Config{FallbackProviders: []config.ProviderConfig{
		{Provider: "__fake_fb_happy", APIKey: "k"},
	}}
	srv, err := api.NewServer(&api.ServerOpts{Config: cfg})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/fallback_providers/0/models", nil)
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
	if len(body.Models) != 2 || body.Models[0] != "fb1" || body.Models[1] != "fb2" {
		t.Errorf("models = %v, want [fb1 fb2]", body.Models)
	}
}

func TestFallbackProvidersModels_IndexOutOfRange(t *testing.T) {
	cfg := &config.Config{FallbackProviders: []config.ProviderConfig{
		{Provider: "x", APIKey: "k"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/fallback_providers/5/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (index out of range)", w.Code)
	}
}

func TestFallbackProvidersModels_NegativeIndex(t *testing.T) {
	cfg := &config.Config{FallbackProviders: []config.ProviderConfig{
		{Provider: "x", APIKey: "k"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/fallback_providers/-1/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (invalid index)", w.Code)
	}
}

func TestFallbackProvidersModels_NonIntegerIndex(t *testing.T) {
	cfg := &config.Config{FallbackProviders: []config.ProviderConfig{
		{Provider: "x", APIKey: "k"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/fallback_providers/abc/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (non-integer index)", w.Code)
	}
}

func TestFallbackProvidersModels_FactoryError(t *testing.T) {
	factory.SetConstructorForTest("__fake_fb_factory_err", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return nil, errors.New("bad config")
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__fake_fb_factory_err") })

	cfg := &config.Config{FallbackProviders: []config.ProviderConfig{
		{Provider: "__fake_fb_factory_err"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/fallback_providers/0/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (factory rejection)", w.Code)
	}
}

func TestFallbackProvidersModels_NotListModelsCapable(t *testing.T) {
	factory.SetConstructorForTest("__fake_fb_no_lister", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &fakeProviderNoLister{}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__fake_fb_no_lister") })

	cfg := &config.Config{FallbackProviders: []config.ProviderConfig{
		{Provider: "__fake_fb_no_lister"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/fallback_providers/0/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", w.Code)
	}
}

func TestFallbackProvidersModels_UpstreamError(t *testing.T) {
	factory.SetConstructorForTest("__fake_fb_upstream_err", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &fakeProvider{err: errors.New("auth failed")}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__fake_fb_upstream_err") })

	cfg := &config.Config{FallbackProviders: []config.ProviderConfig{
		{Provider: "__fake_fb_upstream_err"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg})
	req := httptest.NewRequest("POST", "/api/fallback_providers/0/models", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}
