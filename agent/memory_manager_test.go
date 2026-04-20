// agent/memory_manager_test.go
package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
)

// recordingMemProvider counts SyncTurn / Shutdown calls and can be
// made to fail on demand.
type recordingMemProvider struct {
	name     string
	turns    int
	closed   bool
	syncErr  error
	closeErr error
}

func (r *recordingMemProvider) Name() string { return r.name }
func (r *recordingMemProvider) Initialize(context.Context, string) error {
	return nil
}
func (r *recordingMemProvider) RegisterTools(*tool.Registry) {}
func (r *recordingMemProvider) SyncTurn(_ context.Context, _, _ string) error {
	r.turns++
	return r.syncErr
}
func (r *recordingMemProvider) Shutdown(context.Context) error {
	r.closed = true
	return r.closeErr
}

func TestMemoryManager_NoProvidersEmptyDigest(t *testing.T) {
	mm := NewMemoryManager(nil)
	if got := mm.BuiltinDigest(); got != "" {
		t.Errorf("expected empty digest, got %q", got)
	}
	prompt, err := mm.BuildSystemPrompt(context.Background(), "anything")
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "" {
		t.Errorf("expected empty prompt, got %q", prompt)
	}
}

func TestMemoryManager_BuiltinDigestsRecentTurns(t *testing.T) {
	mm := NewMemoryManager(nil)
	mm.ObserveTurn(message.Message{
		Role:    message.RoleUser,
		Content: message.TextContent("my name is Alice"),
	})
	mm.ObserveTurn(message.Message{
		Role:    message.RoleAssistant,
		Content: message.TextContent("nice to meet you Alice"),
	})

	digest := mm.BuiltinDigest()
	if !strings.Contains(digest, "Alice") {
		t.Errorf("digest missing name: %s", digest)
	}
	if !strings.Contains(digest, "user") || !strings.Contains(digest, "assistant") {
		t.Errorf("digest missing role labels: %s", digest)
	}
}

func TestMemoryManager_ObserveTurnRingLimit(t *testing.T) {
	mm := NewMemoryManager(nil)
	for i := 0; i < 50; i++ {
		mm.ObserveTurn(message.Message{
			Role:    message.RoleUser,
			Content: message.TextContent("msg"),
		})
	}
	// The ring is capped at 20; digest should contain at most 20 lines.
	digest := mm.BuiltinDigest()
	lines := strings.Count(digest, "\n- ")
	if lines > 20 {
		t.Errorf("digest retained too many turns: %d lines", lines)
	}
}

func TestMemoryManager_SyncTurnFansOut(t *testing.T) {
	p1 := &recordingMemProvider{name: "a"}
	p2 := &recordingMemProvider{name: "b"}
	mm := NewMemoryManager(nil)
	mm.AddProvider(p1)
	mm.AddProvider(p2)
	if err := mm.SyncTurn(context.Background(), "hi", "hello"); err != nil {
		t.Fatalf("SyncTurn: %v", err)
	}
	if p1.turns != 1 || p2.turns != 1 {
		t.Errorf("sync turn fan-out: p1=%d p2=%d", p1.turns, p2.turns)
	}
}

func TestMemoryManager_SyncTurnCollectsErrors(t *testing.T) {
	p1 := &recordingMemProvider{name: "a", syncErr: errors.New("boom")}
	p2 := &recordingMemProvider{name: "b"}
	mm := NewMemoryManager([]memprovider.Provider{p1, p2})
	err := mm.SyncTurn(context.Background(), "hi", "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "a: boom") {
		t.Errorf("expected error to mention failing provider, got %v", err)
	}
	// p2 should still have run despite p1's failure.
	if p2.turns != 1 {
		t.Errorf("expected p2 to run even when p1 failed, turns=%d", p2.turns)
	}
}

func TestMemoryManager_ShutdownClosesAllProviders(t *testing.T) {
	p1 := &recordingMemProvider{name: "a"}
	p2 := &recordingMemProvider{name: "b"}
	mm := NewMemoryManager([]memprovider.Provider{p1, p2})
	if err := mm.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if !p1.closed || !p2.closed {
		t.Errorf("expected both providers closed, got p1=%v p2=%v", p1.closed, p2.closed)
	}
	// After shutdown, provider list is empty.
	if got := mm.Providers(); len(got) != 0 {
		t.Errorf("expected empty provider list after shutdown, got %d", len(got))
	}
}

func TestMemoryManager_SummarizeUsesAux(t *testing.T) {
	stub := &stubAuxProvider{response: "brief summary"}
	ac := provider.NewAuxClient([]provider.Provider{stub})
	mm := NewMemoryManager(nil)
	mm.SetAuxClient(ac)
	out, err := mm.Summarize(context.Background(), "lots of text to summarize "+strings.Repeat("x", 200))
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if out != "brief summary" {
		t.Errorf("expected aux response, got %q", out)
	}
}

func TestMemoryManager_SummarizeReturnsInputWithoutAux(t *testing.T) {
	mm := NewMemoryManager(nil)
	out, err := mm.Summarize(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected passthrough, got %q", out)
	}
}

func TestMemoryManager_RetrieveUsesDigestAndAux(t *testing.T) {
	stub := &stubAuxProvider{response: "Alice-related line"}
	ac := provider.NewAuxClient([]provider.Provider{stub})
	mm := NewMemoryManager(nil)
	mm.SetAuxClient(ac)
	mm.ObserveTurn(message.Message{
		Role: message.RoleUser, Content: message.TextContent("my name is Alice"),
	})
	out, err := mm.Retrieve(context.Background(), "what is the user's name?")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("expected re-ranked digest to mention Alice, got %q", out)
	}
}

func TestMemoryManager_RetrieveReturnsDigestWhenNoAux(t *testing.T) {
	mm := NewMemoryManager(nil)
	mm.ObserveTurn(message.Message{
		Role: message.RoleUser, Content: message.TextContent("fact"),
	})
	out, err := mm.Retrieve(context.Background(), "query")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if !strings.Contains(out, "fact") {
		t.Errorf("expected raw digest, got %q", out)
	}
}

func TestMemoryManager_CompressUsesAttachedCompressor(t *testing.T) {
	aux := &stubAuxProvider{response: "compressed summary"}
	c := NewCompressor(config.CompressionConfig{
		Enabled:     true,
		ProtectLast: 2,
		MaxPasses:   1,
	}, aux)
	mm := NewMemoryManager(nil)
	mm.SetCompressor(c)

	hist := make([]message.Message, 0, 10)
	for i := 0; i < 10; i++ {
		hist = append(hist, message.Message{
			Role:    message.RoleUser,
			Content: message.TextContent("msg"),
		})
	}
	out, passes, err := mm.Compress(context.Background(), hist, 100)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if len(out) >= len(hist) {
		t.Errorf("expected shorter history, got %d -> %d", len(hist), len(out))
	}
	if passes != 1 {
		t.Errorf("expected passes=1, got %d", passes)
	}
}

func TestMemoryManager_CompressNoOpWithoutCompressor(t *testing.T) {
	mm := NewMemoryManager(nil)
	hist := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("hi")},
	}
	out, passes, err := mm.Compress(context.Background(), hist, 4000)
	if err != nil {
		t.Fatal(err)
	}
	if passes != 0 {
		t.Errorf("expected passes=0, got %d", passes)
	}
	if len(out) != len(hist) {
		t.Errorf("history changed unexpectedly")
	}
}
