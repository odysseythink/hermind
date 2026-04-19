package gateway

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tracing"
)

// fakeStream returns a single delta then Done.
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

// echoProvider implements provider.Provider by echoing the last user message.
type echoProvider struct{}

func (echoProvider) Name() string { return "echo" }
func (echoProvider) Complete(context.Context, *provider.Request) (*provider.Response, error) {
	return nil, errors.New("not used")
}
func (echoProvider) Stream(_ context.Context, req *provider.Request) (provider.Stream, error) {
	last := req.Messages[len(req.Messages)-1]
	text := "echo: " + last.Content.Text()
	return &fakeStream{
		events: []*provider.StreamEvent{
			{Type: provider.EventDelta, Delta: &provider.StreamDelta{Content: text}},
			{Type: provider.EventDone, Response: &provider.Response{
				Message: message.Message{
					Role:    message.RoleAssistant,
					Content: message.TextContent(text),
				},
				FinishReason: "end_turn",
				Usage:        message.Usage{InputTokens: 1, OutputTokens: 1},
			}},
		},
	}, nil
}
func (echoProvider) ModelInfo(string) *provider.ModelInfo       { return &provider.ModelInfo{ContextLength: 8000} }
func (echoProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (echoProvider) Available() bool                            { return true }

// fakePlatform sends one canned message, records replies, and blocks
// until ctx is cancelled.
type fakePlatform struct {
	name    string
	send    IncomingMessage
	mu      sync.Mutex
	replies []OutgoingMessage
	started chan struct{}
}

func (f *fakePlatform) Name() string { return f.name }
func (f *fakePlatform) Run(ctx context.Context, h MessageHandler) error {
	close(f.started)
	DispatchAndReply(ctx, f, h, f.send)
	<-ctx.Done()
	return nil
}
func (f *fakePlatform) SendReply(_ context.Context, out OutgoingMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = append(f.replies, out)
	return nil
}

// flakyProvider fails the first N times then succeeds by echoing the
// last user message. Used to exercise runWithRetry.
type flakyProvider struct {
	mu       sync.Mutex
	failures int
	succeeds int
}

func (f *flakyProvider) Name() string { return "flaky" }
func (f *flakyProvider) Complete(context.Context, *provider.Request) (*provider.Response, error) {
	return nil, errors.New("not used")
}
func (f *flakyProvider) Stream(_ context.Context, req *provider.Request) (provider.Stream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failures > 0 {
		f.failures--
		return nil, errors.New("transient")
	}
	f.succeeds++
	last := req.Messages[len(req.Messages)-1]
	text := "echo: " + last.Content.Text()
	return &fakeStream{
		events: []*provider.StreamEvent{
			{Type: provider.EventDelta, Delta: &provider.StreamDelta{Content: text}},
			{Type: provider.EventDone, Response: &provider.Response{
				Message: message.Message{Role: message.RoleAssistant, Content: message.TextContent(text)},
				FinishReason: "end_turn",
			}},
		},
	}, nil
}
func (flakyProvider) ModelInfo(string) *provider.ModelInfo       { return nil }
func (flakyProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (flakyProvider) Available() bool                            { return true }

func TestGatewayTracerRecordsSpan(t *testing.T) {
	exp := tracing.NewMemoryExporter()
	tr := tracing.NewTracer(exp)
	cfg := config.Config{
		Model: "anthropic/claude-opus-4-6",
		Agent: config.AgentConfig{MaxTurns: 3},
	}
	g := NewGateway(cfg, echoProvider{}, nil, nil, tool.NewRegistry())
	g.SetTracer(tr)
	in := IncomingMessage{Platform: "fake", UserID: "u-trace", Text: "hello"}
	if _, err := g.handleMessage(context.Background(), in); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}
	spans := exp.Spans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "gateway.handleMessage" {
		t.Errorf("span name = %q", spans[0].Name)
	}
	if spans[0].Status != tracing.StatusOK {
		t.Errorf("status = %d, want OK", spans[0].Status)
	}
	var foundPlatform bool
	for _, a := range spans[0].Attributes {
		if a.Key == "platform" && a.Value == "fake" {
			foundPlatform = true
		}
	}
	if !foundPlatform {
		t.Errorf("missing platform attribute: %+v", spans[0].Attributes)
	}
}

func TestGatewayDedupSkipsDuplicate(t *testing.T) {
	cfg := config.Config{
		Model: "anthropic/claude-opus-4-6",
		Agent: config.AgentConfig{MaxTurns: 3},
	}
	g := NewGateway(cfg, echoProvider{}, nil, nil, tool.NewRegistry())
	ctx := context.Background()
	in := IncomingMessage{Platform: "fake", UserID: "u1", MessageID: "m1", Text: "hello"}
	out, err := g.handleMessage(ctx, in)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if out == nil || out.Text != "echo: hello" {
		t.Fatalf("first out = %+v", out)
	}
	// Duplicate should be skipped (out=nil, err=nil).
	out2, err := g.handleMessage(ctx, in)
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if out2 != nil {
		t.Errorf("expected nil out on duplicate, got %+v", out2)
	}
}

func TestGatewayRetryRecovers(t *testing.T) {
	cfg := config.Config{
		Model: "anthropic/claude-opus-4-6",
		Agent: config.AgentConfig{MaxTurns: 3},
	}
	fp := &flakyProvider{failures: 2}
	g := NewGateway(cfg, fp, nil, nil, tool.NewRegistry())
	ctx := context.Background()
	in := IncomingMessage{Platform: "fake", UserID: "u-retry", Text: "ping"}
	out, err := g.handleMessage(ctx, in)
	if err != nil {
		t.Fatalf("handleMessage: %v", err)
	}
	if out == nil || out.Text != "echo: ping" {
		t.Fatalf("out = %+v", out)
	}
	if fp.succeeds != 1 {
		t.Errorf("expected 1 success, got %d", fp.succeeds)
	}
}

func TestGatewayRoutesMessageAndReplies(t *testing.T) {
	fp := &fakePlatform{
		name:    "fake",
		send:    IncomingMessage{Platform: "fake", UserID: "u1", Text: "hello"},
		started: make(chan struct{}),
	}
	cfg := config.Config{
		Model: "anthropic/claude-opus-4-6",
		Agent: config.AgentConfig{MaxTurns: 3},
	}
	g := NewGateway(cfg, echoProvider{}, nil, nil, tool.NewRegistry())
	g.Register(fp)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- g.Start(ctx) }()
	<-fp.started
	// Give the dispatch goroutine time to send a reply.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		fp.mu.Lock()
		n := len(fp.replies)
		fp.mu.Unlock()
		if n >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-errCh

	fp.mu.Lock()
	defer fp.mu.Unlock()
	if len(fp.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(fp.replies))
	}
	if fp.replies[0].Text != "echo: hello" {
		t.Errorf("unexpected reply: %q", fp.replies[0].Text)
	}
}

func TestGateway_StopUnblocksStart(t *testing.T) {
	g := NewGateway(config.Config{}, nil, nil, nil, nil)
	g.Register(newStopTestPlatform("p1"))

	startErr := make(chan error, 1)
	go func() {
		startErr <- g.Start(context.Background())
	}()

	// Give Start a moment to enter its wait loop.
	time.Sleep(20 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := g.Stop(stopCtx); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	select {
	case err := <-startErr:
		if err != nil && err != context.Canceled {
			t.Errorf("Start returned %v, want nil or context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return within 1s of Stop")
	}
}

// stopTestPlatform is a minimal Platform that blocks in Run until ctx
// is cancelled, then returns ctx.Err().
type stopTestPlatform struct{ name string }

func newStopTestPlatform(name string) *stopTestPlatform { return &stopTestPlatform{name: name} }
func (p *stopTestPlatform) Name() string                 { return p.name }
func (p *stopTestPlatform) Run(ctx context.Context, _ MessageHandler) error {
	<-ctx.Done()
	return ctx.Err()
}
func (p *stopTestPlatform) SendReply(_ context.Context, _ OutgoingMessage) error { return nil }
