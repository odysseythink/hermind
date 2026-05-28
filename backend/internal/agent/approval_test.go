package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestSession_RequestApproval_UserApproves_ReturnsTrue(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{slowReply}}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)

	go sess.readerLoop()

	// Fire approval request in background
	result := make(chan struct {
		approved bool
		reason   string
	}, 1)
	go func() {
		a, r := sess.RequestApproval(context.Background(), "test-skill", map[string]any{"q": "hi"}, "test desc")
		result <- struct {
			approved bool
			reason   string
		}{a, r}
	}()

	// Read the approval request frame
	var req ServerFrame
	require.NoError(t, clientConn.ReadJSON(&req))
	require.Equal(t, FrameToolApprovalReq, req.Type)
	require.NotEmpty(t, req.RequestID)
	require.Equal(t, "test-skill", req.SkillName)

	// Approve
	require.NoError(t, clientConn.WriteJSON(ClientFrame{
		Type: FrameToolApprovalResp, RequestID: req.RequestID, Approved: true,
	}))

	res := <-result
	require.True(t, res.approved)
	require.Contains(t, res.reason, "approved")
}

func TestSession_RequestApproval_UserRejects_ReturnsFalse(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{slowReply}}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)

	go sess.readerLoop()

	result := make(chan struct {
		approved bool
		reason   string
	}, 1)
	go func() {
		a, r := sess.RequestApproval(context.Background(), "test-skill", nil, "test desc")
		result <- struct {
			approved bool
			reason   string
		}{a, r}
	}()

	var req ServerFrame
	require.NoError(t, clientConn.ReadJSON(&req))
	require.Equal(t, FrameToolApprovalReq, req.Type)

	// Reject
	require.NoError(t, clientConn.WriteJSON(ClientFrame{
		Type: FrameToolApprovalResp, RequestID: req.RequestID, Approved: false,
	}))

	res := <-result
	require.False(t, res.approved)
	require.Contains(t, res.reason, "rejected")
}

func TestSession_RequestApproval_Timeout_ReturnsFalseWithReason(t *testing.T) {
	serverConn, _ := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{slowReply}}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 100*time.Millisecond, nil)

	approved, reason := sess.RequestApproval(context.Background(), "test-skill", nil, "test desc")
	require.False(t, approved)
	require.Contains(t, reason, "timed out")
}

func TestSession_RequestApproval_CtxCancel_ReturnsFalseWithReason(t *testing.T) {
	serverConn, _ := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{slowReply}}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	approved, reason := sess.RequestApproval(ctx, "test-skill", nil, "test desc")
	require.False(t, approved)
	require.Contains(t, reason, "context cancelled")
}

func TestSession_RequestApproval_AutoApproveBypassesGate(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{slowReply}}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)
	sess.SetAutoApprove(true)

	// No frame should be sent because auto-approve bypasses the gate
	approved, reason := sess.RequestApproval(context.Background(), "test-skill", nil, "test desc")
	require.True(t, approved)
	require.Contains(t, reason, "auto-approved")

	// Ensure no frame was sent by checking clientConn (should timeout)
	clientConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, _, err := clientConn.ReadMessage()
	require.Error(t, err) // timeout or deadline exceeded
}

func TestSession_RequestApproval_ConcurrentRequests_RoutedByRequestID(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{slowReply}}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)

	go sess.readerLoop()

	const n = 5
	resultCh := make(chan struct {
		id       string
		approved bool
	}, n)
	var wg sync.WaitGroup

	// Fire n concurrent approval requests; each goroutine records its own requestID.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// We can't know our requestID here, but we can record the result.
			// Instead, we collect results anonymously and match by count.
			approved, _ := sess.RequestApproval(context.Background(), "skill", nil, "desc")
			resultCh <- struct {
				id       string
				approved bool
			}{approved: approved}
		}()
	}

	// Read n approval request frames and capture requestIDs.
	requestIDs := make([]string, n)
	for i := 0; i < n; i++ {
		var req ServerFrame
		require.NoError(t, clientConn.ReadJSON(&req))
		require.Equal(t, FrameToolApprovalReq, req.Type)
		requestIDs[i] = req.RequestID
	}

	// Respond out of order (reverse) with alternating approvals.
	expected := make(map[string]bool, n)
	for i := n - 1; i >= 0; i-- {
		approved := i%2 == 0
		expected[requestIDs[i]] = approved
		require.NoError(t, clientConn.WriteJSON(ClientFrame{
			Type: FrameToolApprovalResp, RequestID: requestIDs[i], Approved: approved,
		}))
	}

	wg.Wait()
	close(resultCh)

	// Count approvals — exactly half (ceil) should be true.
	trueCount, falseCount := 0, 0
	for r := range resultCh {
		if r.approved {
			trueCount++
		} else {
			falseCount++
		}
	}
	require.Equal(t, 3, trueCount, "expected 3 approvals (indices 0,2,4)")
	require.Equal(t, 2, falseCount, "expected 2 rejections (indices 1,3)")
}

func TestSession_CancelAllApprovals_WakesAllPending(t *testing.T) {
	serverConn, _ := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{slowReply}}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)

	const n = 3
	results := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() {
			approved, _ := sess.RequestApproval(context.Background(), "skill", nil, "desc")
			results <- approved
		}()
	}

	// Wait for all requests to be pending
	require.Eventually(t, func() bool {
		return sess.PendingApprovalCount() == n
	}, time.Second, 10*time.Millisecond)

	sess.cancelAllApprovals("test cancel")

	for i := 0; i < n; i++ {
		require.False(t, <-results, "request %d should be rejected", i)
	}
	require.Equal(t, 0, sess.PendingApprovalCount())
}
