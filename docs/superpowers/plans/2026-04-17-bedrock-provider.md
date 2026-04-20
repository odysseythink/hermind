# AWS Bedrock Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an AWS Bedrock `provider.Provider` implementation so hermind can talk to Claude (and other Bedrock-hosted models) through AWS's Converse API with IAM credentials instead of a vendor API key.

**Architecture:** New `provider/bedrock/` package mirroring `provider/anthropic/` (one-file-per-responsibility: constructor, types, complete, stream, errors). Uses the unified AWS Converse/ConverseStream API (`bedrockruntime.Client`) so we get tools, images, system prompts, and streaming across every model family without writing per-family adapters. Credentials come from the standard AWS chain (`aws.Config` loaded via `config.LoadDefaultConfig`), with an optional override block in `config.ProviderConfig` for ad-hoc access keys.

**Tech Stack:** Go 1.21+, `github.com/aws/aws-sdk-go-v2` (core, config, credentials), `github.com/aws/aws-sdk-go-v2/service/bedrockruntime` for the Converse API, existing `provider/` + `message/` + `tool/` packages.

---

## File Structure

- Modify: `go.mod` — add aws-sdk-go-v2 dependencies
- Modify: `config/config.go` — extend `ProviderConfig` with Bedrock-specific fields
- Modify: `config/loader_test.go` — cover the new fields
- Create: `provider/bedrock/bedrock.go` — constructor + Name/Available/ModelInfo/EstimateTokens
- Create: `provider/bedrock/convert.go` — request/response shape translation
- Create: `provider/bedrock/complete.go` — non-streaming `Complete()` on the Converse API
- Create: `provider/bedrock/stream.go` — streaming `Stream()` on ConverseStream
- Create: `provider/bedrock/errors.go` — AWS → `provider.Error` mapping
- Create: `provider/bedrock/bedrock_test.go` — end-to-end with an in-memory Converse API double
- Modify: `provider/factory/factory.go` — register the `bedrock` case
- Modify: `provider/factory/factory_test.go` — cover the new branch

---

## Task 1: Add aws-sdk-go-v2 dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the deps**

Run:

```bash
go get github.com/aws/aws-sdk-go-v2@latest
go get github.com/aws/aws-sdk-go-v2/config@latest
go get github.com/aws/aws-sdk-go-v2/credentials@latest
go get github.com/aws/aws-sdk-go-v2/service/bedrockruntime@latest
```

- [ ] **Step 2: Verify the module still builds**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit the vendoring change**

```bash
git add go.mod go.sum
git commit -m "build: add aws-sdk-go-v2 + bedrockruntime"
```

---

## Task 2: Extend ProviderConfig with Bedrock fields

**Files:**
- Modify: `config/config.go`
- Modify: `config/loader_test.go`

- [ ] **Step 1: Write the failing test**

Append to `config/loader_test.go`:

```go
func TestLoadFromPath_BedrockProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := []byte(`
model: bedrock/anthropic.claude-opus-4-v1:0
providers:
  bedrock:
    provider: bedrock
    api_key: ""
    model: anthropic.claude-opus-4-v1:0
    region: us-west-2
    profile: dev
    access_key_id: AKIA
    secret_access_key: secret
    session_token: tok
agent:
  max_turns: 10
terminal:
  backend: local
storage:
  driver: sqlite
`)
	if err := os.WriteFile(path, yaml, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Providers["bedrock"]
	if p.Region != "us-west-2" {
		t.Errorf("region = %q", p.Region)
	}
	if p.Profile != "dev" {
		t.Errorf("profile = %q", p.Profile)
	}
	if p.AccessKeyID != "AKIA" || p.SecretAccessKey != "secret" {
		t.Errorf("static creds = %q/%q", p.AccessKeyID, p.SecretAccessKey)
	}
	if p.SessionToken != "tok" {
		t.Errorf("session token = %q", p.SessionToken)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestLoadFromPath_BedrockProvider -v`
Expected: FAIL with `p.Region undefined` (or similar).

- [ ] **Step 3: Add fields to ProviderConfig**

In `config/config.go`, replace the `ProviderConfig` struct with:

