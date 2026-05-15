// provider/provider.go
package provider

import (
	"context"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/types"
)

// Request is a provider-agnostic chat completion request.
type Request struct {
	Model        string
	SystemPrompt string
	Messages     []message.HermindMessage
	// Tools is the set of tool definitions the LLM may invoke. May be empty.
	Tools         []core.ToolDefinition
	MaxTokens     int
	Temperature   *float64
	TopP          *float64
	StopSequences []string
}

// Response is a provider-agnostic chat completion response.
type Response struct {
	Message      message.HermindMessage
	FinishReason string
	Usage        core.Usage
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

// StreamDelta is the incremental content of a streaming event.
type StreamDelta struct {
	Content   string           `json:"content,omitempty"`
	ToolCalls []types.ToolCall `json:"tool_calls,omitempty"`
	Reasoning string           `json:"reasoning,omitempty"`
}

// EmbedCapable is an optional capability for providers that support text embeddings.
type EmbedCapable interface {
	// Embed returns a float32 vector for the given text.
	// Returns an error if the embedding call fails.
	Embed(ctx context.Context, model, text string) ([]float32, error)
}
