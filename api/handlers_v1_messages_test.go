package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// stubProvider returns a canned response from Complete.
type stubProvider struct {
	resp *provider.Response
	err  error
}

func (s *stubProvider) Name() string { return "stub" }
func (s *stubProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}
func (s *stubProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	return nil, errors.New("not implemented")
}
func (s *stubProvider) ModelInfo(model string) *provider.ModelInfo  { return nil }
func (s *stubProvider) EstimateTokens(model, text string) (int, error) { return 0, nil }
func (s *stubProvider) Available() bool                                 { return true }

// newProxyTestServer assembles a minimal Server with proxy enabled.
func newProxyTestServer(t *testing.T, p provider.Provider) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Proxy.Enabled = true
	srv, err := NewServer(&ServerOpts{
		Config: cfg,
		Deps:   EngineDeps{Provider: p},
	})
	require.NoError(t, err)
	return srv
}

func TestV1Messages_NonStreamingHappyPath(t *testing.T) {
	stub := &stubProvider{resp: &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent("hello"),
		},
		FinishReason: "stop",
		Usage:        message.Usage{InputTokens: 10, OutputTokens: 1},
		Model:        "actual-model",
	}}
	srv := newProxyTestServer(t, stub)

	body := []byte(`{
		"model": "claude-sonnet-4-6",
		"max_tokens": 64,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hi"}]}]
	}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	srv.Router().ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "body=%s", rr.Body.String())

	var got map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, "claude-sonnet-4-6", got["model"])
	require.Equal(t, "actual-model", rr.Header().Get("x-hermind-actual-model"))
}

func TestV1Messages_DisabledReturnsNotFound(t *testing.T) {
	stub := &stubProvider{}
	cfg := &config.Config{} // proxy disabled by default
	srv, err := NewServer(&ServerOpts{
		Config: cfg,
		Deps:   EngineDeps{Provider: stub},
	})
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader([]byte(`{}`)))
	srv.Router().ServeHTTP(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestV1Messages_ProviderNotConfigured(t *testing.T) {
	cfg := &config.Config{}
	cfg.Proxy.Enabled = true
	srv, err := NewServer(&ServerOpts{
		Config: cfg,
		Deps:   EngineDeps{Provider: nil},
	})
	require.NoError(t, err)

	body := []byte(`{
		"model": "x", "max_tokens": 64,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hi"}]}]
	}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	srv.Router().ServeHTTP(rr, req)
	require.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestV1Messages_BadRequestBody(t *testing.T) {
	stub := &stubProvider{}
	srv := newProxyTestServer(t, stub)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader([]byte("{bad json")))
	srv.Router().ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, "error", got["type"])
	errObj, _ := got["error"].(map[string]any)
	require.Equal(t, "invalid_request_error", errObj["type"])
}
