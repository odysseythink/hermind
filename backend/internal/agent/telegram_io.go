package agent

import (
	"context"
	"sync"
)

// TelegramAgentIO implements AgentIO for Telegram.
type TelegramAgentIO struct {
	sendTextFn     func(text string) error
	sendApprovalFn func(requestID, skillName, description string, timeoutMs int) error
	mu             sync.Mutex
	lastText       string
}

func NewTelegramAgentIO(sendText func(text string) error, sendApproval func(requestID, skillName, description string, timeoutMs int) error) *TelegramAgentIO {
	return &TelegramAgentIO{
		sendTextFn:     sendText,
		sendApprovalFn: sendApproval,
	}
}

const telegramMaxMsgLen = 4096

func (t *TelegramAgentIO) Send(frame ServerFrame) error {
	switch frame.Type {
	case FrameStatusResponse:
		return t.sendTextFn(frame.Content)
	case FrameToolApprovalReq:
		return t.sendApprovalFn(frame.RequestID, frame.SkillName, frame.Description, frame.TimeoutMs)
	case FrameWSSFailure:
		return t.sendTextFn("❌ " + frame.Content)
	case FrameWaitingOnInput:
		return t.sendTextFn(frame.Question)
	case "":
		// Chat message (from OnMessage bridge)
		if frame.Content == "" {
			return nil
		}
		t.mu.Lock()
		t.lastText = frame.Content
		t.mu.Unlock()
		return t.sendTextInChunks(frame.Content)
	}
	return nil
}

func (t *TelegramAgentIO) sendTextInChunks(text string) error {
	for len(text) > 0 {
		chunk := text
		if len(chunk) > telegramMaxMsgLen {
			chunk = chunk[:telegramMaxMsgLen]
		}
		if err := t.sendTextFn(chunk); err != nil {
			return err
		}
		text = text[len(chunk):]
	}
	return nil
}

func (t *TelegramAgentIO) Close() error { return nil }

// TelegramInput implements AgentInput for Telegram using channels.
type TelegramInput struct {
	ch chan InputAction
}

func NewTelegramInput() *TelegramInput {
	return &TelegramInput{ch: make(chan InputAction, 4)}
}

func (t *TelegramInput) Read(ctx context.Context) (InputAction, error) {
	select {
	case <-ctx.Done():
		return InputAction{}, ctx.Err()
	case act := <-t.ch:
		return act, nil
	}
}

func (t *TelegramInput) Submit(action InputAction) {
	select {
	case t.ch <- action:
	default:
	}
}
