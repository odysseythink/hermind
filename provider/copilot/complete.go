package copilot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// Complete drives one round-trip: initialize (once per subprocess),
// session/new (if no session yet), then session/prompt. The Copilot
// CLI's async "session/update" notifications arrive on the bridge
// channel but are ignored by Complete — Stream() consumes them.
func (c *Copilot) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	sub, err := c.ensureSubprocess()
	if err != nil {
		return nil, err
	}
	if err := c.initialize(ctx, sub); err != nil {
		return nil, err
	}
	sessionID, err := c.openSession(ctx, sub)
	if err != nil {
		return nil, err
	}

	promptText := BuildPrompt(req)
	res, err := sub.call(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]string{{"type": "text", "text": promptText}},
	})
	if err != nil {
		return nil, err
	}

	// Drain any session/update notifications to build the assistant
	// reply. In the fake harness we just look at the final result.
	var promptResp struct {
		StopReason string `json:"stopReason"`
	}
	_ = json.Unmarshal(res, &promptResp)

	// Collect the assistant text from pending notifications.
	text := drainAssistantText(sub)

	calls, cleaned := ExtractToolCalls(text)
	var content message.Content
	if len(calls) > 0 {
		blocks := []message.ContentBlock{{Type: "text", Text: cleaned}}
		for _, tc := range calls {
			blocks = append(blocks, message.ContentBlock{
				Type:         "tool_use",
				ToolUseID:    tc.ID,
				ToolUseName:  tc.Name,
				ToolUseInput: tc.Arguments,
			})
		}
		content = message.BlockContent(blocks)
	} else {
		content = message.TextContent(cleaned)
	}

	if promptResp.StopReason == "" {
		promptResp.StopReason = "end_turn"
	}
	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: content,
		},
		FinishReason: promptResp.StopReason,
		Model:        req.Model,
	}, nil
}

func (c *Copilot) initialize(ctx context.Context, sub *subprocess) error {
	c.mu.Lock()
	if c.initialized {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	_, err := sub.call(ctx, "initialize", map[string]any{
		"protocolVersion": 1,
	})
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.initialized = true
	c.mu.Unlock()
	return nil
}

func (c *Copilot) openSession(ctx context.Context, sub *subprocess) (string, error) {
	c.mu.Lock()
	if c.sessionID != "" {
		id := c.sessionID
		c.mu.Unlock()
		return id, nil
	}
	c.mu.Unlock()
	res, err := sub.call(ctx, "session/new", map[string]any{"cwd": "."})
	if err != nil {
		return "", err
	}
	var out struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return "", fmt.Errorf("copilot: session/new: %w", err)
	}
	if out.SessionID == "" {
		return "", fmt.Errorf("copilot: session/new returned empty sessionId")
	}
	c.mu.Lock()
	c.sessionID = out.SessionID
	c.mu.Unlock()
	return out.SessionID, nil
}

// drainAssistantText pulls any pending session/update notifications
// and concatenates their text. Blocks only until the bridge drains
// — we never wait for "more" because the prompt response has already
// been delivered.
func drainAssistantText(sub *subprocess) string {
	var sb []byte
	for {
		select {
		case n := <-sub.noteBridge:
			if n.Method != "session/update" {
				continue
			}
			var params struct {
				Update struct {
					AgentMessageChunk struct {
						Text string `json:"text"`
					} `json:"agentMessageChunk"`
				} `json:"update"`
			}
			_ = json.Unmarshal(n.Params, &params)
			sb = append(sb, []byte(params.Update.AgentMessageChunk.Text)...)
		default:
			return string(sb)
		}
	}
}
