// provider/openaicompat/complete.go
package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// Complete sends a non-streaming chat completion request.
func (c *Client) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	apiReq := c.buildRequest(req, false)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal request: %w", c.cfg.ProviderName, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", c.cfg.ProviderName, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	for k, v := range c.cfg.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: c.cfg.ProviderName,
			Message:  fmt.Sprintf("network error: %v", err),
			Cause:    err,
		}
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, mapHTTPError(c.cfg.ProviderName, httpResp)
	}

	var apiResp chatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", c.cfg.ProviderName, err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("%s: response has no choices", c.cfg.ProviderName)
	}

	choice := apiResp.Choices[0]
	msg := convertResponseMessage(choice.Message)
	msg.Reasoning = choice.Message.ReasoningContent
	return &provider.Response{
		Message:      msg,
		FinishReason: choice.FinishReason,
		Usage: message.Usage{
			InputTokens:  apiResp.Usage.PromptTokens,
			OutputTokens: apiResp.Usage.CompletionTokens,
		},
		Model: apiResp.Model,
	}, nil
}
