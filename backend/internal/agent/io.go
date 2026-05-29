package agent

import (
	"context"
	"encoding/json"

	"github.com/gorilla/websocket"
	"github.com/odysseythink/mlog"
)

// AgentIO is the transport-neutral output sink for session events.
type AgentIO interface {
	Send(frame ServerFrame) error
	Close() error
}

// InputType classifies actions coming from the user/transport.
type InputType int

const (
	InputContinue InputType = iota
	InputAbort
	InputToolApprovalResponse
	InputSetAutoApprove
)

// InputAction is a single user action delivered to the session reader.
type InputAction struct {
	Type        InputType
	Content     string
	RequestID   string
	Approved    bool
	AutoApprove bool
}

// AgentInput is the transport-neutral input source for user actions.
type AgentInput interface {
	Read(ctx context.Context) (InputAction, error)
}

// wsInput adapts wsConn into AgentInput for the existing WebSocket path.
type wsInput struct {
	conn *wsConn
}

func (w *wsInput) Read(ctx context.Context) (InputAction, error) {
	mt, raw, err := w.conn.ReadMessage()
	if err != nil {
		return InputAction{}, err
	}
	if mt != websocket.TextMessage {
		return InputAction{}, nil
	}

	trimmed := string(raw)
	if _, ok := bailCommands[trimmed]; ok {
		return InputAction{Type: InputAbort}, nil
	}

	var f ClientFrame
	if err := json.Unmarshal(raw, &f); err != nil {
		mlog.Warning("agent: ignored non-JSON frame: ", trimmed)
		return InputAction{}, nil
	}

	switch f.Type {
	case FrameAwaitingFeedback:
		if _, ok := bailCommands[f.Feedback]; ok {
			return InputAction{Type: InputAbort}, nil
		}
		return InputAction{Type: InputContinue, Content: f.Feedback, RequestID: f.RequestID}, nil
	case FrameToolApprovalResp:
		return InputAction{Type: InputToolApprovalResponse, RequestID: f.RequestID, Approved: f.Approved}, nil
	case FrameSetAutoApprove:
		return InputAction{Type: InputSetAutoApprove, AutoApprove: f.Enabled}, nil
	default:
		mlog.Warning("agent: unknown client frame type=", f.Type)
		return InputAction{}, nil
	}
}
