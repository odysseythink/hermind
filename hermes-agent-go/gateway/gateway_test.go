package gateway

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/tool"
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
