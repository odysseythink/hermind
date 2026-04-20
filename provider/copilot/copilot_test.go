package copilot

import (
	"context"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
)

func TestNew_Fields(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "copilot",
		Model:    "copilot",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "copilot" {
		t.Errorf("name = %q", p.Name())
	}
	// Available should be true as long as a command path is configured.
	if !p.Available() {
		t.Error("expected available = true by default")
	}
}

func TestModelInfo_DefaultsAreReasonable(t *testing.T) {
	p, _ := New(config.ProviderConfig{Provider: "copilot", Model: "copilot"})
	info := p.ModelInfo("copilot")
	if info == nil || info.ContextLength < 8_000 {
		t.Errorf("bad model info: %+v", info)
	}
	if !info.SupportsTools {
		t.Error("copilot must advertise tool support")
	}
}

func TestEstimateTokens_CharHeuristic(t *testing.T) {
	p, _ := New(config.ProviderConfig{Provider: "copilot", Model: "copilot"})
	n, _ := p.EstimateTokens("copilot", "hello world") // 11 chars
	if n < 2 || n > 4 {
		t.Errorf("expected ~3 tokens, got %d", n)
	}
}

func TestSubprocess_EchoInitialize(t *testing.T) {
	bin := buildFakeCopilot(t) // helper defined in fake_copilot_test.go
	t.Setenv("HERMIND_COPILOT_COMMAND", bin)
	t.Setenv("HERMIND_COPILOT_ARGS", "echo")

	p, err := New(config.ProviderConfig{Provider: "copilot", Model: "copilot"})
	if err != nil {
		t.Fatal(err)
	}
	c := p.(*Copilot)
	defer c.Close()

	// Force subprocess spin-up.
	if _, err := c.ensureSubprocess(); err != nil {
		t.Fatalf("ensureSubprocess: %v", err)
	}
	if c.sub == nil {
		t.Fatal("sub was not initialized")
	}
}

func TestExtractToolCalls_SingleBlock(t *testing.T) {
	input := `Sure, let me help.
<tool_call>{"id":"t1","name":"shell","arguments":{"cmd":"ls"}}</tool_call>
Done.`
	calls, cleaned := ExtractToolCalls(input)
	if len(calls) != 1 {
		t.Fatalf("calls = %d", len(calls))
	}
	if calls[0].Name != "shell" {
		t.Errorf("name = %q", calls[0].Name)
	}
	if !strings.Contains(string(calls[0].Arguments), `"cmd":"ls"`) {
		t.Errorf("arguments = %s", calls[0].Arguments)
	}
	if strings.Contains(cleaned, "<tool_call>") {
		t.Errorf("cleaned still has block: %q", cleaned)
	}
}

func TestExtractToolCalls_NoBlocksReturnsOriginal(t *testing.T) {
	input := "just text"
	calls, cleaned := ExtractToolCalls(input)
	if len(calls) != 0 {
		t.Errorf("unexpected calls: %v", calls)
	}
	if cleaned != input {
		t.Errorf("cleaned = %q", cleaned)
	}
}

func TestExtractToolCalls_MultipleBlocks(t *testing.T) {
	input := `<tool_call>{"id":"a","name":"read","arguments":{}}</tool_call>` +
		` and ` +
		`<tool_call>{"id":"b","name":"write","arguments":{}}</tool_call>`
	calls, _ := ExtractToolCalls(input)
	if len(calls) != 2 {
		t.Fatalf("calls = %d", len(calls))
	}
	if calls[0].ID != "a" || calls[1].ID != "b" {
		t.Errorf("ids = %v", calls)
	}
}

func TestBuildPrompt_IncludesSystemAndHistory(t *testing.T) {
	req := &provider.Request{
		SystemPrompt: "be helpful",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("hello")},
			{Role: message.RoleAssistant, Content: message.TextContent("hi")},
			{Role: message.RoleUser, Content: message.TextContent("next")},
		},
	}
	out := BuildPrompt(req)
	for _, want := range []string{"be helpful", "hello", "hi", "next"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in prompt:\n%s", want, out)
		}
	}
}

func TestBuildPrompt_EmitsToolSchema(t *testing.T) {
	req := &provider.Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("run")}},
		Tools: []tool.ToolDefinition{{
			Type:     "function",
			Function: tool.FunctionDef{Name: "shell", Description: "run a shell cmd", Parameters: []byte(`{"type":"object"}`)},
		}},
	}
	out := BuildPrompt(req)
	if !strings.Contains(out, `"name":"shell"`) {
		t.Errorf("missing tool schema: %s", out)
	}
}

func TestComplete_WithFakeCopilot(t *testing.T) {
	bin := buildFakeCopilot(t)
	t.Setenv("HERMIND_COPILOT_COMMAND", bin)
	t.Setenv("HERMIND_COPILOT_ARGS", "")

	p, err := New(config.ProviderConfig{Provider: "copilot", Model: "copilot"})
	if err != nil {
		t.Fatal(err)
	}
	defer p.(*Copilot).Close()

	resp, err := p.Complete(context.Background(), &provider.Request{
		Model: "copilot",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("hi")},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.FinishReason != "end_turn" {
		t.Errorf("finish = %q", resp.FinishReason)
	}
	_ = resp.Message
}
