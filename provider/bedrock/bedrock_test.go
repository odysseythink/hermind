package bedrock

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
)

// ----- New / metadata -----

func TestNew_AcceptsAWSRegionFromEnv(t *testing.T) {
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "examplesecret")
	// Force the SDK to skip shared config lookup so the test doesn't
	// depend on whatever is in the developer's ~/.aws/config.
	t.Setenv("AWS_CONFIG_FILE", "/dev/null")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")
	t.Setenv("AWS_PROFILE", "")

	p, err := New(config.ProviderConfig{
		Provider: "bedrock",
		Model:    "anthropic.claude-opus-4-v1:0",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "bedrock" {
		t.Errorf("Name = %q, want bedrock", p.Name())
	}
	if !p.Available() {
		t.Error("Available should be true once the client is constructed")
	}
}

func TestNew_ErrorWhenRegionMissing(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_CONFIG_FILE", "/dev/null")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")

	_, err := New(config.ProviderConfig{
		Provider: "bedrock",
		Model:    "anthropic.claude-opus-4-v1:0",
	})
	if err == nil {
		t.Fatal("expected error when region cannot be resolved")
	}
}

func TestModelInfo_Claude4(t *testing.T) {
	b := &Bedrock{}
	info := b.ModelInfo("anthropic.claude-opus-4-v1:0")
	if info == nil || info.MaxOutputTokens != 8192 || !info.SupportsCaching {
		t.Errorf("opus-4 info = %+v", info)
	}

	info = b.ModelInfo("us.anthropic.claude-sonnet-4-20250514-v1:0")
	if info == nil || !info.SupportsVision {
		t.Errorf("us. prefix not matched: %+v", info)
	}
}

func TestModelInfo_Claude3(t *testing.T) {
	b := &Bedrock{}
	info := b.ModelInfo("anthropic.claude-3-5-sonnet-20240620-v1:0")
	if info == nil || info.MaxOutputTokens != 4096 {
		t.Errorf("claude-3 info = %+v", info)
	}
}

func TestModelInfo_UnknownFallback(t *testing.T) {
	b := &Bedrock{}
	info := b.ModelInfo("meta.llama3-70b-instruct-v1:0")
	if info == nil || info.SupportsTools == false {
		t.Errorf("unknown model info = %+v", info)
	}
}

func TestEstimateTokens_CharHeuristic(t *testing.T) {
	b := &Bedrock{}
	n, err := b.EstimateTokens("any", "hello world") // 11 chars -> ceil(11/4) = 3
	if err != nil {
		t.Fatalf("EstimateTokens: %v", err)
	}
	if n != 3 {
		t.Errorf("n = %d, want 3", n)
	}

	n, _ = b.EstimateTokens("any", "")
	if n != 0 {
		t.Errorf("empty: n = %d, want 0", n)
	}
}

// ----- buildConverseInput -----

func TestBuildConverseInput_TextOnly(t *testing.T) {
	temp := 0.3
	topP := 0.95
	req := &provider.Request{
		Model:        "anthropic.claude-opus-4-v1:0",
		SystemPrompt: "You are helpful.",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("hello")},
			{Role: message.RoleAssistant, Content: message.TextContent("hi there")},
		},
		MaxTokens:     1024,
		Temperature:   &temp,
		TopP:          &topP,
		StopSequences: []string{"\n\n"},
	}
	in := buildConverseInput(req)

	if got := aws.ToString(in.ModelId); got != "anthropic.claude-opus-4-v1:0" {
		t.Errorf("ModelId = %q", got)
	}
	if len(in.System) != 1 {
		t.Fatalf("System len = %d", len(in.System))
	}
	sys, ok := in.System[0].(*types.SystemContentBlockMemberText)
	if !ok || sys.Value != "You are helpful." {
		t.Errorf("system = %#v", in.System[0])
	}
	if len(in.Messages) != 2 {
		t.Fatalf("messages = %d", len(in.Messages))
	}
	if in.Messages[0].Role != types.ConversationRoleUser {
		t.Errorf("msg0 role = %v", in.Messages[0].Role)
	}
	if in.Messages[1].Role != types.ConversationRoleAssistant {
		t.Errorf("msg1 role = %v", in.Messages[1].Role)
	}
	if in.InferenceConfig == nil || aws.ToInt32(in.InferenceConfig.MaxTokens) != 1024 {
		t.Errorf("maxTokens = %+v", in.InferenceConfig)
	}
	if in.InferenceConfig.Temperature == nil || *in.InferenceConfig.Temperature < 0.29 || *in.InferenceConfig.Temperature > 0.31 {
		t.Errorf("temperature = %v", in.InferenceConfig.Temperature)
	}
	if in.InferenceConfig.TopP == nil || *in.InferenceConfig.TopP < 0.94 || *in.InferenceConfig.TopP > 0.96 {
		t.Errorf("topP = %v", in.InferenceConfig.TopP)
	}
	if len(in.InferenceConfig.StopSequences) != 1 || in.InferenceConfig.StopSequences[0] != "\n\n" {
		t.Errorf("stop = %+v", in.InferenceConfig.StopSequences)
	}
}

