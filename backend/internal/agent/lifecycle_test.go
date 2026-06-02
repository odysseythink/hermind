package agent

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestRuntime_Shutdown_CancelsPendingApprovals(t *testing.T) {
	serverConn, _ := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{slowReply}}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil, nil)

	// Start an approval request in the background
	result := make(chan bool, 1)
	go func() {
		approved, _ := sess.RequestApproval(context.Background(), "skill", nil, "desc")
		result <- approved
	}()

	// Wait for the request to be pending
	require.Eventually(t, func() bool {
		return sess.PendingApprovalCount() == 1
	}, time.Second, 10*time.Millisecond)

	// Shutdown should cancel the approval
	sess.cancelAllApprovals("server shutting down")

	select {
	case approved := <-result:
		require.False(t, approved)
	case <-time.After(time.Second):
		t.Fatal("approval was not cancelled by shutdown")
	}
}

func TestRuntime_Shutdown_MultipleSessions_EachDrained(t *testing.T) {
	// Create 3 sessions, each with a pending approval
	sessions := make([]*Session, 3)
	results := make([]chan bool, 3)

	for i := 0; i < 3; i++ {
		serverConn, _ := newPipedWSInternal(t)
		wc := newWSConn(serverConn)
		defer wc.Close()

		mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{slowReply}}
		ws := &models.Workspace{ID: i + 1}
		sess := newSession(context.Background(), "test-uuid-"+string(rune('a'+i)), ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil, nil)
		sessions[i] = sess

		results[i] = make(chan bool, 1)
		go func(idx int) {
			approved, _ := sess.RequestApproval(context.Background(), "skill", nil, "desc")
			results[idx] <- approved
		}(i)
	}

	// Wait for all to be pending
	for i := 0; i < 3; i++ {
		require.Eventually(t, func() bool {
			return sessions[i].PendingApprovalCount() == 1
		}, time.Second, 10*time.Millisecond)
	}

	// Shutdown all
	for i := 0; i < 3; i++ {
		sessions[i].cancelAllApprovals("server shutting down")
	}

	for i := 0; i < 3; i++ {
		select {
		case approved := <-results[i]:
			require.False(t, approved, "session %d approval should be cancelled", i)
		case <-time.After(time.Second):
			t.Fatalf("session %d approval was not cancelled", i)
		}
	}
}
