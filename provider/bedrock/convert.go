// Package bedrock: request/response conversion between the
// provider-agnostic shapes (provider.Request / provider.Response) and
// the AWS Converse API types.
package bedrock

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// buildConverseInput shapes a provider.Request into a ConverseInput.
// The same input is also used to populate a ConverseStreamInput via
// copyToStreamInput; both Converse and ConverseStream share the same
// wire schema for the common fields.
func buildConverseInput(req *provider.Request) *bedrockruntime.ConverseInput {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
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
						Value: jsonToDoc(t.Function.Parameters),
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

// copyToStreamInput mirrors a ConverseInput onto a ConverseStreamInput.
// The two types share the same fields but are distinct structs in the
// AWS SDK, so a direct assignment is not possible.
func copyToStreamInput(in *bedrockruntime.ConverseInput) *bedrockruntime.ConverseStreamInput {
	return &bedrockruntime.ConverseStreamInput{
		ModelId:         in.ModelId,
		Messages:        in.Messages,
		System:          in.System,
		InferenceConfig: in.InferenceConfig,
		ToolConfig:      in.ToolConfig,
	}
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
					Input:     jsonToDoc(b.ToolUseInput),
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

// jsonToDoc converts a json.RawMessage into a document.Interface that
// the Bedrock Converse wire format expects. The input is decoded to a
// generic Go value (via NewLazyDocument), so any valid JSON round-trips.
// An empty/nil payload becomes an empty object, which Converse accepts
// for tool inputs that take no arguments.
func jsonToDoc(raw json.RawMessage) document.Interface {
	if len(raw) == 0 {
		return document.NewLazyDocument(map[string]any{})
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		// Unparseable JSON: fall back to empty object rather than crash.
		return document.NewLazyDocument(map[string]any{})
	}
	return document.NewLazyDocument(v)
}

// docToRawJSON marshals a document.Interface back to its JSON bytes.
// Returns `{}` on error so callers don't have to handle nil.
func docToRawJSON(d document.Interface) json.RawMessage {
	if d == nil {
		return json.RawMessage(`{}`)
	}
	raw, err := d.MarshalSmithyDocument()
	if err != nil || len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(raw)
}

// convertConverseOutput shapes a ConverseOutput into the provider's
// Response. Tool-use blocks are preserved; plain text messages
// collapse to the convenience TextContent form.
func convertConverseOutput(out *bedrockruntime.ConverseOutput, model string) *provider.Response {
	msg, _ := out.Output.(*types.ConverseOutputMemberMessage)
	var text string
	var blocks []message.ContentBlock
	hasStructured := false
	if msg != nil {
		for _, c := range msg.Value.Content {
			switch v := c.(type) {
			case *types.ContentBlockMemberText:
				blocks = append(blocks, message.ContentBlock{Type: "text", Text: v.Value})
				text += v.Value
			case *types.ContentBlockMemberToolUse:
				hasStructured = true
				blocks = append(blocks, message.ContentBlock{
					Type:         "tool_use",
					ToolUseID:    aws.ToString(v.Value.ToolUseId),
					ToolUseName:  aws.ToString(v.Value.Name),
					ToolUseInput: docToRawJSON(v.Value.Input),
				})
			}
		}
	}

	var content message.Content
	if hasStructured {
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
	case types.StopReasonContentFiltered:
		return "content_filtered"
	case types.StopReasonGuardrailIntervened:
		return "guardrail_intervened"
	default:
		if s := string(r); s != "" {
			return s
		}
		return "end_turn"
	}
}