func TestBuildConverseInput_DefaultMaxTokens(t *testing.T) {
	req := &provider.Request{
		Model:    "anthropic.claude-opus-4-v1:0",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	}
	in := buildConverseInput(req)
	if aws.ToInt32(in.InferenceConfig.MaxTokens) != 4096 {
		t.Errorf("default maxTokens = %d", aws.ToInt32(in.InferenceConfig.MaxTokens))
	}
}

func TestBuildConverseInput_Tools(t *testing.T) {
	req := &provider.Request{
		Model: "anthropic.claude-opus-4-v1:0",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("run it")},
		},
		Tools: []tool.ToolDefinition{
			{
				Type: "function",
				Function: tool.FunctionDef{
					Name:        "shell",
					Description: "run a shell command",
					Parameters:  json.RawMessage(`{"type":"object","properties":{"cmd":{"type":"string"}}}`),
				},
			},
		},
	}
	in := buildConverseInput(req)
	if in.ToolConfig == nil || len(in.ToolConfig.Tools) != 1 {
		t.Fatalf("tool config = %+v", in.ToolConfig)
	}
	spec, ok := in.ToolConfig.Tools[0].(*types.ToolMemberToolSpec)
	if !ok {
		t.Fatalf("tool kind = %T", in.ToolConfig.Tools[0])
	}
	if aws.ToString(spec.Value.Name) != "shell" {
		t.Errorf("tool name = %q", aws.ToString(spec.Value.Name))
	}
	schema, ok := spec.Value.InputSchema.(*types.ToolInputSchemaMemberJson)
	if !ok || schema.Value == nil {
		t.Fatalf("schema = %#v", spec.Value.InputSchema)
	}
	raw, err := schema.Value.MarshalSmithyDocument()
	if err != nil {
		t.Fatalf("schema marshal: %v", err)
	}
	// Must round-trip as a JSON object describing the tool params.
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("schema json: %v (%s)", err, raw)
	}
	if decoded["type"] != "object" {
		t.Errorf("schema type = %v", decoded["type"])
	}
}

