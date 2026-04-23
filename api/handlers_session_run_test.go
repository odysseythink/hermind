package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/odysseythink/hermind/api/sessionrun"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
)

// slowProvider blocks Stream until block is closed. Used to hold a
// session in the registry long enough to test the busy path.
type slowProvider struct{ block chan struct{} }

func (p *slowProvider) Name() string { return "slow" }
func (p *slowProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	return nil, errors.New("complete not used")
}
func (p *slowProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.block:
		return &slowStream{}, nil
	}
}
func (p *slowProvider) ModelInfo(string) *provider.ModelInfo       { return nil }
func (p *slowProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (p *slowProvider) Available() bool                            { return true }

type slowStream struct{}

func (s *slowStream) Recv() (*provider.StreamEvent, error) {
	return &provider.StreamEvent{
		Type: provider.EventDone,
		Response: &provider.Response{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: message.TextContent("ok"),
			},
			FinishReason: "end_turn",
		},
	}, nil
}
func (s *slowStream) Close() error { return nil }

func buildTestServerWithDeps(t *testing.T, deps sessionrun.Deps) *Server {
	t.Helper()
	srv, err := NewServer(&ServerOpts{
		Config:  &config.Config{},
		Storage: nil,

		Streams: NewMemoryStreamHub(),
		Deps:    deps,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

func TestMessagesPost_Accepted(t *testing.T) {
	block := make(chan struct{})
	deps := sessionrun.Deps{
		Provider: &slowProvider{block: block},
		ToolReg:  tool.NewRegistry(),
		AgentCfg: config.AgentConfig{MaxTurns: 5},
	}
	srv := buildTestServerWithDeps(t, deps)
	defer close(block)

	body, _ := json.Marshal(MessageSubmitRequest{Text: "hi"})
	req := httptest.NewRequest("POST", "/api/sessions/s1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != 202 {
		t.Fatalf("code = %d, want 202; body=%s", w.Code, w.Body.String())
	}
	var resp MessageSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.SessionID != "s1" || resp.Status != "accepted" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestMessagesPost_MissingText(t *testing.T) {
	srv := buildTestServerWithDeps(t, sessionrun.Deps{
		Provider: &slowProvider{block: make(chan struct{})},
		ToolReg:  tool.NewRegistry(),
		AgentCfg: config.AgentConfig{MaxTurns: 5},
	})
	req := httptest.NewRequest("POST", "/api/sessions/s1/messages", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("code = %d, want 400", w.Code)
	}
}

func TestMessagesPost_InvalidJSON(t *testing.T) {
	srv := buildTestServerWithDeps(t, sessionrun.Deps{
		Provider: &slowProvider{block: make(chan struct{})},
		ToolReg:  tool.NewRegistry(),
		AgentCfg: config.AgentConfig{MaxTurns: 5},
	})
	req := httptest.NewRequest("POST", "/api/sessions/s1/messages", bytes.NewReader([]byte(`{`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("code = %d, want 400", w.Code)
	}
}

func TestMessagesPost_NoProvider(t *testing.T) {
	srv := buildTestServerWithDeps(t, sessionrun.Deps{}) // Provider == nil
	req := httptest.NewRequest("POST", "/api/sessions/s1/messages",
		bytes.NewReader([]byte(`{"text":"hi"}`)))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}

func TestMessagesPost_Busy(t *testing.T) {
	block := make(chan struct{})
	deps := sessionrun.Deps{
		Provider: &slowProvider{block: block},
		ToolReg:  tool.NewRegistry(),
		AgentCfg: config.AgentConfig{MaxTurns: 5},
	}
	srv := buildTestServerWithDeps(t, deps)

	reqBody := []byte(`{"text":"hi"}`)
	r1 := httptest.NewRequest("POST", "/api/sessions/s1/messages", bytes.NewReader(reqBody))
	r1.Header.Set("Authorization", "Bearer t")
	w1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w1, r1)
	if w1.Code != 202 {
		t.Fatalf("first code = %d, want 202", w1.Code)
	}
	time.Sleep(20 * time.Millisecond)

	r2 := httptest.NewRequest("POST", "/api/sessions/s1/messages", bytes.NewReader(reqBody))
	r2.Header.Set("Authorization", "Bearer t")
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, r2)
	if w2.Code != 409 {
		t.Fatalf("second code = %d, want 409", w2.Code)
	}
	close(block)
	// Give the first request's goroutine a beat to finish cleanly.
	time.Sleep(20 * time.Millisecond)
}

func TestCancelPost_Running(t *testing.T) {
	block := make(chan struct{})
	deps := sessionrun.Deps{
		Provider: &slowProvider{block: block},
		ToolReg:  tool.NewRegistry(),
		AgentCfg: config.AgentConfig{MaxTurns: 5},
	}
	srv := buildTestServerWithDeps(t, deps)

	r1 := httptest.NewRequest("POST", "/api/sessions/s1/messages",
		bytes.NewReader([]byte(`{"text":"hi"}`)))
	r1.Header.Set("Authorization", "Bearer t")
	srv.Router().ServeHTTP(httptest.NewRecorder(), r1)
	time.Sleep(20 * time.Millisecond)

	r2 := httptest.NewRequest("POST", "/api/sessions/s1/cancel", nil)
	r2.Header.Set("Authorization", "Bearer t")
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, r2)
	if w2.Code != 204 {
		t.Fatalf("code = %d, want 204", w2.Code)
	}
	close(block)
	time.Sleep(20 * time.Millisecond)
}

func TestCancelPost_NotRunning(t *testing.T) {
	srv := buildTestServerWithDeps(t, sessionrun.Deps{})
	req := httptest.NewRequest("POST", "/api/sessions/nobody/cancel", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("code = %d, want 404", w.Code)
	}
}

func TestCancelPost_Idempotent(t *testing.T) {
	block := make(chan struct{})
	deps := sessionrun.Deps{
		Provider: &slowProvider{block: block},
		ToolReg:  tool.NewRegistry(),
		AgentCfg: config.AgentConfig{MaxTurns: 5},
	}
	srv := buildTestServerWithDeps(t, deps)

	r1 := httptest.NewRequest("POST", "/api/sessions/s1/messages",
		bytes.NewReader([]byte(`{"text":"hi"}`)))
	r1.Header.Set("Authorization", "Bearer t")
	srv.Router().ServeHTTP(httptest.NewRecorder(), r1)
	time.Sleep(20 * time.Millisecond)

	c1 := httptest.NewRequest("POST", "/api/sessions/s1/cancel", nil)
	c1.Header.Set("Authorization", "Bearer t")
	w1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w1, c1)
	if w1.Code != 204 {
		t.Fatalf("first cancel = %d, want 204", w1.Code)
	}
	close(block)
	time.Sleep(20 * time.Millisecond)

	c2 := httptest.NewRequest("POST", "/api/sessions/s1/cancel", nil)
	c2.Header.Set("Authorization", "Bearer t")
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, c2)
	if w2.Code != 404 {
		t.Fatalf("second cancel = %d, want 404", w2.Code)
	}
}

// silence unused import warning for io when debugging
var _ = io.EOF
