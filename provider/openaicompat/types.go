// provider/openaicompat/types.go
package openaicompat

import "encoding/json"

// chatRequest is the JSON body sent to /v1/chat/completions.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []apiMessage  `json:"messages"`
	Tools       []apiTool     `json:"tools,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	// StreamOptions is OpenAI-specific and controls usage reporting in streams.
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// apiMessage is one message in the chat array.
// OpenAI uses: "system", "user", "assistant", "tool"
type apiMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content"` // string or []apiContentPart or nil
	Name       string         `json:"name,omitempty"`
	ToolCalls  []apiToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// apiContentPart is one element of a multimodal content array.
type apiContentPart struct {
	Type     string `json:"type"` // "text" or "image_url"
	Text     string `json:"text,omitempty"`
	ImageURL *apiImageURL `json:"image_url,omitempty"`
}

type apiImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// apiToolCall is one tool invocation request from the assistant.
type apiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"` // always "function"
	Function apiFunctionCall `json:"function"`
}

type apiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded argument string
}

// apiTool is a single tool definition in the request.
type apiTool struct {
	Type     string         `json:"type"` // always "function"
	Function apiFunctionDef `json:"function"`
}

type apiFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// chatResponse is the JSON body returned by /v1/chat/completions (non-streaming).
type chatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []apiChoice  `json:"choices"`
	Usage   apiUsage     `json:"usage"`
}

type apiChoice struct {
	Index        int        `json:"index"`
	Message      apiMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

type apiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// chatStreamChunk is one SSE chunk in a streaming response.
type chatStreamChunk struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []apiStreamChoice  `json:"choices"`
	Usage   *apiUsage          `json:"usage,omitempty"` // only on final chunk when stream_options.include_usage is true
}

type apiStreamChoice struct {
	Index        int            `json:"index"`
	Delta        apiStreamDelta `json:"delta"`
	FinishReason string         `json:"finish_reason"`
}

type apiStreamDelta struct {
	Role      string               `json:"role,omitempty"`
	Content   string               `json:"content,omitempty"`
	ToolCalls []apiStreamToolCall  `json:"tool_calls,omitempty"`
}

// apiStreamToolCall is a tool call chunk in a streaming response.
// Tool calls are streamed incrementally: id/name come first, then
// arguments arrive as concatenated string chunks.
type apiStreamToolCall struct {
	Index    int                 `json:"index"`
	ID       string              `json:"id,omitempty"`
	Type     string              `json:"type,omitempty"`
	Function *apiStreamFunction  `json:"function,omitempty"`
}

type apiStreamFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"` // partial JSON fragment
}

// apiErrorResponse is the error body returned for non-2xx responses.
type apiErrorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}
