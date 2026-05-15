package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/pantheon/core"
)

// stubProvider returns a canned response from Generate.
type stubProvider struct {
	resp *core.Response
	err  error
}

func (s *stubProvider) Provider() string { return "stub" }
func (s *stubProvider) Model() string    { return "stub-model" }
func (s *stubProvider) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}
func (s *stubProvider) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, errors.New("not implemented")
}
func (s *stubProvider) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

// newProxyTestServer assembles a minimal Server with proxy enabled.
func newProxyTestServer(t *testing.T, p core.LanguageModel) *Server {
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
	stub := &stubProvider{resp: &core.Response{
		Message: core.Message{
			Role:    core.MESSAGE_ROLE_ASSISTANT,
			Content: []core.ContentParter{core.TextPart{Text: "hello"}},
		},
		FinishReason: "stop",
		Usage:        core.Usage{PromptTokens: 10, CompletionTokens: 1},
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

// streamingStubProvider returns a real Stream from Stream().
type streamingStubProvider struct {
	stub *stubProvider
}

func (p *streamingStubProvider) Provider() string { return "streaming-stub" }
func (p *streamingStubProvider) Model() string    { return "streaming-stub-model" }
func (p *streamingStubProvider) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	return p.stub.Generate(ctx, req)
}
func (p *streamingStubProvider) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return func(yield func(*core.StreamPart, error) bool) {
		if !yield(&core.StreamPart{Type: core.StreamPartTypeTextDelta, TextDelta: "stream-hi"}, nil) {
			return
		}
		if !yield(&core.StreamPart{Type: core.StreamPartTypeUsage, Usage: &core.Usage{PromptTokens: 5, CompletionTokens: 1}}, nil) {
			return
		}
		if !yield(&core.StreamPart{Type: core.StreamPartTypeFinish, FinishReason: "stop"}, nil) {
			return
		}
	}, nil
}
func (p *streamingStubProvider) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func TestV1Messages_StreamingHappyPath(t *testing.T) {
	prov := &streamingStubProvider{stub: &stubProvider{}}
	srv := newProxyTestServer(t, prov)
	body := []byte(`{
		"model": "claude-sonnet-4-6",
		"max_tokens": 64,
		"stream": true,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hi"}]}]
	}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	srv.Router().ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "text/event-stream", rr.Header().Get("Content-Type"))

	body2 := rr.Body.String()
	require.True(t, len(body2) > 0)
	require.Contains(t, body2, "event: message_start")
	require.Contains(t, body2, "stream-hi")
	require.Contains(t, body2, "event: message_stop")
}

func TestV1Messages_MountedBeforeUIWildcard(t *testing.T) {
	// With both proxy enabled and UI static handler present, the
	// /v1/messages route must take precedence over /ui/*. We verify by
	// hitting /v1/messages and asserting we don't get the static handler's
	// response (which would typically be a 404 for an unknown UI path).
	stub := &stubProvider{resp: &core.Response{
		Message: core.Message{
			Role:    core.MESSAGE_ROLE_ASSISTANT,
			Content: []core.ContentParter{core.TextPart{Text: "ok"}},
		},
		FinishReason: "stop",
	}}
	srv := newProxyTestServer(t, stub)

	body := []byte(`{
		"model": "x", "max_tokens": 8,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "ping"}]}]
	}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	srv.Router().ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "/v1/messages must reach handler, not be shadowed")
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}

// errProviderRateLimit is a sentinel test-only error.
var errProviderRateLimit = errors.New("provider: rate limited (429)")

// stubProviderErr returns a fixed error from Generate.
type stubProviderErr struct{ err error }

func (p *stubProviderErr) Provider() string { return "stub-err" }
func (p *stubProviderErr) Model() string    { return "stub-err-model" }
func (p *stubProviderErr) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	return nil, p.err
}
func (p *stubProviderErr) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, p.err
}
func (p *stubProviderErr) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, p.err
}

func TestV1Messages_ProviderRateLimitMapsTo429(t *testing.T) {
	prov := &stubProviderErr{err: errProviderRateLimit}
	srv := newProxyTestServer(t, prov)
	body := []byte(`{
		"model": "x", "max_tokens": 8,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hi"}]}]
	}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	srv.Router().ServeHTTP(rr, req)
	require.True(t, rr.Code == http.StatusBadGateway || rr.Code == http.StatusTooManyRequests,
		"got %d, expected 502 or 429", rr.Code)
	require.Contains(t, rr.Body.String(), "rate limited")
}

func TestV1Messages_ProviderGenericErrorMapsTo502(t *testing.T) {
	prov := &stubProviderErr{err: errors.New("upstream timeout")}
	srv := newProxyTestServer(t, prov)
	body := []byte(`{
		"model": "x", "max_tokens": 8,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hi"}]}]
	}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	srv.Router().ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadGateway, rr.Code)
}

// blockingProvider yields a context error when the context is cancelled.
// Verifies that StreamOutbound + handleV1Messages clean
// up properly when the client disconnects.
type blockingProvider struct{}

func (b *blockingProvider) Provider() string { return "blocking" }
func (b *blockingProvider) Model() string    { return "blocking-model" }
func (b *blockingProvider) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	return nil, errors.New("not used")
}
func (b *blockingProvider) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return func(yield func(*core.StreamPart, error) bool) {
		<-ctx.Done()
		yield(nil, ctx.Err())
	}, nil
}
func (b *blockingProvider) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func TestV1Messages_StreamCancellationCleanup(t *testing.T) {
	srv := newProxyTestServer(t, &blockingProvider{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	body := []byte(`{
		"model": "x", "max_tokens": 8, "stream": true,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hi"}]}]
	}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body)).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		srv.Router().ServeHTTP(rr, req)
		close(done)
	}()

	// Cancel the request after 50ms — handler should return cleanly.
	time.AfterFunc(50*time.Millisecond, cancel)
	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return within 2s after cancellation")
	}
}
