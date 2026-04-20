package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/odysseythink/hermind/gateway"
	"github.com/odysseythink/hermind/tool"
)

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestPermissions(t *testing.T) {
	p := NewPermissions()
	p.Grant("t1", "messages:send", "tools:*")
	if !p.Allow("t1", "messages:send") {
		t.Error("expected allow")
	}
	if !p.Allow("t1", "tools:execute") {
		t.Error("wildcard should allow")
	}
	if p.Allow("t1", "admin:wipe") {
		t.Error("unrelated action should be denied")
	}
	if p.Allow("t2", "anything") {
		t.Error("unknown token should be denied")
	}
	p.Revoke("t1")
	if p.Allow("t1", "messages:send") {
		t.Error("revoke failed")
	}
}

func TestEventBusPubSub(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe()
	bus.Publish(Event{Type: "hello", Data: "world"})
	select {
	case ev := <-ch:
		if ev.Type != "hello" || ev.Data != "world" {
			t.Errorf("bad event: %+v", ev)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}
	bus.Unsubscribe(ch)
	if _, open := <-ch; open {
		t.Error("channel should be closed after unsubscribe")
	}
}

func TestServerFullFlow(t *testing.T) {
	addr := freePort(t)
	reg := tool.NewRegistry()
	perms := NewPermissions()
	perms.Grant("tok", "*")

	handler := func(ctx context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		return &gateway.OutgoingMessage{Text: "echo: " + in.Text, UserID: in.UserID, ChatID: in.ChatID}, nil
	}

	srv := NewServer(addr, "hermind-test", reg, handler, perms)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Run(ctx) }()

	// Wait for server to accept.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("tcp", addr); err == nil {
			c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	base := "http://" + addr
	postAuthed := func(path, body string) (*http.Response, error) {
		req, _ := http.NewRequest("POST", base+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer tok")
		return http.DefaultClient.Do(req)
	}

	// .well-known/agent.json (no auth needed).
	resp, err := http.Get(base + "/.well-known/agent.json")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("well-known: %v %v", err, resp)
	}
	var wellKnown map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&wellKnown)
	resp.Body.Close()
	if wellKnown["name"] != "hermind-test" {
		t.Errorf("name = %v", wellKnown["name"])
	}

	// Create session.
	resp, err = postAuthed("/acp/sessions", `{"user":"alice"}`)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("create session: %v %v", err, resp)
	}
	var sessResp createSessionResponse
	_ = json.NewDecoder(resp.Body).Decode(&sessResp)
	resp.Body.Close()
	if sessResp.SessionID == "" {
		t.Fatal("missing session id")
	}

	// Send message.
	msgBody, _ := json.Marshal(messageRequest{SessionID: sessResp.SessionID, Text: "hi"})
	resp, err = postAuthed("/acp/messages", string(msgBody))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("send message: %v %v", err, resp)
	}
	var msgResp messageResponse
	_ = json.NewDecoder(resp.Body).Decode(&msgResp)
	resp.Body.Close()
	if !strings.Contains(msgResp.Text, "echo: hi") {
		t.Errorf("reply = %q", msgResp.Text)
	}

	// Missing auth is rejected.
	req, _ := http.NewRequest("POST", base+"/acp/messages", bytes.NewBufferString(`{}`))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestNotifier_AgentMessageChunk(t *testing.T) {
	var out bytes.Buffer
	n := NewNotifier(&out, nil)
	n.AgentMessageChunk("s1", "hel")
	n.AgentMessageChunk("s1", "lo")
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 frames, got %d: %q", len(lines), out.String())
	}
	for _, l := range lines {
		if !strings.Contains(l, `"method":"session/update"`) {
			t.Errorf("missing method: %s", l)
		}
		if !strings.Contains(l, `"sessionUpdate":"agent_message_chunk"`) {
			t.Errorf("missing update kind: %s", l)
		}
	}
}

func TestNotifier_ToolCallStartAndUpdate(t *testing.T) {
	var out bytes.Buffer
	n := NewNotifier(&out, nil)
	id, err := n.ToolCallStart("s1", "read_file", "execute", `{"path":"/tmp/x"}`)
	if err != nil || id == "" {
		t.Fatalf("ToolCallStart err=%v id=%q", err, id)
	}
	n.ToolCallUpdate("s1", id, "completed", "contents here")

	frames := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}
	var start map[string]any
	if err := json.Unmarshal([]byte(frames[0]), &start); err != nil {
		t.Fatal(err)
	}
	params := start["params"].(map[string]any)
	update := params["update"].(map[string]any)
	if update["sessionUpdate"] != "tool_call_start" {
		t.Errorf("first frame kind = %v", update["sessionUpdate"])
	}
	if update["toolCallId"] != id {
		t.Errorf("id mismatch: %v vs %v", update["toolCallId"], id)
	}
}