```go
// ProviderConfig holds settings for a single LLM provider.
// Most providers use only Provider+BaseURL+APIKey+Model. Bedrock-specific
// fields (Region, Profile, AccessKeyID, SecretAccessKey, SessionToken)
// are ignored by providers that don't consume them.
type ProviderConfig struct {
	Provider string `yaml:"provider"`
	BaseURL  string `yaml:"base_url,omitempty"`
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model"`

	// Bedrock-only overrides. When Region is empty the provider falls back
	// to the standard AWS resolution chain (AWS_REGION env var or the
	// active shared-config profile). Static credentials are optional —
	// leave them empty to use the default credential chain (IAM roles,
	// SSO, instance metadata, env vars, shared config).
	Region          string `yaml:"region,omitempty"`
	Profile         string `yaml:"profile,omitempty"`
	AccessKeyID     string `yaml:"access_key_id,omitempty"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty"`
	SessionToken   string `yaml:"session_token,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -run TestLoadFromPath_BedrockProvider -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add config/config.go config/loader_test.go
git commit -m "feat(config): add Bedrock fields (region, profile, static creds) to ProviderConfig"
```

---

## Task 3: Bedrock constructor

**Files:**
- Create: `provider/bedrock/bedrock.go`

- [ ] **Step 1: Write the failing test**

Create `provider/bedrock/bedrock_test.go`:

```go
package bedrock

import (
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestNew_RequiresRegion(t *testing.T) {
	_, err := New(config.ProviderConfig{
		Provider: "bedrock",
		Model:    "anthropic.claude-opus-4-v1:0",
	})
	if err == nil {
		t.Fatal("expected error when region is empty and AWS_REGION unset")
	}
}

func TestNew_AcceptsRegion(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "examplesecret")
	p, err := New(config.ProviderConfig{
		Provider: "bedrock",
		Model:    "anthropic.claude-opus-4-v1:0",
		Region:   "us-east-1",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "bedrock" {
		t.Errorf("Name = %q", p.Name())
	}
	if !p.Available() {
		t.Error("Available should be true when creds are resolvable")
	}
}

func TestEstimateTokens_CharHeuristic(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "examplesecret")
	p, err := New(config.ProviderConfig{
		Provider: "bedrock",
		Model:    "anthropic.claude-opus-4-v1:0",
		Region:   "us-east-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	n, _ := p.EstimateTokens("anthropic.claude-opus-4-v1:0", "hello world") // 11 chars
	if n < 2 || n > 4 {
		t.Errorf("expected ~3 tokens, got %d", n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/bedrock/ -run TestNew -v`
Expected: FAIL — package does not yet exist.

- [ ] **Step 3: Implement the constructor**

Create `provider/bedrock/bedrock.go`:

