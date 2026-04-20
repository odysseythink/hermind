package stdio

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// stubProvider implements provider.Provider with a canned response.
type stubProvider struct {
	reply    string
	err      error
	received *provider.Request
}

func (s *stubProvider) Name() string                             { return "stub" }
func (s *stubProvider) Available() bool                          { return true }
func (s *stubProvider) ModelInfo(string) *provider.ModelInfo     { return &provider.ModelInfo{ContextLength: 1000, MaxOutputTokens: 100} }
func (s *stubProvider) EstimateTokens(_, t string) (int, error)  { return len(t) / 4, nil }
func (s *stubProvider) Stream(context.Context, *provider.Request) (provider.Stream, error) {
	return nil, errors.New("stream not supported in stub")
}
func (s *stubProvider) Complete(_ context.Context, req *provider.Request) (*provider.Response, error) {
	s.received = req
	if s.err != nil {
		return nil, s.err
	}
	return &provider.Response{
		Message:      message.Message{Role: message.RoleAssistant, Content: message.TextContent(s.reply)},
		FinishReason: "end_turn",
	}, nil
}

func newTestHandlers(t *testing.T) *Handlers {
	t.Helper()
	store := openTestStore(t)
	return &Handlers{
		Sessions: NewSessionManager(store),
		Factory: func(model string) (provider.Provider, error) {
			return &stubProvider{reply: "ok"}, nil
		},
		AgentCfg: config.AgentConfig{MaxTurns: 3},
	}
}

func TestHandleInitialize(t *testing.T) {
	h := newTestHandlers(t)
	raw, err := h.handleInitialize(context.Background(), json.RawMessage(`{"clientInfo":{"name":"zed"}}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	_ = json.Unmarshal(raw, &resp)
	if resp["protocolVersion"] == nil {
		t.Errorf("missing protocolVersion: %v", resp)
	}
	if resp["agentInfo"] == nil {
		t.Errorf("missing agentInfo: %v", resp)
	}
	if resp["agentCapabilities"] == nil {
		t.Errorf("missing agentCapabilities: %v", resp)
	}
}

func TestHandleNewSession_ReturnsID(t *testing.T) {
	h := newTestHandlers(t)
	raw, err := h.handleNewSession(context.Background(), json.RawMessage(`{"cwd":"/tmp"}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]string
	_ = json.Unmarshal(raw, &resp)
	if resp["sessionId"] == "" {
		t.Errorf("no sessionId: %v", resp)
	}
}

func TestHandleNewSession_RequiresCwd(t *testing.T) {
	h := newTestHandlers(t)
	if _, err := h.handleNewSession(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Error("expected error when cwd is missing")
	}
}

func TestHandleLoadSession_UpdatesCwd(t *testing.T) {
	h := newTestHandlers(t)
	s, _ := h.Sessions.Create(context.Background(), "/tmp", "stub/model")
	params, _ := json.Marshal(map[string]any{"sessionId": s.ID, "cwd": "/work"})
	if _, err := h.handleLoadSession(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	got, _ := h.Sessions.Get(context.Background(), s.ID)
	if got.Cwd != "/work" {
		t.Errorf("cwd = %q", got.Cwd)
	}
}

func TestHandlePrompt_EchoesProvider(t *testing.T) {
	h := newTestHandlers(t)
	s, _ := h.Sessions.Create(context.Background(), "/tmp", "stub/model")

	params, _ := json.Marshal(map[string]any{
		"sessionId": s.ID,
		"prompt":    []any{map[string]any{"type": "text", "text": "hi"}},
	})
	raw, err := h.handlePrompt(context.Background(), params)
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	var resp map[string]any
	_ = json.Unmarshal(raw, &resp)
	if resp["stopReason"] != "end_turn" {
		t.Errorf("stopReason = %v", resp["stopReason"])
	}

	msgs, _ := h.Sessions.History(context.Background(), s.ID)
	if len(msgs) != 2 {
		t.Errorf("history len = %d, want 2", len(msgs))
	}
}

func TestHandlePrompt_EmptyBlocksReturnsRefusal(t *testing.T) {
	h := newTestHandlers(t)
	s, _ := h.Sessions.Create(context.Background(), "/tmp", "stub/model")
	params, _ := json.Marshal(map[string]any{
		"sessionId": s.ID,
		"prompt":    []any{},
	})
	raw, err := h.handlePrompt(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !containsJSON(raw, `"stopReason":"refusal"`) {
		t.Errorf("want refusal, got %s", string(raw))
	}
}

func TestHandleCancel_InterruptsActivePrompt(t *testing.T) {
	h := newTestHandlers(t)
	s, _ := h.Sessions.Create(context.Background(), "/tmp", "stub/model")

	ctx, cancel := context.WithCancel(context.Background())
	h.Sessions.SetCancel(s.ID, cancel)

	params, _ := json.Marshal(map[string]any{"sessionId": s.ID})
	if _, err := h.handleCancel(ctx, params); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
		// OK
	default:
		t.Error("expected ctx done after cancel")
	}
}

// containsJSON is a tiny helper — json.RawMessage is []byte under the
// hood, but substring matching keeps the assertion readable.
func containsJSON(raw json.RawMessage, want string) bool {
	return string(raw) != "" && string(raw[:]) == want || indexOf(string(raw), want) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