func TestNotifier_SerializedWrites(t *testing.T) {
	var out bytes.Buffer
	var mu sync.Mutex
	n := NewNotifier(&out, &mu)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n.AgentMessageChunk("s", "x")
		}()
	}
	wg.Wait()
	// Every line must parse — if writes interleaved, some would be garbled.
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var tmp map[string]any
		if err := json.Unmarshal([]byte(l), &tmp); err != nil {
			t.Errorf("corrupt frame: %q", l)
		}
	}
}

func TestPermissionBroker_AllowOnce(t *testing.T) {
	outbox := make(chan []byte, 4)
	broker := NewPermissionBroker(func(data []byte) { outbox <- data }, 1*time.Second)

	outcomeCh := make(chan PermissionOutcome, 1)
	go func() {
		oc, _ := broker.Request(context.Background(), "s1", "run rm -rf /tmp/x", "execute")
		outcomeCh <- oc
	}()

	frame := <-outbox
	var req struct {
		ID     json.Number     `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(frame), &req); err != nil {
		t.Fatalf("unmarshal frame: %v", err)
	}
	if req.Method != "session/request_permission" {
		t.Errorf("method = %q", req.Method)
	}

	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result": map[string]any{
			"outcome": map[string]any{"optionId": "allow_once"},
		},
	}
	raw, _ := json.Marshal(resp)
	broker.HandleResponse(raw)

	select {
	case oc := <-outcomeCh:
		if oc != PermissionAllowOnce {
			t.Errorf("outcome = %v", oc)
		}
	case <-time.After(time.Second):
		t.Fatal("Request did not return")
	}
}

func TestPermissionBroker_TimeoutDenies(t *testing.T) {
	outbox := make(chan []byte, 4)
	broker := NewPermissionBroker(func(data []byte) { outbox <- data }, 50*time.Millisecond)

	oc, err := broker.Request(context.Background(), "s1", "ls", "execute")
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timeout, got %v", err)
	}
	if oc != PermissionDeny {
		t.Errorf("outcome = %v", oc)
	}
}

func TestBuildRegistry_DefaultShape(t *testing.T) {
	data := BuildRegistry(RegistryOpts{Version: "1.2.3"})
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "hermind" {
		t.Errorf("name = %v", got["name"])
	}
	if got["display_name"] != "Hermind" {
		t.Errorf("display_name = %v", got["display_name"])
	}
	dist, _ := got["distribution"].(map[string]any)
	if dist["type"] != "command" {
		t.Errorf("distribution.type = %v", dist["type"])
	}
	if dist["command"] != "hermind" {
		t.Errorf("distribution.command = %v", dist["command"])
	}
	args, _ := dist["args"].([]any)
	if len(args) != 1 || args[0] != "acp" {
		t.Errorf("distribution.args = %v", args)
	}
}

func TestBuildRegistry_CustomBinary(t *testing.T) {
	data := BuildRegistry(RegistryOpts{
		Version:    "1.0.0",
		BinaryPath: "/usr/local/bin/hermind-dev",
	})
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	dist := got["distribution"].(map[string]any)
	if dist["command"] != "/usr/local/bin/hermind-dev" {
		t.Errorf("command = %v", dist["command"])
	}
}

func TestSessionManager_ForkCopiesHistory(t *testing.T) {
	m := NewSessionManager()
	s, err := m.Create(context.Background(), "/tmp", "stub/model")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.AppendUserText(context.Background(), s.ID, "hello"); err != nil {
		t.Fatal(err)
	}
	fork, err := m.Fork(context.Background(), s.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if fork.ID == s.ID {
		t.Fatalf("fork returned same id")
	}
	msgs, err := m.History(context.Background(), fork.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Text != "hello" || msgs[0].Role != "user" {
		t.Errorf("forked history = %+v", msgs)
	}
}

func TestSessionManager_ListAndSetModel(t *testing.T) {
	m := NewSessionManager()
	s1, _ := m.Create(context.Background(), "/tmp", "m1")
	_, _ = m.Create(context.Background(), "/tmp", "m2")

	all, err := m.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 2 {
		t.Errorf("list = %d sessions", len(all))
	}

	if err := m.SetModel(context.Background(), s1.ID, "m3"); err != nil {
		t.Fatal(err)
	}
	after, _ := m.Get(context.Background(), s1.ID)
	if after.Model != "m3" {
		t.Errorf("model = %q", after.Model)
	}
}