```go
// Package bedrock implements provider.Provider on top of the AWS
// Bedrock Converse API, giving hermind access to every model family
// Bedrock hosts (Claude, Llama, Mistral, Titan, etc.) through a single
// adapter.
package bedrock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
)

const (
	defaultRequestTimeout = 300 * time.Second
)

// Bedrock is the provider.Provider implementation for AWS Bedrock models.
type Bedrock struct {
	client *bedrockruntime.Client
	model  string
	region string
}

// New constructs a Bedrock provider from config. Region must be set
// (either in cfg.Region or via AWS_REGION / profile). Credentials come
// from the standard AWS chain unless cfg.AccessKeyID is non-empty, in
// which case the static credential trio is used verbatim.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	region := cfg.Region
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		return nil, errors.New("bedrock: region is required (set config.providers.bedrock.region or AWS_REGION)")
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if cfg.Profile != "" {
		loadOpts = append(loadOpts, awsconfig.WithSharedConfigProfile(cfg.Profile))
	}
	if cfg.AccessKeyID != "" {
		loadOpts = append(loadOpts,
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken,
			)),
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("bedrock: load aws config: %w", err)
	}

	return &Bedrock{
		client: bedrockruntime.NewFromConfig(awsCfg),
		model:  cfg.Model,
		region: region,
	}, nil
}

// Name returns "bedrock".
func (b *Bedrock) Name() string { return "bedrock" }

// Available returns true when a client is constructed. The AWS SDK
// defers credential resolution until the first request, so we treat
// "has client" as "available"; request-time failures surface as
// provider.Error instead.
func (b *Bedrock) Available() bool { return b.client != nil }

// ModelInfo returns capabilities for known Bedrock-hosted models.
// Unknown models fall back to a conservative default that still
// enables tools and streaming — Converse supports both universally.
func (b *Bedrock) ModelInfo(model string) *provider.ModelInfo {
	// Minimal bootstrap table. A richer plan will add per-model caps.
	switch {
	case hasPrefix(model, "anthropic.claude-opus-4", "anthropic.claude-sonnet-4", "us.anthropic.claude-opus-4", "global.anthropic.claude-opus-4"):
		return &provider.ModelInfo{
			ContextLength:     200_000,
			MaxOutputTokens:   8_192,
			SupportsVision:    true,
			SupportsTools:     true,
			SupportsStreaming: true,
			SupportsCaching:   true,
			SupportsReasoning: false,
		}
	case hasPrefix(model, "anthropic.claude-3", "us.anthropic.claude-3"):
		return &provider.ModelInfo{
			ContextLength:     200_000,
			MaxOutputTokens:   4_096,
			SupportsVision:    true,
			SupportsTools:     true,
			SupportsStreaming: true,
			SupportsCaching:   false,
			SupportsReasoning: false,
		}
	default:
		return &provider.ModelInfo{
			ContextLength:     128_000,
			MaxOutputTokens:   4_096,
			SupportsVision:    false,
			SupportsTools:     true,
			SupportsStreaming: true,
			SupportsCaching:   false,
			SupportsReasoning: false,
		}
	}
}

// EstimateTokens uses a simple ~4-chars-per-token heuristic, matching
// the other provider bootstrap implementations. A future plan can swap
// in a per-family tokenizer.
func (b *Bedrock) EstimateTokens(model, text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	return (len(text) + 3) / 4, nil
}

func hasPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if len(s) >= len(p) && s[:len(p)] == p {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/bedrock/ -run "TestNew|TestEstimate" -v`
Expected: PASS (3 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add provider/bedrock/bedrock.go provider/bedrock/bedrock_test.go
git commit -m "feat(provider/bedrock): add constructor + ModelInfo scaffolding"
```

---

## Task 4: Request conversion (provider.Request → Converse input)

**Files:**
- Create: `provider/bedrock/convert.go`

- [ ] **Step 1: Write the failing test**

Append to `provider/bedrock/bedrock_test.go`:

```go
import (
	// add alongside existing imports
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

func TestBuildConverseInput_TextOnly(t *testing.T) {
	req := &provider.Request{
		Model:        "anthropic.claude-opus-4-v1:0",
		SystemPrompt: "You are helpful.",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("hello")},
		},
		MaxTokens: 1024,
	}
	in := buildConverseInput(req)
	if got := *in.ModelId; got != "anthropic.claude-opus-4-v1:0" {
		t.Errorf("ModelId = %q", got)
	}
	if len(in.System) != 1 {
		t.Fatalf("System len = %d", len(in.System))
	}
	sys, ok := in.System[0].(*types.SystemContentBlockMemberText)
	if !ok || sys.Value != "You are helpful." {
		t.Errorf("system = %#v", in.System[0])
	}
	if len(in.Messages) != 1 || in.Messages[0].Role != types.ConversationRoleUser {
		t.Errorf("messages = %#v", in.Messages)
	}
}

