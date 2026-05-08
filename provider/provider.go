// provider/provider.go
package provider

import (
	"context"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/tool"
)

// Provider is the interface every LLM backend implements.
// Implementations must be safe for concurrent use.
type Provider interface {
	// Name returns the provider's canonical name (e.g., "anthropic").
	Name() string

	// Complete makes a non-streaming API call.
	Complete(ctx context.Context, req *Request) (*Response, error)

	// Stream makes a streaming API call. The returned Stream is not
	// safe for concurrent use.
	Stream(ctx context.Context, req *Request) (Stream, error)

	// ModelInfo returns capabilities for a specific model. Returns nil
	// if the model is unknown to this provider.
	ModelInfo(model string) *ModelInfo

	// EstimateTokens estimates the token count for a text string using
	// this provider's tokenizer.
	EstimateTokens(model string, text string) (int, error)

	// Available returns true if the provider has valid configuration
	// (e.g., API key is set) and is ready to accept requests.
	Available() bool
}

// Stream is an iterator over streaming API events.
// Call Recv repeatedly until EventDone or an error. Always Close when done.
type Stream interface {
	// Recv blocks until the next event or an error.
	Recv() (*StreamEvent, error)
	// Close releases any resources (connections, decoders).
	Close() error
}

// StreamEventType discriminates between streaming events.
type StreamEventType int

const (
	EventDelta StreamEventType = iota // incremental content or tool call
	EventDone                         // stream finished, Response populated
	EventError                        // error, Err populated
)

// StreamEvent is a single item produced by a streaming provider.
type StreamEvent struct {
	Type     StreamEventType
	Delta    *StreamDelta
	Response *Response
	Err      error
}

// StreamDelta is the incremental content of a streaming event.
type StreamDelta struct {
	Content   string             `json:"content,omitempty"`
	ToolCalls []message.ToolCall `json:"tool_calls,omitempty"`
	Reasoning string             `json:"reasoning,omitempty"`
}

// Request is a provider-agnostic chat completion request.
type Request struct {
	Model        string
	SystemPrompt string
	Messages     []message.Message
	// Tools is the set of tool definitions the LLM may invoke. May be empty.
	Tools         []tool.ToolDefinition
	MaxTokens     int
	Temperature   *float64
	TopP          *float64
	StopSequences []string
}

// Response is a provider-agnostic chat completion response.
type Response struct {
	Message      message.Message
	FinishReason string
	Usage        message.Usage
	Model        string // actual model used
}

// ModelInfo describes a model's capabilities.
type ModelInfo struct {
	ContextLength     int
	MaxOutputTokens   int
	SupportsVision    bool
	SupportsTools     bool
	SupportsStreaming bool
	SupportsCaching   bool
	SupportsReasoning bool
}

// ModelLister is an optional capability for providers that expose a
// models-listing endpoint. Webconfig consumers do a type assertion
// (`lister, ok := p.(ModelLister)`) before offering model discovery.
// Providers whose backend does not expose model listing simply do not
// implement this interface; callers should handle the negative assert.
type ModelLister interface {
	// ListModels returns the model IDs advertised by the provider.
	// Ordering is provider-defined (typically the server's response
	// order preserved). An empty slice is a valid result. Errors
	// carry the underlying HTTP status or transport error; callers
	// should surface them without further wrapping.
	ListModels(ctx context.Context) ([]string, error)
}
