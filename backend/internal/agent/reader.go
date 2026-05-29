package agent

import (
	"github.com/odysseythink/mlog"
)

var bailCommands = map[string]struct{}{
	"exit": {}, "/exit": {}, "stop": {}, "/stop": {}, "halt": {}, "/halt": {}, "/reset": {},
}

// readerLoopWithInput runs in a goroutine until Session ctx is cancelled or input errors.
func (s *Session) readerLoopWithInput(input AgentInput) {
	defer func() {
		if r := recover(); r != nil {
			mlog.Error("agent reader panic: ", r)
			s.Abort("internal reader panic")
		}
	}()
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		action, err := input.Read(s.ctx)
		if err != nil {
			s.cancel()
			return
		}

		switch action.Type {
		case InputAbort:
			s.Abort("")
			return
		case InputContinue:
			s.Continue(action.Content, nil)
		case InputToolApprovalResponse:
			if action.RequestID == "" {
				mlog.Warning("agent: toolApprovalResponse with empty requestId")
				continue
			}
			s.handleApprovalResponse(action.RequestID, action.Approved)
		case InputSetAutoApprove:
			s.SetAutoApprove(action.AutoApprove)
			mlog.Info("agent: setAutoApprove → ", action.AutoApprove)
		}
	}
}