func TestBuildConverseInput_WithToolUse(t *testing.T) {
	req := &provider.Request{
		Model: "anthropic.claude-opus-4-v1:0",
		Messages: []message.Message{
			{
				Role: message.RoleAssistant,
				Content: message.BlockContent([]message.ContentBlock{
					{Type: "tool_use", ToolUseID: "t1", ToolUseName: "shell", ToolUseInput: []byte(`{"cmd":"ls"}`)},
				}),
			},
			{
				Role: message.RoleUser,
				Content: message.BlockContent([]message.ContentBlock{
					{Type: "tool_result", ToolUseID: "t1", ToolResult: "total 0"},
				}),
			},
		},
		MaxTokens: 512,
	}
	in := buildConverseInput(req)
	if len(in.Messages) != 2 {
		t.Fatalf("messages len = %d", len(in.Messages))
	}
	// assistant tool_use
	tu, ok := in.Messages[0].Content[0].(*types.ContentBlockMemberToolUse)
	if !ok || *tu.Value.ToolUseId != "t1" || *tu.Value.Name != "shell" {
		t.Errorf("tool_use = %#v", in.Messages[0].Content[0])
	}
	// user tool_result
	tr, ok := in.Messages[1].Content[0].(*types.ContentBlockMemberToolResult)
	if !ok || *tr.Value.ToolUseId != "t1" {
		t.Errorf("tool_result = %#v", in.Messages[1].Content[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/bedrock/ -run TestBuildConverseInput -v`
Expected: FAIL — `buildConverseInput` undefined.

- [ ] **Step 3: Implement the converter**

Create `provider/bedrock/convert.go`:

```go
package bedrock

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	smithydoc "github.com/aws/smithy-go/document"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// buildConverseInput shapes a provider.Request into a ConverseInput.
// Streaming and non-streaming use the same input type; the caller
// picks between client.Converse and client.ConverseStream.
func buildConverseInput(req *provider.Request) *bedrockruntime.ConverseInput {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	in := &bedrockruntime.ConverseInput{
		ModelId:  aws.String(req.Model),
		Messages: make([]types.Message, 0, len(req.Messages)),
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens: aws.Int32(int32(maxTokens)),
		},
	}
	if req.Temperature != nil {
		tmp := float32(*req.Temperature)
		in.InferenceConfig.Temperature = &tmp
	}
	if req.TopP != nil {
		tp := float32(*req.TopP)
		in.InferenceConfig.TopP = &tp
	}
	if len(req.StopSequences) > 0 {
		in.InferenceConfig.StopSequences = req.StopSequences
	}
	if req.SystemPrompt != "" {
		in.System = []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{Value: req.SystemPrompt},
		}
	}
	if len(req.Tools) > 0 {
		tools := make([]types.Tool, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, &types.ToolMemberToolSpec{
				Value: types.ToolSpecification{
					Name:        aws.String(t.Function.Name),
					Description: aws.String(t.Function.Description),
					InputSchema: &types.ToolInputSchemaMemberJson{
						Value: rawJSONDoc(t.Function.Parameters),
					},
				},
			})
		}
		in.ToolConfig = &types.ToolConfiguration{Tools: tools}
	}

	for _, m := range req.Messages {
		role := types.ConversationRoleUser
		if m.Role == message.RoleAssistant {
			role = types.ConversationRoleAssistant
		}
		in.Messages = append(in.Messages, types.Message{
			Role:    role,
			Content: contentToBedrockBlocks(m.Content),
		})
	}
	return in
}

func contentToBedrockBlocks(c message.Content) []types.ContentBlock {
	if c.IsText() {
		return []types.ContentBlock{
			&types.ContentBlockMemberText{Value: c.Text()},
		}
	}
	out := make([]types.ContentBlock, 0, len(c.Blocks()))
	for _, b := range c.Blocks() {
		switch b.Type {
		case "text":
			out = append(out, &types.ContentBlockMemberText{Value: b.Text})
		case "tool_use":
			out = append(out, &types.ContentBlockMemberToolUse{
				Value: types.ToolUseBlock{
					ToolUseId: aws.String(b.ToolUseID),
					Name:      aws.String(b.ToolUseName),
					Input:     rawJSONDoc(b.ToolUseInput),
				},
			})
		case "tool_result":
			out = append(out, &types.ContentBlockMemberToolResult{
				Value: types.ToolResultBlock{
					ToolUseId: aws.String(b.ToolUseID),
					Content: []types.ToolResultContentBlock{
						&types.ToolResultContentBlockMemberText{Value: b.ToolResult},
					},
				},
			})
		}
	}
	return out
}

// rawJSONDoc wraps a json.RawMessage in the Smithy document wrapper
// Bedrock's Converse API expects. A nil/empty payload becomes `{}`.
func rawJSONDoc(raw json.RawMessage) smithydoc.Interface {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	return &rawDoc{raw: raw}
}

// rawDoc implements smithydoc.Interface by passing the JSON through.
// The Smithy runtime calls MarshalSmithyDocument to serialize.
type rawDoc struct{ raw json.RawMessage }

func (d *rawDoc) UnmarshalSmithyDocument(out interface{}) error {
	return json.Unmarshal(d.raw, out)
}

func (d *rawDoc) MarshalSmithyDocument() ([]byte, error) {
	return d.raw, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/bedrock/ -run TestBuildConverseInput -v`
Expected: PASS (both sub-tests).

- [ ] **Step 5: Commit**

```bash
git add provider/bedrock/convert.go provider/bedrock/bedrock_test.go
git commit -m "feat(provider/bedrock): request conversion (provider.Request -> Converse)"
```

---

## Task 5: Response conversion + Complete

**Files:**
- Modify: `provider/bedrock/convert.go` (add `convertConverseOutput`)
- Create: `provider/bedrock/complete.go`

- [ ] **Step 1: Write the failing test**

Append to `provider/bedrock/bedrock_test.go`:

```go
func TestConvertConverseOutput_TextAndUsage(t *testing.T) {
	out := &bedrockruntime.ConverseOutput{
		Output: &types.ConverseOutputMemberMessage{
			Value: types.Message{
				Role: types.ConversationRoleAssistant,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{Value: "Hello back"},
				},
			},
		},
		StopReason: types.StopReasonEndTurn,
		Usage: &types.TokenUsage{
			InputTokens:  aws.Int32(10),
			OutputTokens: aws.Int32(5),
			TotalTokens:  aws.Int32(15),
		},
	}
	resp := convertConverseOutput(out, "anthropic.claude-opus-4-v1:0")
	if resp.Message.Role != message.RoleAssistant {
		t.Errorf("role = %v", resp.Message.Role)
	}
	if resp.Message.Content.Text() != "Hello back" {
		t.Errorf("text = %q", resp.Message.Content.Text())
	}
	if resp.Usage.InputTokens != 10 || resp.Usage.OutputTokens != 5 {
		t.Errorf("usage = %+v", resp.Usage)
	}
	if resp.FinishReason != "end_turn" {
		t.Errorf("finish = %q", resp.FinishReason)
	}
}

func TestConvertConverseOutput_ToolUse(t *testing.T) {
	out := &bedrockruntime.ConverseOutput{
		Output: &types.ConverseOutputMemberMessage{
			Value: types.Message{
				Role: types.ConversationRoleAssistant,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{Value: "calling tool"},
					&types.ContentBlockMemberToolUse{
						Value: types.ToolUseBlock{
							ToolUseId: aws.String("t42"),
							Name:      aws.String("shell"),
							Input:     rawJSONDoc([]byte(`{"cmd":"ls"}`)),
						},
					},
				},
			},
		},
		StopReason: types.StopReasonToolUse,
		Usage:      &types.TokenUsage{InputTokens: aws.Int32(1), OutputTokens: aws.Int32(1)},
	}
	resp := convertConverseOutput(out, "anthropic.claude-opus-4-v1:0")
	blocks := resp.Message.Content.Blocks()
	if len(blocks) != 2 {
		t.Fatalf("blocks = %d", len(blocks))
	}
	if blocks[1].Type != "tool_use" || blocks[1].ToolUseID != "t42" {
		t.Errorf("tool_use = %+v", blocks[1])
	}
	if string(blocks[1].ToolUseInput) != `{"cmd":"ls"}` {
		t.Errorf("input = %s", blocks[1].ToolUseInput)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/bedrock/ -run TestConvertConverseOutput -v`
Expected: FAIL — `convertConverseOutput` undefined.

- [ ] **Step 3: Add convertConverseOutput to convert.go**

Append to `provider/bedrock/convert.go`:

```go
func convertConverseOutput(out *bedrockruntime.ConverseOutput, model string) *provider.Response {
	msg, _ := out.Output.(*types.ConverseOutputMemberMessage)
	var text string
	var blocks []message.ContentBlock
	hasTool := false
	if msg != nil {
		for _, c := range msg.Value.Content {
			switch v := c.(type) {
			case *types.ContentBlockMemberText:
				blocks = append(blocks, message.ContentBlock{Type: "text", Text: v.Value})
				text += v.Value
			case *types.ContentBlockMemberToolUse:
				hasTool = true
				raw, _ := v.Value.Input.MarshalSmithyDocument()
				blocks = append(blocks, message.ContentBlock{
					Type:         "tool_use",
					ToolUseID:    aws.ToString(v.Value.ToolUseId),
					ToolUseName:  aws.ToString(v.Value.Name),
					ToolUseInput: raw,
				})
			}
		}
	}

	var content message.Content
	if hasTool {
		content = message.BlockContent(blocks)
	} else {
		content = message.TextContent(text)
	}

	usage := message.Usage{}
	if out.Usage != nil {
		if out.Usage.InputTokens != nil {
			usage.InputTokens = int(*out.Usage.InputTokens)
		}
		if out.Usage.OutputTokens != nil {
			usage.OutputTokens = int(*out.Usage.OutputTokens)
		}
	}

	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: content,
		},
		FinishReason: stopReasonToString(out.StopReason),
		Usage:        usage,
		Model:        model,
	}
}

