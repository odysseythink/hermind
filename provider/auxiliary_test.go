// provider/auxiliary_test.go
package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/message"
)

type countingProvider struct {
	name  string
	fails int
	seen  int
}

func (p *countingProvider) Name() string                          { return p.name }
func (p *countingProvider) Available() bool                       { return true }
func (p *countingProvider) ModelInfo(string) *ModelInfo           { return &ModelInfo{ContextLength: 10000} }
func (p *countingProvider) EstimateTokens(_, _ string) (int, error) { return 1, nil }
func (p *countingProvider) Stream(context.Context, *Request) (Stream, error) {
	return nil, errors.New("not supported")
}
func (p *countingProvider) Complete(_ context.Context, _ *Request) (*Response, error) {
	p.seen++
	if p.fails > 0 {
		p.fails--
		return nil, &Error{Kind: ErrServerError, Provider: p.name, Message: "boom"}
	}
	return &Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent("ok from " + p.name),
		},
	}, nil
}

func TestAuxClient_AskUsesFirstWorking(t *testing.T) {
	p1 := &countingProvider{name: "openrouter", fails: 1}
	p2 := &countingProvider{name: "nous"}
	ac := NewAuxClient([]Provider{p1, p2})
	text, err := ac.Ask(context.Background(), "summarize", "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "nous") {
		t.Errorf("got %q, want response from nous", text)
	}
	if p1.seen != 1 || p2.seen != 1 {
		t.Errorf("seen: p1=%d p2=%d", p1.seen, p2.seen)
	}
}

func TestAuxClient_EmptyChainIsError(t *testing.T) {
	ac := NewAuxClient(nil)
	if _, err := ac.Ask(context.Background(), "x", "y"); err == nil {
		t.Error("expected error on empty chain")
	}
}

func TestAuxClient_SkipsNilEntries(t *testing.T) {
	p := &countingProvider{name: "solo"}
	ac := NewAuxClient([]Provider{nil, p, nil})
	if len(ac.Providers()) != 1 {
		t.Fatalf("expected 1 provider after nil filtering, got %d", len(ac.Providers()))
	}
	text, err := ac.Ask(context.Background(), "sys", "usr")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "solo") {
		t.Errorf("got %q, want response from solo", text)
	}
}

func TestAuxClient_AskWithRequestUsesCustomRequest(t *testing.T) {
	p := &countingProvider{name: "primary"}
	ac := NewAuxClient([]Provider{p})
	req := &Request{
		SystemPrompt: "cheap",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("context")},
		},
		MaxTokens: 100,
	}
	text, err := ac.AskWithRequest(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "primary") {
		t.Errorf("got %q", text)
	}
}

func TestAuxClient_CompleteReturnsResponse(t *testing.T) {
	p := &countingProvider{name: "ok"}
	ac := NewAuxClient([]Provider{p})
	resp, err := ac.Complete(context.Background(), &Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil || !strings.Contains(resp.Message.Content.Text(), "ok") {
		t.Errorf("unexpected response: %+v", resp)
	}
}
