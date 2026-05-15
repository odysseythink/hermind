package message

import (
	"fmt"

	"github.com/odysseythink/pantheon/core"
)

// ---- pantheon aliases (agent layer) ----

type (
	HermindMessage = core.Message
)

// ToPantheon converts a hermind Message to a pantheon core.Message.
// Since the types are identical this is almost a no-op, but it preserves
// the legacy compatibility fix that rewrites core.MESSAGE_ROLE_USER → core.MESSAGE_ROLE_TOOL when the
// message carries tool results.
func ToPantheon(m HermindMessage) core.Message {
	role := m.Role
	origRole := role

	if role == core.MESSAGE_ROLE_USER && hasToolResultPart(m.Content) {
		role = core.MESSAGE_ROLE_TOOL
	}
	if origRole != role {
		fmt.Printf("[ToPantheon] CONVERTED role %s -> %s parts=%d\n", origRole, role, len(m.Content))
		for i, p := range m.Content {
			fmt.Printf("[ToPantheon]   part[%d] type=%T\n", i, p)
		}
	}

	return core.Message{
		Role:       core.MessageRoleType(role),
		Content:    m.Content,
		Name:       m.Name,
		ToolCallID: m.ToolCallID,
	}
}

func hasToolResultPart(parts []core.ContentParter) bool {
	for _, p := range parts {
		if _, ok := p.(core.ToolResultPart); ok {
			return true
		}
	}
	return false
}

// MessageFromPantheon converts a pantheon core.Message to a hermind Message.
func MessageFromPantheon(m core.Message) HermindMessage {
	return HermindMessage(m)
}

// ---- legacy types (provider layer) ----

// Role identifies who produced a message in the conversation (user, assistant, tool, system).
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
	RoleSystem    Role = "system"
)

// Message represents a single turn in a conversation.
// Content is a typed union — see content.go for the Content type.
type Message struct {
	Role         Role       `json:"role"`
	Content      Content    `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID   string     `json:"tool_call_id,omitempty"`
	ToolName     string     `json:"tool_name,omitempty"`
	Reasoning    string     `json:"reasoning,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
}

// ToolCall represents a single tool invocation requested by the assistant.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // always "function"
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the function name and JSON-encoded arguments of a tool call.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded argument string
}

// Usage tracks token accounting for a single API call.
type Usage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	PromptTokens     int `json:"prompt_tokens"`      // alias for InputTokens
	CompletionTokens int `json:"completion_tokens"`  // alias for OutputTokens
	CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
	ReasoningTokens  int `json:"reasoning_tokens,omitempty"`
}

// ExtractToolCalls returns tool calls from the legacy ToolCalls field,
// converted to pantheon core.ToolCallPart for compatibility.
func (m Message) ExtractToolCalls() []core.ToolCallPart {
	out := make([]core.ToolCallPart, len(m.ToolCalls))
	for i, tc := range m.ToolCalls {
		out[i] = core.ToolCallPart{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		}
	}
	return out
}

// Text returns the text content of the message.
func (m Message) Text() string {
	if m.Content.IsText() {
		return m.Content.Text()
	}
	var out string
	for _, b := range m.Content.Blocks() {
		if b.Type == "text" {
			out += b.Text
		}
	}
	return out
}

// HermindMessageToLegacy converts a pantheon core.Message to the legacy message.Message.
// This is used at the boundary between the agent layer (HermindMessage) and the provider layer (Message).
func HermindMessageToLegacy(m HermindMessage) Message {
	var content Content
	if len(m.Content) == 1 {
		if tp, ok := m.Content[0].(core.TextPart); ok {
			content = TextContent(tp.Text)
		} else {
			content = partsToContent(m.Content)
		}
	} else if len(m.Content) > 1 {
		content = partsToContent(m.Content)
	}

	return Message{
		Role:       Role(m.Role),
		Content:    content,
		ToolCallID: m.ToolCallID,
	}
}

// LegacyToHermindMessage converts a legacy message.Message to a pantheon core.Message.
func LegacyToHermindMessage(m Message) HermindMessage {
	var parts []core.ContentParter
	if m.Content.IsText() {
		parts = core.NewTextContent(m.Content.Text())
	} else {
		parts = blocksToParts(m.Content.Blocks())
	}

	msg := HermindMessage{
		Role:       core.MessageRoleType(m.Role),
		Content:    parts,
		ToolCallID: m.ToolCallID,
		Name:       "",
	}
	return msg
}

func partsToContent(parts []core.ContentParter) Content {
	blocks := make([]ContentBlock, 0, len(parts))
	for _, p := range parts {
		switch part := p.(type) {
		case core.TextPart:
			blocks = append(blocks, ContentBlock{Type: "text", Text: part.Text})
		case core.ImagePart:
			blocks = append(blocks, ContentBlock{Type: "image_url", ImageURL: &Image{URL: part.URL, Detail: part.Detail}})
		case core.ToolCallPart:
			blocks = append(blocks, ContentBlock{
				Type:         "tool_use",
				ToolUseID:    part.ID,
				ToolUseName:  part.Name,
				ToolUseInput: []byte(part.Arguments),
			})
		case core.ToolResultPart:
			var resultText string
			for _, cp := range part.Content {
				if tp, ok := cp.(core.TextPart); ok {
					resultText += tp.Text
				}
			}
			blocks = append(blocks, ContentBlock{
				Type:       "tool_result",
				ToolUseID:  part.ToolCallID,
				ToolResult: resultText,
			})
		default:
			if tp, ok := p.(core.TextPart); ok {
				blocks = append(blocks, ContentBlock{Type: "text", Text: tp.Text})
			}
		}
	}
	return BlockContent(blocks)
}

func blocksToParts(blocks []ContentBlock) []core.ContentParter {
	parts := make([]core.ContentParter, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, core.TextPart{Text: b.Text})
		case "image_url":
			if b.ImageURL != nil {
				parts = append(parts, core.ImagePart{URL: b.ImageURL.URL, Detail: b.ImageURL.Detail})
			}
		case "tool_use":
			parts = append(parts, core.ToolCallPart{
				ID:        b.ToolUseID,
				Name:      b.ToolUseName,
				Arguments: string(b.ToolUseInput),
			})
		case "tool_result":
			parts = append(parts, core.ToolResultPart{
				ToolCallID: b.ToolUseID,
				Content:    core.NewTextContent(b.ToolResult),
			})
		}
	}
	return parts
}
