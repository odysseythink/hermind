// Package anthropic — inbound translator: Anthropic Messages API request
// body → hermind's internal provider.Request.
//
// The Anthropic wire types in this package (messagesRequest, apiMessage,
// apiContentItem, anthropicTool) are reused; the conversion lives here so
// the same struct definitions serve both the outbound provider client
// and the inbound proxy endpoint.
package anthropic

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/pantheon/core"
)

// errInvalid wraps an Anthropic-style invalid_request_error message.
// Callers convert this to a 400 response with the matching error type.
type errInvalid struct {
	code    string // matches Anthropic error type, e.g. "invalid_request_error"
	message string
}

func (e *errInvalid) Error() string { return e.code + ": " + e.message }

func newInvalid(code, msg string) error {
	return &errInvalid{code: code, message: msg}
}

// Inbound parses an Anthropic Messages API request body into an internal
// provider.Request, plus the request's `model` field (echoed in the
// response) and the streaming flag (used to route to Stream vs Complete).
func Inbound(body []byte) (req *provider.Request, requestModel string, stream bool, err error) {
	// Use a permissive raw struct because `system` accepts both string and array.
	var raw struct {
		Model         string          `json:"model"`
		Messages      []apiMessage    `json:"messages"`
		System        json.RawMessage `json:"system"`
		MaxTokens     int             `json:"max_tokens"`
		Temperature   *float64        `json:"temperature"`
		TopP          *float64        `json:"top_p"`
		StopSequences []string        `json:"stop_sequences"`
		Stream        bool            `json:"stream"`
		Tools         []anthropicTool `json:"tools"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, "", false, newInvalid("invalid_request_error", "body decode failed: "+err.Error())
	}

	if raw.MaxTokens <= 0 {
		return nil, "", false, newInvalid("invalid_request_error", "max_tokens is required and must be > 0")
	}
	if len(raw.Messages) == 0 {
		return nil, "", false, newInvalid("invalid_request_error", "messages must be non-empty")
	}

	// system: string accepted; system: []block rejected (caching shape).
	systemPrompt := ""
	if len(raw.System) > 0 {
		var asString string
		if err := json.Unmarshal(raw.System, &asString); err == nil {
			systemPrompt = asString
		} else {
			return nil, "", false, newInvalid("unsupported_system_format", "system as block array is not supported in v1")
		}
	}

	internalMsgs, err := convertInboundMessages(raw.Messages)
	if err != nil {
		return nil, "", false, err
	}

	tools := make([]core.ToolDefinition, 0, len(raw.Tools))
	for _, t := range raw.Tools {
		params, _ := core.SchemaFromJSON(t.InputSchema)
		tools = append(tools, core.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}

	legacyMsgs := make([]message.Message, len(internalMsgs))
	for i, m := range internalMsgs {
		legacyMsgs[i] = message.HermindMessageToLegacy(m)
	}

	out := &provider.Request{
		Model:         raw.Model,
		SystemPrompt:  systemPrompt,
		Messages:      legacyMsgs,
		Tools:         tools,
		MaxTokens:     raw.MaxTokens,
		Temperature:   raw.Temperature,
		TopP:          raw.TopP,
		StopSequences: raw.StopSequences,
	}
	return out, raw.Model, raw.Stream, nil
}

// convertInboundMessages converts Anthropic's content-array messages to
// hermind's per-role internal form, splitting tool_results out of user
// content into separate core.MESSAGE_ROLE_TOOL messages.
func convertInboundMessages(in []apiMessage) ([]message.HermindMessage, error) {
	out := make([]message.HermindMessage, 0, len(in))
	for _, m := range in {
		switch m.Role {
		case "system":
			return nil, newInvalid("system_role_in_messages", "system role must use top-level system field")
		case "user":
			toolResults, textParts, err := splitUserContent(m.Content)
			if err != nil {
				return nil, err
			}
			for _, tr := range toolResults {
				out = append(out, message.HermindMessage{
					Role:       core.MESSAGE_ROLE_TOOL,
					Content:    core.NewTextContent(tr.Content),
					ToolCallID: tr.ToolUseID,
				})
			}
			if len(textParts) > 0 {
				out = append(out, message.HermindMessage{
					Role:    core.MESSAGE_ROLE_USER,
					Content: core.NewTextContent(strings.Join(textParts, "\n")),
				})
			}
		case "assistant":
			texts, toolUses, err := splitAssistantContent(m.Content)
			if err != nil {
				return nil, err
			}
			contentParts := core.NewTextContent(strings.Join(texts, "\n"))
			for _, tu := range toolUses {
				contentParts = append(contentParts, core.ToolCallPart{
					ID:        tu.ID,
					Name:      tu.Name,
					Arguments: string(tu.Input),
				})
			}
			out = append(out, message.HermindMessage{
				Role:    core.MESSAGE_ROLE_ASSISTANT,
				Content: contentParts,
			})
		default:
			return nil, newInvalid("invalid_request_error", "unknown message role: "+m.Role)
		}
	}
	return out, nil
}

// userToolResult collects tool_result blocks pulled out of a user message.
type userToolResult struct {
	ToolUseID string
	Content   string
}

func splitUserContent(items []apiContentItem) ([]userToolResult, []string, error) {
	var trs []userToolResult
	var texts []string
	for _, it := range items {
		switch it.Type {
		case "text":
			texts = append(texts, it.Text)
		case "tool_result":
			trs = append(trs, userToolResult{
				ToolUseID: it.ToolUseID,
				Content:   it.Content,
			})
		case "image":
			// v1: pass through as image text placeholder; provider may reject.
			// Out-of-scope: real image content blocks. Insert a marker so the
			// provider knows non-text was present without breaking the request.
			texts = append(texts, "[image omitted in v1 proxy]")
		default:
			return nil, nil, newInvalid("invalid_request_error", "unsupported user content type: "+it.Type)
		}
	}
	return trs, texts, nil
}

func splitAssistantContent(items []apiContentItem) ([]string, []apiContentItem, error) {
	var texts []string
	var toolUses []apiContentItem
	for _, it := range items {
		switch it.Type {
		case "text":
			texts = append(texts, it.Text)
		case "tool_use":
			toolUses = append(toolUses, it)
		default:
			return nil, nil, newInvalid("invalid_request_error", "unsupported assistant content type: "+it.Type)
		}
	}
	return texts, toolUses, nil
}

// invalidErrorCode returns the Anthropic error type from an *errInvalid,
// or "invalid_request_error" for any other error. Used by callers that
// need to map errors to HTTP error envelopes.
func invalidErrorCode(err error) string {
	var e *errInvalid
	if errors.As(err, &e) {
		return e.code
	}
	return "invalid_request_error"
}

// Outbound serializes a non-streaming provider.Response into Anthropic
// Messages API JSON. The requestModel is echoed unchanged in the
// response's "model" field for SDK-compatibility (some clients validate
// the model name they sent matches what came back). The msgID is the
// server-generated message id (typically randMsgID()).
func Outbound(resp *provider.Response, requestModel, msgID string) ([]byte, error) {
	if resp == nil {
		return nil, fmt.Errorf("anthropic: outbound nil response")
	}
	toolCalls := resp.Message.ExtractToolCalls()
	content := make([]apiContentItem, 0, 1+len(toolCalls))
	if txt := resp.Message.Text(); txt != "" {
		content = append(content, apiContentItem{Type: "text", Text: txt})
	}
	for _, tc := range toolCalls {
		content = append(content, apiContentItem{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Name,
			Input: json.RawMessage(tc.Arguments),
		})
	}
	out := messagesResponse{
		ID:         msgID,
		Type:       "message",
		Role:       "assistant",
		Model:      requestModel,
		Content:    content,
		StopReason: mapFinishReason(resp.FinishReason),
		Usage: apiUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			// CacheRead/CacheCreation intentionally not populated — caching not in v1.
		},
	}
	return json.Marshal(out)
}

// mapFinishReason converts hermind's internal finish reason vocabulary to
// Anthropic's stop_reason vocabulary. Unknown values default to end_turn.
func mapFinishReason(in string) string {
	switch in {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "stop_sequence":
		return "stop_sequence"
	default:
		return "end_turn"
	}
}

// randMsgID returns a fresh "msg_..." identifier for outbound responses.
func randMsgID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "msg_" + hex.EncodeToString(b[:])
}

// NewMsgID returns a fresh "msg_..." identifier suitable for outbound responses.
// Exported wrapper around randMsgID for callers in other packages.
func NewMsgID() string { return randMsgID() }

// InvalidErrorCode returns the Anthropic error type embedded in err if err
// originated from the inbound translator. Returns "" if err is from
// elsewhere.
func InvalidErrorCode(err error) string {
	var e *errInvalid
	if errors.As(err, &e) {
		return e.code
	}
	return ""
}
