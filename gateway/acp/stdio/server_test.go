package stdio

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestServer_EndToEnd_NewSessionAndPrompt(t *testing.T) {
	h := newTestHandlers(t)
	srv := NewServer(h)

	// Phase 1: drive initialize + session/new.
	phase1 := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"cwd":"/tmp","model":"stub/model"}}` + "\n")
	var out bytes.Buffer
	if err := srv.RunOnce(context.Background(), phase1, &out, 2); err != nil {
		t.Fatalf("phase1: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %q", len(lines), out.String())
	}
	var resp2 struct {
		Result newSessionResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &resp2); err != nil {
		t.Fatalf("unmarshal resp2: %v", err)
	}
	sessionID := resp2.Result.SessionID
	if sessionID == "" {
		t.Fatalf("no session id in %q", lines[1])
	}

	// Phase 2: drive prompt.
	out.Reset()
	phase2 := `{"jsonrpc":"2.0","id":3,"method":"session/prompt","params":{"sessionId":"` + sessionID + `","prompt":[{"type":"text","text":"hi"}]}}` + "\n"
	if err := srv.RunOnce(context.Background(), strings.NewReader(phase2), &out, 1); err != nil {
		t.Fatalf("phase2: %v", err)
	}
	if !strings.Contains(out.String(), `"stopReason":"end_turn"`) {
		t.Errorf("unexpected response: %s", out.String())
	}
}

func TestServer_UnknownMethodReturnsMethodNotFound(t *testing.T) {
	h := newTestHandlers(t)
	srv := NewServer(h)
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"does_not_exist"}` + "\n")
	var out bytes.Buffer
	if err := srv.RunOnce(context.Background(), in, &out, 1); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), `"code":-32601`) {
		t.Errorf("expected -32601, got %s", out.String())
	}
}

func TestServer_ParseErrorOnBadFrame(t *testing.T) {
	h := newTestHandlers(t)
	srv := NewServer(h)
	in := strings.NewReader("not json\n")
	var out bytes.Buffer
	_ = srv.RunOnce(context.Background(), in, &out, 1)
	if !strings.Contains(out.String(), `"code":-32700`) {
		t.Errorf("expected parse error -32700, got %s", out.String())
	}
}

func TestServer_NotificationEmitsNoResponse(t *testing.T) {
	h := newTestHandlers(t)
	// Register a session so cancel has something to target.
	s, _ := h.Sessions.Create(context.Background(), "/tmp", "stub/model")
	srv := NewServer(h)
	in := strings.NewReader(`{"jsonrpc":"2.0","method":"session/cancel","params":{"sessionId":"` + s.ID + `"}}` + "\n")
	var out bytes.Buffer
	if err := srv.RunOnce(context.Background(), in, &out, 1); err != nil {
		t.Fatalf("run: %v", err)
	}
	if strings.TrimSpace(out.String()) != "" {
		t.Errorf("notification must produce no response, got %q", out.String())
	}
}

func TestServer_AcceptsSnakeCaseAliases(t *testing.T) {
	h := newTestHandlers(t)
	srv := NewServer(h)
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"new_session","params":{"cwd":"/tmp"}}` + "\n")
	var out bytes.Buffer
	if err := srv.RunOnce(context.Background(), in, &out, 1); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), `"sessionId"`) {
		t.Errorf("expected sessionId in response, got %s", out.String())
	}
}
