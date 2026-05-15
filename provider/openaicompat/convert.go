// provider/openaicompat/convert.go
package openaicompat

import (
	"encoding/json"
	"log"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/pantheon/core"
)

// buildRequest converts a provider.Request into an OpenAI-compatible chatRequest.
// This is where the internal canonical format (Anthropic-native BlockContent)
// is flattened into OpenAI's tool_calls/tool role shape.
func (c *Client) buildRequest(req *provider.Request, stream bool) *chatRequest {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	model := req.Model
	if model == "" {
		model = c.cfg.Model
	}

	apiReq := &chatRequest{
		Model:       model,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.StopSequences,
		Stream:      stream,
		Messages:    make([]apiMessage, 0, len(req.Messages)+1),
	}

	if stream {
		apiReq.StreamOptions = &streamOptions{IncludeUsage: true}
	}

	// System prompt goes in as the first system message
	if req.SystemPrompt != "" {
		apiReq.Messages = append(apiReq.Messages, apiMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Convert tool definitions
	if len(req.Tools) > 0 {
		apiReq.Tools = make([]apiTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			apiReq.Tools = append(apiReq.Tools, convertToolDefinition(t))
		}
		apiReq.ToolChoice = "auto"
		log.Printf("[%s] buildRequest: sending %d tools (tool_choice=auto)", c.cfg.ProviderName, len(apiReq.Tools))
		for _, t := range apiReq.Tools {
			log.Printf("[%s]   tool: %s", c.cfg.ProviderName, t.Function.Name)
		}
	} else {
		log.Printf("[%s] buildRequest: no tools in request", c.cfg.ProviderName)
	}

	// Convert conversation messages
	for _, m := range req.Messages {
		apiReq.Messages = append(apiReq.Messages, convertMessage(m)...)
	}

	return apiReq
}

// convertToolDefinition maps a core.ToolDefinition to an apiTool.
func convertToolDefinition(t core.ToolDefinition) apiTool {
	return apiTool{
		Type: "function",
		Function: apiFunctionDef{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  schemaToRaw(t.Parameters),
		},
	}
}

// convertMessage converts a message.Message to one or more apiMessage values.
// One internal message may expand into MULTIPLE wire messages (e.g. a user
// message with multiple tool_result blocks becomes N tool messages).
func convertMessage(m message.Message) []apiMessage {
	// Plain text path
	if m.Content.IsText() {
		return []apiMessage{{
			Role:    string(m.Role),
			Content: m.Content.Text(),
		}}
	}

	// BlockContent path — inspect block types
	blocks := m.Content.Blocks()

	// Separate block types
	var (
		textParts   []string
		toolUses    []apiToolCall
		toolResults []apiMessage
	)
	for _, b := range blocks {
		switch b.Type {
		case "text":
			textParts = append(textParts, b.Text)
		case "tool_use":
			toolUses = append(toolUses, apiToolCall{
				ID:   b.ToolUseID,
				Type: "function",
				Function: apiFunctionCall{
					Name:      b.ToolUseName,
					Arguments: string(b.ToolUseInput),
				},
			})
		case "tool_result":
			// Each tool_result becomes its own "tool" role message.
			toolResults = append(toolResults, apiMessage{
				Role:       "tool",
				ToolCallID: b.ToolUseID,
				Name:       b.ToolUseName,
				Content:    b.ToolResult,
			})
		}
	}

	// Decide the shape of the primary message based on block types.
	var primary apiMessage

	if len(toolUses) > 0 {
		// Assistant message requesting tool calls.
		// content may be nil OR a text prefix the assistant said before calling tools.
		var content any
		if len(textParts) > 0 {
			content = joinStrings(textParts)
		}
		primary = apiMessage{
			Role:             string(m.Role),
			Content:          content,
			ToolCalls:        toolUses,
			ReasoningContent: m.Reasoning,
		}
		// tool_result blocks shouldn't be in the same message as tool_use, but
		// if they are, append them after the primary as separate tool messages.
		if len(toolResults) > 0 {
			out := append([]apiMessage{primary}, toolResults...)
			return out
		}
		return []apiMessage{primary}
	}

	if len(toolResults) > 0 {
		// Pure tool_result message (typical for user role after tool execution).
		// Return only the tool messages — the role on the outer message is ignored.
		return toolResults
	}

	// Plain text-in-blocks fallback
	primary = apiMessage{
		Role:             string(m.Role),
		Content:          joinStrings(textParts),
		ReasoningContent: m.Reasoning,
	}
	return []apiMessage{primary}
}

// joinStrings concatenates strings without importing strings.Join if empty.
func joinStrings(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	out := make([]byte, 0, total)
	for _, p := range parts {
		out = append(out, p...)
	}
	return string(out)
}

// convertResponseMessage converts an OpenAI apiMessage (from a response) into
// a message.Message in the internal canonical format. Tool calls on the
// response are mapped to tool_use ContentBlocks.
func convertResponseMessage(m apiMessage) message.Message {
	// If the assistant returned tool_calls, produce BlockContent.
	if len(m.ToolCalls) > 0 {
		blocks := make([]message.ContentBlock, 0, len(m.ToolCalls)+1)

		// Optional leading text if the model said something before calling tools.
		if text := asString(m.Content); text != "" {
			blocks = append(blocks, message.ContentBlock{Type: "text", Text: text})
		}

		for _, tc := range m.ToolCalls {
			blocks = append(blocks, message.ContentBlock{
				Type:         "tool_use",
				ToolUseID:    tc.ID,
				ToolUseName:  tc.Function.Name,
				ToolUseInput: []byte(tc.Function.Arguments),
			})
		}

		return message.Message{
			Role:      message.Role(m.Role),
			Content:   message.BlockContent(blocks),
			Reasoning: m.ReasoningContent,
		}
	}

	// Plain text response
	return message.Message{
		Role:      message.Role(m.Role),
		Content:   message.TextContent(asString(m.Content)),
		Reasoning: m.ReasoningContent,
	}
}

// asString extracts a plain string from an OpenAI content field which may be
// a string, an array of content parts, or nil.
func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []any:
		// Array form — join text parts. Ignore non-text parts.
		var out []byte
		for _, item := range t {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if m["type"] == "text" {
				if s, ok := m["text"].(string); ok {
					out = append(out, s...)
				}
			}
		}
		return string(out)
	default:
		return ""
	}
}

// convertUsage maps the OpenAI usage shape to the internal Usage.
func convertUsage(u apiUsage) provider.Response {
	return provider.Response{
		Usage: message.Usage{
			InputTokens:  u.PromptTokens,
			OutputTokens: u.CompletionTokens,
		},
	}
}

// Silence unused imports (message is used in Message return types)
var _ = message.RoleAssistant

// schemaToRaw marshals a pantheon core.Schema to json.RawMessage.
// Returns nil if schema is nil.
func schemaToRaw(schema *core.Schema) json.RawMessage {
	if schema == nil {
		return nil
	}
	b, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	return b
}
