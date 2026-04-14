// provider/wenxin/types.go
package wenxin

// chatRequest is the Wenxin chat API request body.
// Reference: https://cloud.baidu.com/doc/WENXINWORKSHOP/s/4lilb2lpf
type chatRequest struct {
	Messages        []chatMessage `json:"messages"`
	Stream          bool          `json:"stream,omitempty"`
	Temperature     *float64      `json:"temperature,omitempty"`
	TopP            *float64      `json:"top_p,omitempty"`
	System          string        `json:"system,omitempty"`
	MaxOutputTokens int           `json:"max_output_tokens,omitempty"`
	Stop            []string      `json:"stop,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// chatResponse is the full (non-streaming) chat response.
type chatResponse struct {
	ID               string `json:"id"`
	Object           string `json:"object"`
	Created          int64  `json:"created"`
	Result           string `json:"result"`
	IsTruncated      bool   `json:"is_truncated"`
	NeedClearHistory bool   `json:"need_clear_history"`
	Usage            usage  `json:"usage"`

	// Error fields (populated on failure)
	ErrorCode int    `json:"error_code,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// chatStreamEvent is one SSE event in a streaming response.
type chatStreamEvent struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Result  string `json:"result"`
	IsEnd   bool   `json:"is_end"`
	Usage   *usage `json:"usage,omitempty"`

	ErrorCode int    `json:"error_code,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}
