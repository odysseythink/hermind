package copilot

import (
	"encoding/json"
	"strings"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// BuildPrompt turns a provider.Request into the single user-visible
// text blob the Copilot ACP server expects. The format mirrors the
// Python copilot_acp_client so tool-call extraction works identically.
func BuildPrompt(req *provider.Request) string {
	var b strings.Builder
	b.WriteString("You are being used as the active ACP agent backend for Hermind.\n")
	b.WriteString("If you take an action with a tool, you MUST output tool calls using\n")
	b.WriteString("<tool_call>{...}</tool_call> blocks with JSON exactly in OpenAI function-call shape.\n")
	b.WriteString("If no tool is needed, answer normally.\n\n")

	if req.SystemPrompt != "" {
		b.WriteString("System:\n")
		b.WriteString(req.SystemPrompt)
		b.WriteString("\n\n")
	}

	if len(req.Tools) > 0 {
		b.WriteString("Available tools:\n")
		for _, t := range req.Tools {
			params, _ := json.Marshal(t.Parameters)
			schema, _ := json.Marshal(map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  json.RawMessage(params),
			})
			b.Write(schema)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("Conversation transcript:\n\n")
	for _, m := range req.Messages {
		role := "User"
		if m.Role == message.RoleAssistant {
			role = "Assistant"
		} else if m.Role == message.RoleTool {
			role = "Tool"
		} else if m.Role == message.RoleSystem {
			role = "System"
		}
		b.WriteString(role)
		b.WriteString(":\n")
		b.WriteString(messageText(m))
		b.WriteString("\n\n")
	}
	b.WriteString("Continue the conversation from the latest user request.")
	return b.String()
}

// messageText renders a Message's content to plain text. Structured
// blocks are flattened into their text / tool-result payloads.
func messageText(m message.Message) string {
	if m.Content.IsText() {
		return m.Content.Text()
	}
	var sb strings.Builder
	for _, blk := range m.Content.Blocks() {
		switch blk.Type {
		case "text":
			sb.WriteString(blk.Text)
		case "tool_result":
			sb.WriteString(blk.ToolResult)
		case "tool_use":
			sb.WriteString("<tool_call>")
			raw, _ := json.Marshal(map[string]any{
				"id":        blk.ToolUseID,
				"name":      blk.ToolUseName,
				"arguments": json.RawMessage(blk.ToolUseInput),
			})
			sb.Write(raw)
			sb.WriteString("</tool_call>")
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}
