// provider/anthropic/complete.go
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
)

// Complete sends a non-streaming request to /v1/messages.
func (a *Anthropic) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	apiReq := a.buildRequest(req, false)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", defaultAPIVersion)

	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		// Network errors map to ErrServerError (retryable)
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: "anthropic",
			Message:  fmt.Sprintf("network error: %v", err),
			Cause:    err,
		}
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, mapHTTPError(httpResp)
	}

	var apiResp messagesResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w", err)
	}

	return a.convertResponse(&apiResp), nil
}

// buildRequest converts a provider.Request to the Anthropic wire format.
func (a *Anthropic) buildRequest(req *provider.Request, stream bool) *messagesRequest {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	apiReq := &messagesRequest{
		Model:         req.Model,
		System:        req.SystemPrompt,
		MaxTokens:     maxTokens,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.StopSequences,
		Stream:        stream,
		Messages:      make([]apiMessage, 0, len(req.Messages)),
	}

	for _, m := range req.Messages {
		// Anthropic only supports "user" and "assistant" roles in messages.
		// System messages go to the top-level System field.
		// Tool messages are handled in Plan 2.
		role := string(m.Role)
		if role != "user" && role != "assistant" {
			continue
		}
		apiReq.Messages = append(apiReq.Messages, apiMessage{
			Role:    role,
			Content: contentToAPIItems(m.Content),
		})
	}
	return apiReq
}

// contentToAPIItems converts message.Content to Anthropic's content array format.
func contentToAPIItems(c message.Content) []apiContentItem {
	if c.IsText() {
		return []apiContentItem{{Type: "text", Text: c.Text()}}
	}
	items := make([]apiContentItem, 0, len(c.Blocks()))
	for _, b := range c.Blocks() {
		if b.Type == "text" {
			items = append(items, apiContentItem{Type: "text", Text: b.Text})
		}
		// Image and tool blocks handled in Plan 2/5.
	}
	return items
}

// convertResponse converts an Anthropic wire response to the provider shape.
func (a *Anthropic) convertResponse(apiResp *messagesResponse) *provider.Response {
	// Concatenate all text blocks into a single text response.
	// In Plan 2, tool_use blocks will be converted to ToolCalls.
	var text string
	for _, c := range apiResp.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}

	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent(text),
		},
		FinishReason: apiResp.StopReason,
		Usage: message.Usage{
			InputTokens:      apiResp.Usage.InputTokens,
			OutputTokens:     apiResp.Usage.OutputTokens,
			CacheReadTokens:  apiResp.Usage.CacheReadInputTokens,
			CacheWriteTokens: apiResp.Usage.CacheCreationInputTokens,
		},
		Model: apiResp.Model,
	}
}
