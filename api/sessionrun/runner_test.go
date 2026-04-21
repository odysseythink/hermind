package sessionrun

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
)

// --- fake hub ---

type fakeHub struct {
	mu     sync.Mutex
	events []Event
}

func (h *fakeHub) Publish(e Event) {
	h.mu.Lock()
	h.events = append(h.events, e)
	h.mu.Unlock()
}

func (h *fakeHub) types() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	t := make([]string, len(h.events))
	for i, e := range h.events {
		t[i] = e.Type
	}
	return t
}

// --- fake provider modeled on agent/engine_test.go ---

type fakeProvider struct {
	streamFn func() (provider.Stream, error)
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	return f.streamFn()
}
func (f *fakeProvider) ModelInfo(string) *provider.ModelInfo       { return nil }
func (f *fakeProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (f *fakeProvider) Available() bool                            { return true }

type fakeStream struct {
	events []*provider.StreamEvent
	idx    int
}

func (s *fakeStream) Recv() (*provider.StreamEvent, error) {
	if s.idx >= len(s.events) {
		return nil, io.EOF
	}
	ev := s.events[s.idx]
	s.idx++
	return ev, nil
}
func (s *fakeStream) Close() error { return nil }

// streamingProvider emits each delta in order, then a final Done event
// with the concatenated text as the assistant response.
func streamingProvider(deltas ...string) *fakeProvider {
	return &fakeProvider{
		streamFn: func() (provider.Stream, error) {
			evs := make([]*provider.StreamEvent, 0, len(deltas)+1)
			full := ""
			for _, d := range deltas {
				evs = append(evs, &provider.StreamEvent{
					Type:  provider.EventDelta,
					Delta: &provider.StreamDelta{Content: d},
				})
				full += d
			}
			evs = append(evs, &provider.StreamEvent{
				Type: provider.EventDone,
				Response: &provider.Response{
					Message: message.Message{
						Role:    message.RoleAssistant,
						Content: message.TextContent(full),
					},
					FinishReason: "end_turn",
					Usage:        message.Usage{InputTokens: 5, OutputTokens: 3},
				},
			})
			return &fakeStream{events: evs}, nil
		},
	}
}

func TestRun_HappyPath(t *testing.T) {
	hub := &fakeHub{}
	deps := Deps{
		Provider: streamingProvider("Hello, ", "world"),
		Storage:  nil, // NewEngineWithToolsAndAux accepts nil storage
		ToolReg:  tool.NewRegistry(),
		AgentCfg: config.AgentConfig{MaxTurns: 5},
		Hub:      hub,
	}
	err := Run(context.Background(), deps, Request{SessionID: "s1", UserMessage: "hi"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	types := hub.types()
	// Expected sequence: status(running), token, token, message_complete, status(idle)
	want := []string{"status", "token", "token", "message_complete", "status"}
	if len(types) != len(want) {
		t.Fatalf("events = %v, want %v", types, want)
	}
	for i, w := range want {
		if types[i] != w {
			t.Errorf("events[%d] = %q, want %q", i, types[i], w)
		}
	}
}

// scriptedProvider returns responses in order, one per Stream call,
// each wrapping the response in a single EventDone event.
type scriptedProvider struct {
	responses []*provider.Response
	idx       int
}

func (p *scriptedProvider) Name() string { return "script" }
func (p *scriptedProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	return nil, errors.New("not implemented")
}
func (p *scriptedProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	if p.idx >= len(p.responses) {
		return nil, errors.New("unexpected extra stream call")
	}
	resp := p.responses[p.idx]
	p.idx++
	return &fakeStream{events: []*provider.StreamEvent{{Type: provider.EventDone, Response: resp}}}, nil
}
func (p *scriptedProvider) ModelInfo(string) *provider.ModelInfo       { return nil }
func (p *scriptedProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (p *scriptedProvider) Available() bool                            { return true }

func TestRun_ToolCall(t *testing.T) {
	hub := &fakeHub{}
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "echo",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"echoed":true}`, nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "echo",
				Description: "echo input back",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
	})

	// Turn 1: assistant returns tool_use; turn 2: final text.
	p := &scriptedProvider{responses: []*provider.Response{
		{
			Message: message.Message{
				Role: message.RoleAssistant,
				Content: message.BlockContent([]message.ContentBlock{
					{
						Type:         "tool_use",
						ToolUseID:    "t1",
						ToolUseName:  "echo",
						ToolUseInput: json.RawMessage(`{}`),
					},
				}),
			},
			FinishReason: "tool_use",
			Usage:        message.Usage{InputTokens: 10, OutputTokens: 5},
		},
		{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: message.TextContent("done"),
			},
			FinishReason: "end_turn",
			Usage:        message.Usage{InputTokens: 15, OutputTokens: 8},
		},
	}}

	deps := Deps{
		Provider: p,
		Storage:  nil,
		ToolReg:  reg,
		AgentCfg: config.AgentConfig{MaxTurns: 5},
		Hub:      hub,
	}
	if err := Run(context.Background(), deps, Request{SessionID: "s1", UserMessage: "go"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	types := hub.types()
	sawCall, sawResult := false, false
	for _, ty := range types {
		if ty == "tool_call" {
			sawCall = true
		}
		if ty == "tool_result" {
			sawResult = true
		}
	}
	if !sawCall || !sawResult {
		t.Errorf("events missing tool_call/tool_result: %v", types)
	}
}

// erroringProvider returns an error from Stream.
type erroringProvider struct{ err error }

func (p *erroringProvider) Name() string { return "err" }
func (p *erroringProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	return nil, p.err
}
func (p *erroringProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	return nil, p.err
}
func (p *erroringProvider) ModelInfo(string) *provider.ModelInfo       { return nil }
func (p *erroringProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (p *erroringProvider) Available() bool                            { return true }

func TestRun_ProviderError(t *testing.T) {
	hub := &fakeHub{}
	deps := Deps{
		Provider: &erroringProvider{err: errors.New("provider down")},
		Storage:  nil,
		ToolReg:  tool.NewRegistry(),
		AgentCfg: config.AgentConfig{MaxTurns: 5},
		Hub:      hub,
	}
	err := Run(context.Background(), deps, Request{SessionID: "s1", UserMessage: "hi"})
	if err == nil || !strings.Contains(err.Error(), "provider down") {
		t.Fatalf("expected provider error, got %v", err)
	}
	types := hub.types()
	if len(types) == 0 || types[len(types)-1] != "status" {
		t.Errorf("last event should be status(error): %v", types)
	}
}

// blockingProvider blocks Stream until ctx done.
type blockingProvider struct{ block chan struct{} }

func (p *blockingProvider) Name() string { return "block" }
func (p *blockingProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	return nil, errors.New("complete not used")
}
func (p *blockingProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.block:
		return nil, ctx.Err()
	}
}
func (p *blockingProvider) ModelInfo(string) *provider.ModelInfo       { return nil }
func (p *blockingProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (p *blockingProvider) Available() bool                            { return true }

func TestRun_ContextCancelled(t *testing.T) {
	hub := &fakeHub{}
	block := make(chan struct{})
	p := &blockingProvider{block: block}
	deps := Deps{
		Provider: p, Storage: nil, ToolReg: tool.NewRegistry(),
		AgentCfg: config.AgentConfig{MaxTurns: 5}, Hub: hub,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, deps, Request{SessionID: "s1", UserMessage: "hi"}) }()
	cancel()
	close(block)

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	types := hub.types()
	if len(types) == 0 || types[len(types)-1] != "status" {
		t.Errorf("last event should be status(cancelled): %v", types)
	}
}

// panicProvider panics in Stream.
type panicProvider struct{}

func (p *panicProvider) Name() string { return "panic" }
func (p *panicProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	panic("boom")
}
func (p *panicProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	panic("boom")
}
func (p *panicProvider) ModelInfo(string) *provider.ModelInfo       { return nil }
func (p *panicProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (p *panicProvider) Available() bool                            { return true }

func TestRun_PanicRecovered(t *testing.T) {
	hub := &fakeHub{}
	deps := Deps{
		Provider: &panicProvider{}, Storage: nil, ToolReg: tool.NewRegistry(),
		AgentCfg: config.AgentConfig{MaxTurns: 5}, Hub: hub,
	}
	err := Run(context.Background(), deps, Request{SessionID: "s1", UserMessage: "hi"})
	if err == nil {
		t.Fatal("panic should surface as error")
	}
	types := hub.types()
	if len(types) == 0 || types[len(types)-1] != "status" {
		t.Errorf("last event should be status(error): %v", types)
	}
}
