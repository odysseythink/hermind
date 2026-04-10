// provider/anthropic/types.go
package anthropic

import "encoding/json"

// These types match the Anthropic Messages API wire format.
// Ref: https://docs.anthropic.com/en/api/messages

// messagesRequest is the JSON body sent to /v1/messages.
type messagesRequest struct {
	Model         string          `json:"model"`
	Messages      []apiMessage    `json:"messages"`
	System        string          `json:"system,omitempty"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	Tools         []anthropicTool `json:"tools,omitempty"`
}

// anthropicTool is the Anthropic wire format for tool definitions.
// Ref: https://docs.anthropic.com/en/api/tool-use
type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// apiMessage is a single message in the Anthropic Messages API format.
// Anthropic uses "user"/"assistant" roles only — "system" is a separate field.
type apiMessage struct {
	Role    string           `json:"role"`
	Content []apiContentItem `json:"content"`
}

// apiContentItem represents one element of an Anthropic message content array.
// Anthropic supports "text", "image", "tool_use", "tool_result".
type apiContentItem struct {
	Type string `json:"type"`

	// text
	Text string `json:"text,omitempty"`

	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// messagesResponse is the JSON body returned by /v1/messages.
type messagesResponse struct {
	ID           string           `json:"id"`
	Type         string           `json:"type"`
	Role         string           `json:"role"`
	Model        string           `json:"model"`
	Content      []apiContentItem `json:"content"`
	StopReason   string           `json:"stop_reason"`
	StopSequence string           `json:"stop_sequence,omitempty"`
	Usage        apiUsage         `json:"usage"`
}

type apiUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// apiErrorResponse is the JSON body for error responses.
type apiErrorResponse struct {
	Type  string   `json:"type"`
	Error apiError `json:"error"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
