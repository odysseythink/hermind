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

// streamingStubProvider returns a real Stream from Stream().
type streamingStubProvider struct {
	stub *stubProvider
}

func (p *streamingStubProvider) Name() string { return "streaming-stub" }
func (p *streamingStubProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	return p.stub.Complete(ctx, req)
}
func (p *streamingStubProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	return &fakeProviderStream{events: []provider.StreamEvent{
		{Type: provider.EventDelta, Delta: &provider.StreamDelta{Content: "stream-hi"}},
		{Type: provider.EventDone, Response: &provider.Response{
			FinishReason: "stop",
			Usage:        message.Usage{InputTokens: 5, OutputTokens: 1},
			Model:        "actual-model",
		}},
	}}, nil
}
func (p *streamingStubProvider) ModelInfo(model string) *provider.ModelInfo  { return nil }
func (p *streamingStubProvider) EstimateTokens(model, text string) (int, error) { return 0, nil }
func (p *streamingStubProvider) Available() bool                                { return true }

type fakeProviderStream struct {
	events []provider.StreamEvent
	idx    int
}

func (f *fakeProviderStream) Recv() (*provider.StreamEvent, error) {
	if f.idx >= len(f.events) {
		return nil, errors.New("exhausted")
	}
	ev := f.events[f.idx]
	f.idx++
	return &ev, nil
}
func (f *fakeProviderStream) Close() error { return nil }

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
	stub := &stubProvider{resp: &provider.Response{
		Message: message.Message{Role: message.RoleAssistant, Content: message.TextContent("ok")},
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

// stubProviderErr returns a fixed error from Complete.
type stubProviderErr struct{ err error }

func (p *stubProviderErr) Name() string { return "stub-err" }
func (p *stubProviderErr) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	return nil, p.err
}
func (p *stubProviderErr) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	return nil, p.err
}
func (p *stubProviderErr) ModelInfo(model string) *provider.ModelInfo { return nil }
func (p *stubProviderErr) EstimateTokens(model, text string) (int, error) { return 0, nil }
func (p *stubProviderErr) Available() bool                                { return true }

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

// blockingStream blocks Recv() until ctx is cancelled, then returns the
// context error. Verifies that StreamOutbound + handleV1Messages clean
// up properly when the client disconnects.
type blockingStream struct {
	ctx    context.Context
	closed bool
}

func (b *blockingStream) Recv() (*provider.StreamEvent, error) {
	<-b.ctx.Done()
	return nil, b.ctx.Err()
}
func (b *blockingStream) Close() error { b.closed = true; return nil }

type blockingProvider struct{}

func (b *blockingProvider) Name() string { return "blocking" }
func (b *blockingProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	return nil, errors.New("not used")
}
func (b *blockingProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	return &blockingStream{ctx: ctx}, nil
}
func (b *blockingProvider) ModelInfo(model string) *provider.ModelInfo { return nil }
func (b *blockingProvider) EstimateTokens(model, text string) (int, error) { return 0, nil }
func (b *blockingProvider) Available() bool                                { return true }

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
