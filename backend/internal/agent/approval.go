package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/mlog"
)

type approvalResp struct {
	approved bool
	reason   string
}

// RequestApproval blocks until the user approves/rejects the tool call,
// the request times out, or the session context is cancelled.
//
// skillName is the tool name (e.g., "gmail-send"); args is the marshalled
// tool arguments; description is a human-readable explanation shown to the
// user. Returns (approved, reason). On non-approval, reason is populated.
func (s *Session) RequestApproval(ctx context.Context, skillName string, args any, description string) (bool, string) {
	if s.autoApprove.Load() {
		return true, "auto-approved (session toggle)"
	}
	requestID := uuid.NewString()
	ch := make(chan approvalResp, 1)
	s.approvalsMu.Lock()
	s.approvals[requestID] = ch
	s.approvalsMu.Unlock()
	defer func() {
		s.approvalsMu.Lock()
		delete(s.approvals, requestID)
		s.approvalsMu.Unlock()
	}()

	ttl := s.approvalTTL
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	if err := s.wsConn.Send(ServerFrame{
		Type:        FrameToolApprovalReq,
		RequestID:   requestID,
		SkillName:   skillName,
		Payload:     args,
		Description: description,
		TimeoutMs:   int(ttl / time.Millisecond),
	}); err != nil {
		mlog.Warning("agent: approval request send failed: ", err)
		return false, "approval request could not be delivered"
	}

	select {
	case resp := <-ch:
		return resp.approved, resp.reason
	case <-time.After(ttl):
		return false, fmt.Sprintf("approval timed out after %s", ttl)
	case <-ctx.Done():
		return false, "approval cancelled (context cancelled)"
	case <-s.ctx.Done():
		return false, "approval cancelled (session ended)"
	}
}

// handleApprovalResponse routes a toolApprovalResponse client frame to its waiting goroutine.
// Unknown requestIDs are logged and dropped (idempotent — clients may retry).
func (s *Session) handleApprovalResponse(requestID string, approved bool) {
	s.approvalsMu.Lock()
	ch, ok := s.approvals[requestID]
	s.approvalsMu.Unlock()
	if !ok {
		mlog.Info("agent: unknown approval requestID: ", requestID)
		return
	}
	reason := "approved by user"
	if !approved {
		reason = "rejected by user"
	}
	// Non-blocking send — chan is buffered 1; second response (unlikely) is dropped.
	select {
	case ch <- approvalResp{approved: approved, reason: reason}:
	default:
		mlog.Warning("agent: approval channel full (duplicate response?)")
	}
}

// cancelAllApprovals delivers a synthetic rejection to all pending approvals.
// Used by Abort + Shutdown.
func (s *Session) cancelAllApprovals(reason string) {
	s.approvalsMu.Lock()
	defer s.approvalsMu.Unlock()
	for _, ch := range s.approvals {
		select {
		case ch <- approvalResp{approved: false, reason: reason}:
		default:
		}
	}
	s.approvals = make(map[string]chan approvalResp)
}

// SetAutoApprove toggles per-session auto-approval. Called from reader on `setAutoApprove` client frame.
func (s *Session) SetAutoApprove(b bool) {
	s.autoApprove.Store(b)
}

// AutoApprove returns the current per-session auto-approve state. Test-only.
func (s *Session) AutoApprove() bool {
	return s.autoApprove.Load()
}

// PendingApprovalCount returns the number of in-flight approvals. Test-only.
func (s *Session) PendingApprovalCount() int {
	s.approvalsMu.Lock()
	defer s.approvalsMu.Unlock()
	return len(s.approvals)
}
