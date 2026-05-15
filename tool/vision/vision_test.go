package vision

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

type fakeProvider struct {
	lastReq *core.Request
	resp    string
	err     error
}

func (f *fakeProvider) Provider() string { return "fake" }
func (f *fakeProvider) Model() string    { return "fake-model" }
func (f *fakeProvider) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return &core.Response{
		Message: core.Message{
			Role:    core.MESSAGE_ROLE_ASSISTANT,
			Content: []core.ContentParter{core.TextPart{Text: f.resp}},
		},
	}, nil
}
func (f *fakeProvider) Stream(context.Context, *core.Request) (core.StreamResponse, error) {
	return nil, errors.New("stream not supported")
}
func (f *fakeProvider) GenerateObject(context.Context, *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

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
	parts := fp.lastReq.Messages[0].Content
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if p, ok := parts[0].(core.TextPart); !ok || p.Text == "" {
		t.Errorf("expected text part first, got %+v", parts[0])
	}
	if p, ok := parts[1].(core.ImagePart); !ok || p.URL != "https://example.com/cube.png" {
		t.Errorf("expected image_url part, got %+v", parts[1])
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
