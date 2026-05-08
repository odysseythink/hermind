// provider/anthropic/complete.go
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
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

	// Convert tools
	if len(req.Tools) > 0 {
		apiReq.Tools = make([]anthropicTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			apiReq.Tools = append(apiReq.Tools, anthropicTool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: t.Function.Parameters,
			})
		}
	}

	// Convert messages
	for _, m := range req.Messages {
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
// Handles text, tool_use, and tool_result blocks.
func contentToAPIItems(c message.Content) []apiContentItem {
	if c.IsText() {
		return []apiContentItem{{Type: "text", Text: c.Text()}}
	}
	items := make([]apiContentItem, 0, len(c.Blocks()))
	for _, b := range c.Blocks() {
		switch b.Type {
		case "text":
			items = append(items, apiContentItem{Type: "text", Text: b.Text})
		case "tool_use":
			items = append(items, apiContentItem{
				Type:  "tool_use",
				ID:    b.ToolUseID,
				Name:  b.ToolUseName,
				Input: b.ToolUseInput,
			})
		case "tool_result":
			items = append(items, apiContentItem{
				Type:      "tool_result",
				ToolUseID: b.ToolUseID,
				Content:   b.ToolResult,
			})
		}
	}
	return items
}

// convertResponse converts an Anthropic wire response to the provider shape.
// If the response contains any tool_use blocks, Content is returned as
// BlockContent preserving all blocks. Otherwise, Content is TextContent
// with all text concatenated.
func (a *Anthropic) convertResponse(apiResp *messagesResponse) *provider.Response {
	hasToolUse := false
	for _, c := range apiResp.Content {
		if c.Type == "tool_use" {
			hasToolUse = true
			break
		}
	}

	var content message.Content
	if hasToolUse {
		blocks := make([]message.ContentBlock, 0, len(apiResp.Content))
		for _, c := range apiResp.Content {
			switch c.Type {
			case "text":
				blocks = append(blocks, message.ContentBlock{Type: "text", Text: c.Text})
			case "tool_use":
				blocks = append(blocks, message.ContentBlock{
					Type:         "tool_use",
					ToolUseID:    c.ID,
					ToolUseName:  c.Name,
					ToolUseInput: c.Input,
				})
			}
		}
		content = message.BlockContent(blocks)
	} else {
		var text strings.Builder
		for _, c := range apiResp.Content {
			if c.Type == "text" {
				text.WriteString(c.Text)
			}
		}
		content = message.TextContent(text.String())
	}

	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: content,
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
