// provider/wenxin/complete.go
package wenxin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// Complete sends a non-streaming chat request to Wenxin.
func (w *Wenxin) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	token, err := w.getAccessToken(ctx)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrAuth,
			Provider: "wenxin",
			Message:  err.Error(),
			Cause:    err,
		}
	}

	apiReq := w.buildRequest(req, false)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("wenxin: marshal: %w", err)
	}

	url := fmt.Sprintf("%s/%s?access_token=%s", w.chatBaseURL, w.modelForURL(req), token)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("wenxin: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := w.http.Do(httpReq)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: "wenxin",
			Message:  fmt.Sprintf("network: %v", err),
			Cause:    err,
		}
	}
	defer resp.Body.Close()

	var apiResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("wenxin: decode: %w", err)
	}

	if apiResp.ErrorCode != 0 {
		return nil, mapErrorCode(apiResp.ErrorCode, apiResp.ErrorMsg)
	}

	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent(apiResp.Result),
		},
		FinishReason: "stop",
		Usage: message.Usage{
			InputTokens:  apiResp.Usage.PromptTokens,
			OutputTokens: apiResp.Usage.CompletionTokens,
		},
		Model: w.modelForURL(req),
	}, nil
}

// buildRequest converts a provider.Request into a Wenxin chatRequest.
// Tool definitions are silently dropped (Plan 3 limitation).
func (w *Wenxin) buildRequest(req *provider.Request, stream bool) *chatRequest {
	out := &chatRequest{
		Stream:          stream,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		System:          req.SystemPrompt,
		MaxOutputTokens: req.MaxTokens,
		Stop:            req.StopSequences,
		Messages:        make([]chatMessage, 0, len(req.Messages)),
	}

	for _, m := range req.Messages {
		role := string(m.Role)
		if role != "user" && role != "assistant" {
			continue
		}
		text := ""
		if m.Content.IsText() {
			text = m.Content.Text()
		} else {
			// Concatenate only text blocks — tool blocks are dropped.
			for _, b := range m.Content.Blocks() {
				if b.Type == "text" {
					text += b.Text
				}
			}
		}
		if text == "" {
			continue
		}
		out.Messages = append(out.Messages, chatMessage{Role: role, Content: text})
	}
	return out
}

// modelForURL returns the model name to put into the Wenxin chat URL path.
// If the Request specifies a model, use it; otherwise use the configured default.
func (w *Wenxin) modelForURL(req *provider.Request) string {
	if req.Model != "" {
		return req.Model
	}
	return w.model
}

// mapErrorCode maps a Baidu error code to a provider.Error.
// Common codes:
//
//	110 — access_token invalid/expired → refresh next call (treat as auth)
//	17 / 18 — rate limit
//	336503 — content filter
func mapErrorCode(code int, msg string) error {
	kind := provider.ErrUnknown
	switch code {
	case 110, 111:
		kind = provider.ErrAuth
	case 17, 18, 4:
		kind = provider.ErrRateLimit
	case 336503:
		kind = provider.ErrContentFilter
	case 336000, 336001:
		kind = provider.ErrServerError
	}
	return &provider.Error{
		Kind:       kind,
		Provider:   "wenxin",
		StatusCode: 200, // Wenxin returns errors with HTTP 200
		Message:    fmt.Sprintf("error %d: %s", code, msg),
	}
}
