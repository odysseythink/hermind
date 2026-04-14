package vision

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
)

type fakeProvider struct {
	lastReq *provider.Request
	resp    string
	err     error
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent(f.resp),
		},
	}, nil
}
func (f *fakeProvider) Stream(context.Context, *provider.Request) (provider.Stream, error) {
	return nil, errors.New("stream not supported")
}
func (f *fakeProvider) ModelInfo(string) *provider.ModelInfo       { return nil }
func (f *fakeProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (f *fakeProvider) Available() bool                            { return true }

func TestVisionAnalyzeHappyPath(t *testing.T) {
	fp := &fakeProvider{resp: "A red cube on a blue background."}
	reg := tool.NewRegistry()
	Register(reg, fp, "gpt-4o")

	args, _ := json.Marshal(Args{
		ImageURL: "https://example.com/cube.png",
		Prompt:   "what's in the picture?",
	})
	out, err := reg.Dispatch(context.Background(), "vision_analyze", args)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out, "red cube") {
		t.Errorf("expected description in output, got %s", out)
	}

	if fp.lastReq == nil || len(fp.lastReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %+v", fp.lastReq)
	}
	blocks := fp.lastReq.Messages[0].Content.Blocks()
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "text" || blocks[0].Text == "" {
		t.Errorf("expected text block first, got %+v", blocks[0])
	}
	if blocks[1].Type != "image_url" || blocks[1].ImageURL == nil ||
		blocks[1].ImageURL.URL != "https://example.com/cube.png" {
		t.Errorf("expected image_url block, got %+v", blocks[1])
	}
}

func TestVisionAnalyzeMissingURL(t *testing.T) {
	fp := &fakeProvider{}
	reg := tool.NewRegistry()
	Register(reg, fp, "gpt-4o")
	out, err := reg.Dispatch(context.Background(), "vision_analyze", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out, "image_url is required") {
		t.Errorf("expected required error, got %s", out)
	}
}

func TestVisionRegisterNilProvider(t *testing.T) {
	reg := tool.NewRegistry()
	Register(reg, nil, "gpt-4o")
	if len(reg.Definitions(nil)) != 0 {
		t.Errorf("expected no tools registered with nil provider")
	}
}
