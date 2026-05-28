package agent

import (
	"encoding/json"

	"github.com/gorilla/websocket"
	"github.com/odysseythink/mlog"
)

var bailCommands = map[string]struct{}{
	"exit": {}, "/exit": {}, "stop": {}, "/stop": {}, "halt": {}, "/halt": {}, "/reset": {},
}

// readerLoop runs in a goroutine until Session ctx is cancelled or the conn errors.
func (s *Session) readerLoop() {
	defer func() {
		if r := recover(); r != nil {
			mlog.Error("agent reader panic: ", r)
			s.Abort("internal reader panic")
		}
	}()
	for {
		mt, raw, err := s.wsConn.ReadMessage()
		if err != nil {
			s.cancel()
			return
		}
		if mt != websocket.TextMessage {
			continue
		}

		// 1. bare bail-command string
		trimmed := string(raw)
		if _, ok := bailCommands[trimmed]; ok {
			s.Abort("")
			return
		}

		// 2. JSON frame
		var f ClientFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			mlog.Warning("agent: ignored non-JSON frame: ", trimmed)
			continue
		}
		switch f.Type {
		case FrameAwaitingFeedback:
			if _, ok := bailCommands[f.Feedback]; ok {
				s.Abort("")
				return
			}
			s.Continue(f.Feedback, f.Attachments)
		case FrameToolApprovalResp:
			if f.RequestID == "" {
				mlog.Warning("agent: toolApprovalResponse with empty requestId")
				continue
			}
			s.handleApprovalResponse(f.RequestID, f.Approved)
		case FrameSetAutoApprove:
			s.SetAutoApprove(f.Enabled)
			mlog.Info("agent: setAutoApprove → ", f.Enabled)
		default:
			mlog.Warning("agent: unknown client frame type=", f.Type)
		}
	}
}
