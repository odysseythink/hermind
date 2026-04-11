# Plan 6e: Vision Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Give the agent a `vision_analyze` tool that downloads or references an image URL, routes it to a vision-capable LLM via the auxiliary provider, and returns a text description to the calling agent.

**Architecture:** A new `tool/vision` package holds the tool. The handler builds a `message.Message` with a structured `BlockContent` (one text block, one image_url block) and calls `provider.Provider.Complete`. The provider is injected at registration time (same pattern as delegate). The CLI wires the auxiliary provider in if it's configured; otherwise the tool is not registered.

**Tech Stack:** Go 1.25, stdlib `net/http` for image download, existing `provider`, `message`, `tool` packages. No new deps.

**Deliverable at end of plan:**
```
> describe https://upload.wikimedia.org/example.jpg
⚡ vision_analyze: {"image_url":"https://...","prompt":"describe this image"}
│ {"description":"A golden retriever puppy chasing a red ball on a grass lawn..."}
└
```

**Non-goals for this plan (deferred):**
- Local file paths (only URLs for Plan 6e)
- Image generation / editing (different feature)
- Multi-image batch analysis
- OCR-specific tools (the LLM handles it implicitly)
- Caching vision results

**Plans 1-6d dependencies this plan touches:**
- `tool/vision/` — NEW package
- `cli/repl.go` — wire vision tool with auxProvider

---

## File Structure

```
hermes-agent-go/
├── tool/
│   └── vision/                         # NEW
│       ├── vision.go                   # handler + Register
│       └── vision_test.go
└── cli/repl.go                         # MODIFIED
```

---

## Task 1: vision package

- [ ] **Step 1:** Create `tool/vision/vision.go`:

```go
// Package vision provides an image analysis tool backed by a
// vision-capable LLM. The handler constructs a multimodal message with
// the user's prompt and an image_url block, sends it via the provided
// Provider, and returns the assistant's text reply.
package vision

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/tool"
)

const visionSchema = `{
  "type":"object",
  "properties":{
    "image_url":{"type":"string","description":"Public URL of the image to analyze"},
    "prompt":{"type":"string","description":"Question or instruction for the image (default: describe)"},
    "detail":{"type":"string","enum":["low","high","auto"],"description":"Image detail level (OpenAI-style)"}
  },
  "required":["image_url"]
}`

// Args is the JSON shape vision_analyze accepts.
type Args struct {
	ImageURL string `json:"image_url"`
	Prompt   string `json:"prompt,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Model    string `json:"model,omitempty"`
}

// Result is the JSON shape vision_analyze returns.
type Result struct {
	Description string `json:"description"`
	Model       string `json:"model,omitempty"`
}

// newHandler builds the tool handler that calls prov.Complete with a
// multimodal request. defaultModel is the model name used when the
// caller doesn't specify one (typically the auxiliary provider's
// configured model).
func newHandler(prov provider.Provider, defaultModel string) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args Args
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if strings.TrimSpace(args.ImageURL) == "" {
			return tool.ToolError("image_url is required"), nil
		}
		prompt := args.Prompt
		if strings.TrimSpace(prompt) == "" {
			prompt = "Describe this image in detail."
		}
		detail := args.Detail
		if detail == "" {
			detail = "auto"
		}
		model := args.Model
		if model == "" {
			model = defaultModel
		}

		req := &provider.Request{
			Model: model,
			Messages: []message.Message{
				{
					Role: message.RoleUser,
					Content: message.BlockContent([]message.ContentBlock{
						{Type: "text", Text: prompt},
						{Type: "image_url", ImageURL: &message.Image{
							URL:    args.ImageURL,
							Detail: detail,
						}},
					}),
				},
			},
		}

		resp, err := prov.Complete(ctx, req)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("vision call: %v", err)), nil
		}
		if resp == nil || resp.Message == nil {
			return tool.ToolError("vision: empty response"), nil
		}
		text := resp.Message.Content.Text()
		if text == "" {
			// Fall back to concatenating text blocks if the provider
			// returned a structured form.
			for _, b := range resp.Message.Content.Blocks() {
				if b.Type == "text" {
					text += b.Text
				}
			}
		}
		return tool.ToolResult(Result{Description: text, Model: model}), nil
	}
}

// Register registers vision_analyze into reg with the given provider.
// If prov is nil the tool is not registered (matches delegate pattern).
func Register(reg *tool.Registry, prov provider.Provider, defaultModel string) {
	if prov == nil {
		return
	}
	reg.Register(&tool.Entry{
		Name:        "vision_analyze",
		Toolset:     "vision",
		Description: "Analyze an image URL using a vision-capable LLM.",
		Emoji:       "👁",
		Handler:     newHandler(prov, defaultModel),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "vision_analyze",
				Description: "Analyze an image. Supply a public image_url and an optional prompt.",
				Parameters:  json.RawMessage(visionSchema),
			},
		},
	})
}
```

- [ ] **Step 2:** Create `tool/vision/vision_test.go` with a fake provider:

```go
package vision

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/tool"
)

// fakeProvider captures the request and returns a canned response.
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
		Message: &message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent(f.resp),
		},
	}, nil
}
func (f *fakeProvider) Stream(context.Context, *provider.Request) (provider.Stream, error) {
	return nil, errors.New("stream not supported")
}
func (f *fakeProvider) ModelInfo(string) *provider.ModelInfo        { return nil }
func (f *fakeProvider) EstimateTokens(string, string) (int, error)  { return 0, nil }
func (f *fakeProvider) Available() bool                             { return true }

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

	// Verify the request message carries a structured image block.
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
```

- [ ] **Step 3:** `go test ./tool/vision/...` — PASS.
- [ ] **Step 4:** Commit `feat(tool/vision): add vision_analyze tool`.

---

## Task 2: CLI wiring

- [ ] **Step 1:** In `cli/repl.go`, import `"github.com/nousresearch/hermes-agent/tool/vision"` and after the browser tools block:

```go
// Vision tool (Plan 6e). Only registered when the auxiliary provider
// is configured — otherwise there's nobody to send the image to.
visionModel := app.Config.Auxiliary.Model
if visionModel == "" {
    visionModel = displayModel
}
vision.Register(toolRegistry, auxProvider, visionModel)
```

- [ ] **Step 2:** `go build ./... && go test ./...` — PASS.
- [ ] **Step 3:** Commit `feat(cli): register vision_analyze when aux provider configured`.

---

## Verification Checklist

- [ ] `go test ./...` passes
- [ ] Without `auxiliary` config block, vision_analyze is not registered
- [ ] With `auxiliary` set to any capable provider, it is registered