func TestBuildConverseInput_WithToolUseAndResult(t *testing.T) {
	req := &provider.Request{
		Model: "anthropic.claude-opus-4-v1:0",
		Messages: []message.Message{
			{
				Role: message.RoleAssistant,
				Content: message.BlockContent([]message.ContentBlock{
					{Type: "text", Text: "calling tool"},
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
	}
	in := buildConverseInput(req)
	if len(in.Messages) != 2 {
		t.Fatalf("messages = %d", len(in.Messages))
	}
	// assistant message should have text + tool_use
	if len(in.Messages[0].Content) != 2 {
		t.Fatalf("assistant blocks = %d", len(in.Messages[0].Content))
	}
	tu, ok := in.Messages[0].Content[1].(*types.ContentBlockMemberToolUse)
	if !ok {
		t.Fatalf("block[1] kind = %T", in.Messages[0].Content[1])
	}
	if aws.ToString(tu.Value.ToolUseId) != "t1" || aws.ToString(tu.Value.Name) != "shell" {
		t.Errorf("tool_use = %+v", tu.Value)
	}
	// user message should have tool_result
	tr, ok := in.Messages[1].Content[0].(*types.ContentBlockMemberToolResult)
	if !ok {
		t.Fatalf("block[0] kind = %T", in.Messages[1].Content[0])
	}
	if aws.ToString(tr.Value.ToolUseId) != "t1" {
		t.Errorf("tool_result id = %q", aws.ToString(tr.Value.ToolUseId))
	}
	if len(tr.Value.Content) != 1 {
		t.Fatalf("result content = %d", len(tr.Value.Content))
	}
	txt, ok := tr.Value.Content[0].(*types.ToolResultContentBlockMemberText)
	if !ok || txt.Value != "total 0" {
		t.Errorf("result text = %#v", tr.Value.Content[0])
	}
}

// ----- convertConverseOutput -----

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
	if resp.Model != "anthropic.claude-opus-4-v1:0" {
		t.Errorf("model = %q", resp.Model)
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
							Input:     jsonToDoc([]byte(`{"cmd":"ls"}`)),
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
	if blocks[1].Type != "tool_use" || blocks[1].ToolUseID != "t42" || blocks[1].ToolUseName != "shell" {
		t.Errorf("tool_use = %+v", blocks[1])
	}
	// Tool input must round-trip as JSON equal to the original.
	var got map[string]any
	if err := json.Unmarshal(blocks[1].ToolUseInput, &got); err != nil {
		t.Fatalf("tool input json: %v (%s)", err, blocks[1].ToolUseInput)
	}
	if got["cmd"] != "ls" {
		t.Errorf("input = %+v", got)
	}
	if resp.FinishReason != "tool_use" {
		t.Errorf("finish = %q", resp.FinishReason)
	}
}

// ----- error mapping -----

func TestMapAWSError_Throttling(t *testing.T) {
	src := &types.ThrottlingException{Message: aws.String("slow down")}
	mapped := mapAWSError(src)
	var pErr *provider.Error
	if !stderrors.As(mapped, &pErr) {
		t.Fatalf("expected provider.Error, got %T", mapped)
	}
	if pErr.Kind != provider.ErrRateLimit {
		t.Errorf("kind = %v, want ErrRateLimit", pErr.Kind)
	}
	if pErr.Provider != "bedrock" {
		t.Errorf("provider = %q", pErr.Provider)
	}
}

func TestMapAWSError_AccessDenied(t *testing.T) {
	src := &types.AccessDeniedException{Message: aws.String("nope")}
	mapped := mapAWSError(src)
	var pErr *provider.Error
	_ = stderrors.As(mapped, &pErr)
	if pErr == nil || pErr.Kind != provider.ErrAuth {
		t.Errorf("kind = %v, want ErrAuth", pErr)
	}
}

func TestMapAWSError_Validation(t *testing.T) {
	src := &types.ValidationException{Message: aws.String("bad input")}
	mapped := mapAWSError(src)
	var pErr *provider.Error
	_ = stderrors.As(mapped, &pErr)
	if pErr == nil || pErr.Kind != provider.ErrInvalidRequest {
		t.Errorf("kind = %v, want ErrInvalidRequest", pErr)
	}
}

func TestMapAWSError_InternalServer(t *testing.T) {
	src := &types.InternalServerException{Message: aws.String("500")}
	mapped := mapAWSError(src)
	var pErr *provider.Error
	_ = stderrors.As(mapped, &pErr)
	if pErr == nil || pErr.Kind != provider.ErrServerError {
		t.Errorf("kind = %v, want ErrServerError", pErr)
	}
}

func TestMapAWSError_ModelTimeout(t *testing.T) {
	src := &types.ModelTimeoutException{Message: aws.String("timed out")}
	mapped := mapAWSError(src)
	var pErr *provider.Error
	_ = stderrors.As(mapped, &pErr)
	if pErr == nil || pErr.Kind != provider.ErrTimeout {
		t.Errorf("kind = %v, want ErrTimeout", pErr)
	}
}

func TestMapAWSError_ServiceQuota(t *testing.T) {
	src := &types.ServiceQuotaExceededException{Message: aws.String("quota")}
	mapped := mapAWSError(src)
	var pErr *provider.Error
	_ = stderrors.As(mapped, &pErr)
	if pErr == nil || pErr.Kind != provider.ErrRateLimit {
		t.Errorf("kind = %v, want ErrRateLimit", pErr)
	}
}

func TestMapAWSError_Nil(t *testing.T) {
	if mapAWSError(nil) != nil {
		t.Error("nil input should return nil")
	}
}

func TestMapAWSError_Unknown(t *testing.T) {
	mapped := mapAWSError(stderrors.New("some wire error"))
	var pErr *provider.Error
	_ = stderrors.As(mapped, &pErr)
	if pErr == nil || pErr.Kind != provider.ErrUnknown {
		t.Errorf("kind = %v, want ErrUnknown", pErr)
	}
}

// ----- streaming (pure) -----

func TestStreamEventFromChunk_TextDelta(t *testing.T) {
	ev := streamEventFromChunk(&types.ConverseStreamOutputMemberContentBlockDelta{
		Value: types.ContentBlockDeltaEvent{
			ContentBlockIndex: aws.Int32(0),
			Delta:             &types.ContentBlockDeltaMemberText{Value: "hel"},
		},
	})
	if ev.Type != provider.EventDelta || ev.Delta == nil || ev.Delta.Content != "hel" {
		t.Errorf("got %#v / delta = %#v", ev, ev.Delta)
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

// ----- streaming (stateful) via injected event reader -----

// fakeReader implements eventReader by replaying events from a slice.
type fakeReader struct {
	ch     chan types.ConverseStreamOutput
	err    error
	closed bool
}

func newFakeReader(events []types.ConverseStreamOutput) *fakeReader {
	ch := make(chan types.ConverseStreamOutput, len(events))
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return &fakeReader{ch: ch}
}

func (f *fakeReader) Events() <-chan types.ConverseStreamOutput { return f.ch }
func (f *fakeReader) Err() error                                { return f.err }
func (f *fakeReader) Close() error                              { f.closed = true; return nil }

func TestBedrockStream_TextAndToolUse(t *testing.T) {
	reader := newFakeReader([]types.ConverseStreamOutput{
		&types.ConverseStreamOutputMemberMessageStart{
			Value: types.MessageStartEvent{Role: types.ConversationRoleAssistant},
		},
		&types.ConverseStreamOutputMemberContentBlockDelta{
			Value: types.ContentBlockDeltaEvent{
				ContentBlockIndex: aws.Int32(0),
				Delta:             &types.ContentBlockDeltaMemberText{Value: "hello "},
			},
		},
		&types.ConverseStreamOutputMemberContentBlockDelta{
			Value: types.ContentBlockDeltaEvent{
				ContentBlockIndex: aws.Int32(0),
				Delta:             &types.ContentBlockDeltaMemberText{Value: "world"},
			},
		},
		&types.ConverseStreamOutputMemberContentBlockStart{
			Value: types.ContentBlockStartEvent{
				ContentBlockIndex: aws.Int32(1),
				Start: &types.ContentBlockStartMemberToolUse{
					Value: types.ToolUseBlockStart{
						ToolUseId: aws.String("tu-7"),
						Name:      aws.String("shell"),
					},
				},
			},
		},
		&types.ConverseStreamOutputMemberContentBlockDelta{
			Value: types.ContentBlockDeltaEvent{
				ContentBlockIndex: aws.Int32(1),
				Delta: &types.ContentBlockDeltaMemberToolUse{
					Value: types.ToolUseBlockDelta{Input: aws.String(`{"cmd":`)},
				},
			},
		},
		&types.ConverseStreamOutputMemberContentBlockDelta{
			Value: types.ContentBlockDeltaEvent{
				ContentBlockIndex: aws.Int32(1),
				Delta: &types.ContentBlockDeltaMemberToolUse{
					Value: types.ToolUseBlockDelta{Input: aws.String(`"ls"}`)},
				},
			},
		},
		&types.ConverseStreamOutputMemberContentBlockStop{
			Value: types.ContentBlockStopEvent{ContentBlockIndex: aws.Int32(1)},
		},
		&types.ConverseStreamOutputMemberMessageStop{
			Value: types.MessageStopEvent{StopReason: types.StopReasonToolUse},
		},
	})

	s := &bedrockStream{reader: reader, model: "anthropic.claude-opus-4-v1:0"}

	var (
		textAcc     string
		toolCallSaw *message.ToolCall
		done        bool
	)
	for {
		ev, err := s.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		switch ev.Type {
		case provider.EventDelta:
			if ev.Delta != nil {
				textAcc += ev.Delta.Content
				if len(ev.Delta.ToolCalls) > 0 {
					tc := ev.Delta.ToolCalls[0]
					toolCallSaw = &tc
				}
			}
		case provider.EventDone:
			done = true
			if ev.Response == nil || ev.Response.FinishReason != "tool_use" {
				t.Errorf("done response = %+v", ev.Response)
			}
		}
	}

	if textAcc != "hello world" {
		t.Errorf("text = %q", textAcc)
	}
	if toolCallSaw == nil {
		t.Fatal("expected a tool_call delta")
	}
	if toolCallSaw.ID != "tu-7" || toolCallSaw.Function.Name != "shell" {
		t.Errorf("tool_call = %+v", toolCallSaw)
	}
	if toolCallSaw.Function.Arguments != `{"cmd":"ls"}` {
		t.Errorf("args = %q", toolCallSaw.Function.Arguments)
	}
	if !done {
		t.Error("never saw EventDone")
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if !reader.closed {
		t.Error("Close was not called on the reader")
	}
}

// ----- Complete via fake API -----

// fakeAPI is a converseAPI double used for Complete tests.
type fakeAPI struct {
	converseOut *bedrockruntime.ConverseOutput
	converseErr error
	gotInput    *bedrockruntime.ConverseInput
}

func (f *fakeAPI) Converse(ctx context.Context, in *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
	f.gotInput = in
	if f.converseErr != nil {
		return nil, f.converseErr
	}
	return f.converseOut, nil
}

func (f *fakeAPI) ConverseStream(ctx context.Context, in *bedrockruntime.ConverseStreamInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamOutput, error) {
	return nil, stderrors.New("not implemented in fakeAPI")
}

func TestComplete_HappyPath(t *testing.T) {
	fake := &fakeAPI{
		converseOut: &bedrockruntime.ConverseOutput{
			Output: &types.ConverseOutputMemberMessage{
				Value: types.Message{
					Role:    types.ConversationRoleAssistant,
					Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: "pong"}},
				},
			},
			StopReason: types.StopReasonEndTurn,
			Usage:      &types.TokenUsage{InputTokens: aws.Int32(3), OutputTokens: aws.Int32(1)},
		},
	}
	b := &Bedrock{client: fake, model: "anthropic.claude-opus-4-v1:0", region: "us-east-1"}
	resp, err := b.Complete(context.Background(), &provider.Request{
		Model:    "anthropic.claude-opus-4-v1:0",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("ping")}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Message.Content.Text() != "pong" {
		t.Errorf("text = %q", resp.Message.Content.Text())
	}
	if fake.gotInput == nil || aws.ToString(fake.gotInput.ModelId) != "anthropic.claude-opus-4-v1:0" {
		t.Errorf("forwarded input = %+v", fake.gotInput)
	}
}

func TestComplete_MapsAWSError(t *testing.T) {
	fake := &fakeAPI{converseErr: &types.ThrottlingException{Message: aws.String("slow")}}
	b := &Bedrock{client: fake, model: "x", region: "us-east-1"}
	_, err := b.Complete(context.Background(), &provider.Request{
		Model:    "x",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var pErr *provider.Error
	if !stderrors.As(err, &pErr) || pErr.Kind != provider.ErrRateLimit {
		t.Errorf("err = %#v", err)
	}
}

func TestComplete_EmptyOutput(t *testing.T) {
	fake := &fakeAPI{converseOut: &bedrockruntime.ConverseOutput{}} // Output == nil
	b := &Bedrock{client: fake, model: "x", region: "us-east-1"}
	_, err := b.Complete(context.Background(), &provider.Request{
		Model:    "x",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	if err == nil {
		t.Fatal("expected error on empty response")
	}
}