func stopReasonToString(r types.StopReason) string {
	switch r {
	case types.StopReasonEndTurn:
		return "end_turn"
	case types.StopReasonToolUse:
		return "tool_use"
	case types.StopReasonMaxTokens:
		return "max_tokens"
	case types.StopReasonStopSequence:
		return "stop_sequence"
	default:
		return string(r)
	}
}
```

- [ ] **Step 4: Implement Complete**

Create `provider/bedrock/complete.go`:

```go
package bedrock

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/provider"
)

// Complete sends a non-streaming Converse request.
func (b *Bedrock) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	in := buildConverseInput(req)
	out, err := b.client.Converse(ctx, in)
	if err != nil {
		return nil, mapAWSError(err)
	}
	if out == nil || out.Output == nil {
		return nil, fmt.Errorf("bedrock: empty response")
	}
	return convertConverseOutput(out, req.Model), nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./provider/bedrock/ -run TestConvertConverseOutput -v`
Expected: PASS (both sub-tests).

Note: `mapAWSError` is defined in Task 6 — the `Complete` file currently references it but is not exercised in this task's tests, so compilation is fine only if we stub it. Add a temporary stub at the top of `complete.go` **only if** `go build ./provider/bedrock/` fails; otherwise move on.

If stub is needed, prepend to `complete.go`:

```go
// Temporary stub — real impl in errors.go (Task 6).
var mapAWSError = func(err error) error { return err }
```

And delete it when Task 6 lands.

- [ ] **Step 6: Commit**

```bash
git add provider/bedrock/convert.go provider/bedrock/complete.go provider/bedrock/bedrock_test.go
git commit -m "feat(provider/bedrock): Complete() via Converse + response conversion"
```

---

## Task 6: AWS error mapping

**Files:**
- Create: `provider/bedrock/errors.go`

- [ ] **Step 1: Write the failing test**

Append to `provider/bedrock/bedrock_test.go`:

```go
import (
	// alongside others
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

func TestMapAWSError_ThrottlingIsRateLimit(t *testing.T) {
	src := &types.ThrottlingException{Message: aws.String("slow down")}
	mapped := mapAWSError(src)
	var pErr *provider.Error
	if !errors.As(mapped, &pErr) {
		t.Fatalf("expected provider.Error, got %T", mapped)
	}
	if pErr.Kind != provider.ErrRateLimit {
		t.Errorf("kind = %v", pErr.Kind)
	}
	if pErr.Provider != "bedrock" {
		t.Errorf("provider = %q", pErr.Provider)
	}
}

func TestMapAWSError_AccessDenied(t *testing.T) {
	src := &types.AccessDeniedException{Message: aws.String("nope")}
	mapped := mapAWSError(src)
	var pErr *provider.Error
	errors.As(mapped, &pErr)
	if pErr.Kind != provider.ErrAuth {
		t.Errorf("kind = %v", pErr.Kind)
	}
}

func TestMapAWSError_Validation(t *testing.T) {
	src := &types.ValidationException{Message: aws.String("bad input")}
	mapped := mapAWSError(src)
	var pErr *provider.Error
	errors.As(mapped, &pErr)
	if pErr.Kind != provider.ErrInvalidRequest {
		t.Errorf("kind = %v", pErr.Kind)
	}
}

func TestMapAWSError_Internal(t *testing.T) {
	src := &types.InternalServerException{Message: aws.String("500")}
	mapped := mapAWSError(src)
	var pErr *provider.Error
	errors.As(mapped, &pErr)
	if pErr.Kind != provider.ErrServerError {
		t.Errorf("kind = %v", pErr.Kind)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/bedrock/ -run TestMapAWSError -v`
Expected: FAIL — the stub from Task 5 returns the raw error so `errors.As(&provider.Error)` is false.

- [ ] **Step 3: Implement mapAWSError**

Create `provider/bedrock/errors.go` (and remove the Task-5 stub from `complete.go` if you added it):

```go
package bedrock

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/odysseythink/hermind/provider"
)

// mapAWSError translates a bedrockruntime modeled error into the shared
// provider.Error taxonomy. Unknown errors become ErrUnknown so the
// caller can still display them without treating them as retryable.
func mapAWSError(err error) error {
	if err == nil {
		return nil
	}
	kind := provider.ErrUnknown

	var (
		throttling      *types.ThrottlingException
		accessDenied    *types.AccessDeniedException
		validation      *types.ValidationException
		serviceQuota    *types.ServiceQuotaExceededException
		internalServer  *types.InternalServerException
		modelTimeout    *types.ModelTimeoutException
		resourceNotFound *types.ResourceNotFoundException
	)

	switch {
	case errors.As(err, &throttling), errors.As(err, &serviceQuota):
		kind = provider.ErrRateLimit
	case errors.As(err, &accessDenied):
		kind = provider.ErrAuth
	case errors.As(err, &validation), errors.As(err, &resourceNotFound):
		kind = provider.ErrInvalidRequest
	case errors.As(err, &internalServer):
		kind = provider.ErrServerError
	case errors.As(err, &modelTimeout):
		kind = provider.ErrTimeout
	}

	return &provider.Error{
		Kind:     kind,
		Provider: "bedrock",
		Message:  fmt.Sprintf("aws: %v", err),
		Cause:    err,
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/bedrock/ -run TestMapAWSError -v`
Expected: PASS (all 4 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add provider/bedrock/errors.go provider/bedrock/complete.go
git commit -m "feat(provider/bedrock): AWS error taxonomy mapping"
```

---

## Task 7: Streaming via ConverseStream

**Files:**
- Create: `provider/bedrock/stream.go`

- [ ] **Step 1: Write the failing test**

Append to `provider/bedrock/bedrock_test.go`:

```go
func TestStreamEventFromChunk_TextDelta(t *testing.T) {
	ev := streamEventFromChunk(&types.ConverseStreamOutputMemberContentBlockDelta{
		Value: types.ContentBlockDeltaEvent{
			Delta: &types.ContentBlockDeltaMemberText{Value: "hel"},
		},
	})
	if ev.Type != provider.EventDelta || ev.Delta == nil || ev.Delta.Content != "hel" {
		t.Errorf("got %#v", ev)
	}
}

func TestStreamEventFromChunk_MessageStop(t *testing.T) {
	ev := streamEventFromChunk(&types.ConverseStreamOutputMemberMessageStop{
		Value: types.MessageStopEvent{StopReason: types.StopReasonEndTurn},
	})
	if ev.Type != provider.EventDone {
		t.Errorf("type = %v", ev.Type)
	}
	if ev.Response == nil || ev.Response.FinishReason != "end_turn" {
		t.Errorf("response = %#v", ev.Response)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/bedrock/ -run TestStreamEventFromChunk -v`
Expected: FAIL — `streamEventFromChunk` undefined.

- [ ] **Step 3: Implement streaming**

Create `provider/bedrock/stream.go`:

```go
package bedrock

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// Stream sends a ConverseStream request and returns a Stream wrapper
// over the event stream reader.
func (b *Bedrock) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	in := buildConverseInput(req)
	out, err := b.client.ConverseStream(ctx, &bedrockruntime.ConverseStreamInput{
		ModelId:         in.ModelId,
		Messages:        in.Messages,
		System:          in.System,
		InferenceConfig: in.InferenceConfig,
		ToolConfig:      in.ToolConfig,
	})
	if err != nil {
		return nil, mapAWSError(err)
	}
	return &stream{
		reader: out.GetStream(),
		model:  req.Model,
	}, nil
}

type stream struct {
	reader interface {
		Events() <-chan types.ConverseStreamOutput
		Close() error
		Err() error
	}
	model string
}

func (s *stream) Recv() (*provider.StreamEvent, error) {
	ev, ok := <-s.reader.Events()
	if !ok {
		if err := s.reader.Err(); err != nil {
			return nil, mapAWSError(err)
		}
		return &provider.StreamEvent{Type: provider.EventDone}, io.EOF
	}
	return streamEventFromChunk(ev), nil
}

func (s *stream) Close() error { return s.reader.Close() }

// streamEventFromChunk converts one Converse stream event into the
// provider-level StreamEvent. Unsupported events map to an EventDelta
// with no content so the caller can keep iterating.
func streamEventFromChunk(chunk types.ConverseStreamOutput) *provider.StreamEvent {
	switch c := chunk.(type) {
	case *types.ConverseStreamOutputMemberContentBlockDelta:
		switch d := c.Value.Delta.(type) {
		case *types.ContentBlockDeltaMemberText:
			return &provider.StreamEvent{
				Type:  provider.EventDelta,
				Delta: &provider.StreamDelta{Content: d.Value},
			}
		case *types.ContentBlockDeltaMemberToolUse:
			// Tool-use deltas carry partial JSON; forward as-is.
			return &provider.StreamEvent{
				Type: provider.EventDelta,
				Delta: &provider.StreamDelta{
					ToolCalls: []message.ToolCall{{
						ID:        "", // Bedrock emits ID in ContentBlockStart
						Name:      "",
						Arguments: d.Value.Input,
					}},
				},
			}
		}
	case *types.ConverseStreamOutputMemberMessageStop:
		return &provider.StreamEvent{
			Type: provider.EventDone,
			Response: &provider.Response{
				FinishReason: stopReasonToString(c.Value.StopReason),
			},
		}
	}
	return &provider.StreamEvent{Type: provider.EventDelta, Delta: &provider.StreamDelta{}}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/bedrock/ -run TestStreamEventFromChunk -v`
Expected: PASS (both sub-tests).

- [ ] **Step 5: Commit**

```bash
git add provider/bedrock/stream.go provider/bedrock/bedrock_test.go
git commit -m "feat(provider/bedrock): streaming via ConverseStream"
```

---

## Task 8: Register in factory

**Files:**
- Modify: `provider/factory/factory.go`
- Modify: `provider/factory/factory_test.go`

- [ ] **Step 1: Write the failing test**

Append to `provider/factory/factory_test.go`:

```go
func TestFactory_Bedrock(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	p, err := New(config.ProviderConfig{
		Provider: "bedrock",
		Model:    "anthropic.claude-opus-4-v1:0",
		Region:   "us-east-1",
	})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if p.Name() != "bedrock" {
		t.Errorf("name = %q", p.Name())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/factory/ -run TestFactory_Bedrock -v`
Expected: FAIL — `unknown provider "bedrock"`.

- [ ] **Step 3: Register the new branch**

In `provider/factory/factory.go`:

```go
import (
	// add alongside existing provider imports
	"github.com/odysseythink/hermind/provider/bedrock"
)
```

And inside `New()`'s switch statement, add:

```go
	case "bedrock":
		return bedrock.New(cfg)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/factory/ -run TestFactory_Bedrock -v`
Expected: PASS.

- [ ] **Step 5: Full suite**

Run: `go test ./...`
Expected: PASS everywhere.

- [ ] **Step 6: Commit**

```bash
git add provider/factory/factory.go provider/factory/factory_test.go
git commit -m "feat(provider/factory): register bedrock"
```

---

## Task 9: End-to-end smoke test (manual)

**Files:** none — manual verification.

- [ ] **Step 1: Configure a local AWS profile** (only if you have real Bedrock access)

```bash
aws configure --profile hermind-dev
# enter keys + region us-west-2
aws bedrock list-foundation-models --profile hermind-dev --region us-west-2 | head
# expect a non-empty list
```

- [ ] **Step 2: Add bedrock to your hermind config**

```bash
cat >> ~/.hermind/config.yaml <<'EOF'
providers:
  bedrock:
    provider: bedrock
    api_key: ""
    model: anthropic.claude-opus-4-v1:0
    region: us-west-2
    profile: hermind-dev
EOF
```

- [ ] **Step 3: Run a one-shot prompt**

```bash
go run ./cmd/hermind run --model bedrock/anthropic.claude-opus-4-v1:0 <<'EOF'
Say hello in one word.
EOF
```

Expected: a single-word response streamed from Bedrock.

If you do not have Bedrock access, skip Task 9 entirely — the automated tests cover the wire conversion, which is the risky part.

- [ ] **Step 4: Final commit** (optional, nothing to commit if only manual)

```bash
git commit --allow-empty -m "test(provider/bedrock): manual smoke test with live AWS account"
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - Region + default credential chain ↔ Task 3 (`awsconfig.LoadDefaultConfig`) ✓
   - Static creds override ↔ Task 3 (`credentials.NewStaticCredentialsProvider`) ✓
   - Cross-region inference profile IDs (`us.*`, `global.*`) ↔ Task 3 (`ModelInfo` prefix match) ✓
   - Tool calls ↔ Task 4 + Task 5 (tool_use / tool_result block conversion) ✓
   - Streaming ↔ Task 7 (ConverseStream + chunk mapping) ✓
   - Error taxonomy ↔ Task 6 ✓
   - Factory registration ↔ Task 8 ✓

2. **Placeholders:** none. The only conditional stub (Task 5 Step 5) is explicitly time-bounded and removed in Task 6 Step 3.

3. **Type consistency:**
   - `buildConverseInput` signature stable across Task 4 → Task 7.
   - `convertConverseOutput(out, model)` signature stable across Task 5 test and Task 6 usage.
   - `mapAWSError` signature stable across Tasks 5, 6, 7.
   - `rawJSONDoc` used in both Task 4 and Task 5 tests with the same shape.

4. **Gaps:**
   - No tokenizer (intentionally uses 4-char heuristic, matches Anthropic provider).
   - No Guardrails wiring (out of MVP scope).
   - No model discovery via `bedrock.ListFoundationModels` — intentionally deferred.

---

## Definition of Done

- `go test ./provider/bedrock/... ./provider/factory/... ./config/...` all pass.
- `go build ./...` succeeds.
- `hermind run --model bedrock/<id>` routes through the new provider (verified manually if AWS access is available).
- Config docs mention `region` / `profile` / static cred fields.
