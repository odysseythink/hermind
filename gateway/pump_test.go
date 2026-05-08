package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/odysseythink/hermind/config"
)

// mockRunner implements Runner for testing.
type mockRunner struct {
	reply string
	err   error
	calls []string
}

func (m *mockRunner) RunTurn(_ context.Context, msg string) (string, error) {
	m.calls = append(m.calls, msg)
	return m.reply, m.err
}

// mockPlatform implements Platform for testing.
type mockPlatform struct {
	name     string
	runErr   error
	messages []IncomingMessage
	replies  []OutgoingMessage
}

func (m *mockPlatform) Name() string { return m.name }
func (m *mockPlatform) Run(ctx context.Context, handler MessageHandler) error {
	for _, msg := range m.messages {
		out, err := handler(ctx, msg)
		if err != nil {
			continue
		}
		if out != nil {
			m.replies = append(m.replies, *out)
		}
	}
	return m.runErr
}
func (m *mockPlatform) SendReply(_ context.Context, out OutgoingMessage) error {
	m.replies = append(m.replies, out)
	return nil
}

func TestNewPump_EmptyConfig(t *testing.T) {
	p, err := NewPump(config.GatewayConfig{}, &mockRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if p.HasPlatforms() {
		t.Error("should have no platforms for empty config")
	}
}

func TestNewPump_DisabledPlatformSkipped(t *testing.T) {
	cfg := config.GatewayConfig{
		Platforms: map[string]config.PlatformConfig{
			"bot1": {Type: "mock", Enabled: false},
		},
	}
	p, err := NewPump(cfg, &mockRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if p.HasPlatforms() {
		t.Error("disabled platform should not be registered")
	}
}

func TestNewPump_UnknownTypeSkipped(t *testing.T) {
	cfg := config.GatewayConfig{
		Platforms: map[string]config.PlatformConfig{
			"bot1": {Type: "unknowntype", Enabled: true},
		},
	}
	p, err := NewPump(cfg, &mockRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if p.HasPlatforms() {
		t.Error("unknown platform type should be skipped")
	}
}

func TestNewPump_RegisteredTypeBuilt(t *testing.T) {
	const testType = "mock_v2"
	mp := &mockPlatform{name: "test"}
	RegisterBuilder(testType, func(name string, opts map[string]string) (Platform, error) {
		return mp, nil
	})
	defer func() {
		builderMu.Lock()
		delete(builders, testType)
		builderMu.Unlock()
	}()

	cfg := config.GatewayConfig{
		Platforms: map[string]config.PlatformConfig{
			"bot1": {Type: testType, Enabled: true},
		},
	}
	p, err := NewPump(cfg, &mockRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if !p.HasPlatforms() {
		t.Error("registered+enabled platform should be present")
	}
}

func TestNewPump_BuilderError(t *testing.T) {
	const testType = "mock_err"
	RegisterBuilder(testType, func(name string, opts map[string]string) (Platform, error) {
		return nil, errors.New("build failed")
	})
	defer func() {
		builderMu.Lock()
		delete(builders, testType)
		builderMu.Unlock()
	}()

	cfg := config.GatewayConfig{
		Platforms: map[string]config.PlatformConfig{
			"bot1": {Type: testType, Enabled: true},
		},
	}
	_, err := NewPump(cfg, &mockRunner{})
	if err == nil {
		t.Error("builder error should propagate from NewPump")
	}
}

func TestPump_Handle_RoutesMsgToRunner(t *testing.T) {
	runner := &mockRunner{reply: "pong"}
	p := &Pump{runner: runner, dedup: NewDedup(64)}
	in := IncomingMessage{Platform: "test", ChatID: "c1", Text: "ping", MessageID: "1"}
	out, err := p.handle(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.Text != "pong" {
		t.Fatalf("expected reply 'pong', got %v", out)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "ping" {
		t.Error("runner should have been called once with 'ping'")
	}
}

func TestPump_Handle_DeduplicatesMessages(t *testing.T) {
	runner := &mockRunner{reply: "ok"}
	p := &Pump{runner: runner, dedup: NewDedup(64)}
	in := IncomingMessage{Platform: "test", MessageID: "dup-1", Text: "hello"}

	p.handle(context.Background(), in)
	out, err := p.handle(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Error("duplicate message should return nil, not a reply")
	}
	if len(runner.calls) != 1 {
		t.Errorf("runner should be called once, got %d calls", len(runner.calls))
	}
}

func TestPump_Handle_NoDedup_WhenNoMessageID(t *testing.T) {
	runner := &mockRunner{reply: "ok"}
	p := &Pump{runner: runner, dedup: NewDedup(64)}
	in := IncomingMessage{Platform: "test", MessageID: "", Text: "hello"}

	p.handle(context.Background(), in)
	p.handle(context.Background(), in)
	if len(runner.calls) != 2 {
		t.Errorf("messages without MessageID should not be deduped, got %d calls", len(runner.calls))
	}
}

func TestPump_Handle_RunnerError(t *testing.T) {
	runner := &mockRunner{err: errors.New("llm unavailable")}
	p := &Pump{runner: runner, dedup: NewDedup(64)}
	in := IncomingMessage{Platform: "test", Text: "hi", MessageID: "e1"}
	_, err := p.handle(context.Background(), in)
	if err == nil {
		t.Error("runner error should propagate from handle")
	}
}

func TestPump_Start_CancelsCleanly(t *testing.T) {
	const testType = "mock_block"
	RegisterBuilder(testType, func(name string, opts map[string]string) (Platform, error) {
		return &blockingPlatform{}, nil
	})
	defer func() {
		builderMu.Lock()
		delete(builders, testType)
		builderMu.Unlock()
	}()

	cfg := config.GatewayConfig{
		Platforms: map[string]config.PlatformConfig{
			"bot1": {Type: testType, Enabled: true},
		},
	}
	runner := &mockRunner{}
	p, err := NewPump(cfg, runner)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	p.Start(ctx) // should return after ctx is cancelled
}

// blockingPlatform blocks until ctx is cancelled.
type blockingPlatform struct{}

func (b *blockingPlatform) Name() string { return "blocking" }
func (b *blockingPlatform) Run(ctx context.Context, _ MessageHandler) error {
	<-ctx.Done()
	return nil
}
func (b *blockingPlatform) SendReply(_ context.Context, _ OutgoingMessage) error { return nil }
