package agent_test

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func expectFrame(t *testing.T, conn anyFrameReader, frameType string) agent.ServerFrame {
	t.Helper()
	var f agent.ServerFrame
	require.NoError(t, conn.ReadJSON(&f))
	require.Equal(t, frameType, f.Type, "expected frame type %q, got %q", frameType, f.Type)
	return f
}

type anyFrameReader interface {
	ReadJSON(v interface{}) error
}

func TestSession_Run_SingleTurnReply(t *testing.T) {
	serverConn, clientConn := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{"Hello back!", "TERMINATE"},
	}

	ws := &models.Workspace{ID: 1}
	sess := agent.NewSessionForTesting(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc)

	done := make(chan error, 1)
	go func() {
		done <- sess.Run("@agent hi")
	}()

	chat := expectFrame(t, clientConn, "")
	require.Equal(t, "@agent", chat.From)
	require.Equal(t, "USER", chat.To)
	require.Equal(t, "Hello back!", chat.Content)
	require.Equal(t, "success", chat.State)

	err := <-done
	require.NoError(t, err)
}

func TestSession_Run_ContextCancelStopsRunLoop(t *testing.T) {
	serverConn, _ := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{slowReply},
	}

	ctx, cancel := context.WithCancel(context.Background())
	ws := &models.Workspace{ID: 1}
	sess := agent.NewSessionForTesting(ctx, "test-uuid", ws, nil, mock, "You are helpful.", nil, wc)

	done := make(chan error, 1)
	go func() {
		done <- sess.Run("@agent hi")
	}()

	// Give Run a moment to start the mock
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-done
	require.ErrorIs(t, err, context.Canceled)
}

func TestSession_Run_InterruptAndContinue(t *testing.T) {
	serverConn, clientConn := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{"INTERRUPT", "Continuing now.", "TERMINATE"},
	}

	ws := &models.Workspace{ID: 1}
	sess := agent.NewSessionForTesting(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc)

	done := make(chan error, 1)
	go func() {
		done <- sess.Run("@agent hi")
	}()

	// First message is the user seed (muted), then INTERRUPT triggers OnInterrupt
	waiting := expectFrame(t, clientConn, agent.FrameWaitingOnInput)
	require.Contains(t, waiting.Question, "Provide feedback")

	// Send feedback to continue
	sess.Continue("go ahead", nil)

	chat := expectFrame(t, clientConn, "")
	require.Equal(t, "Continuing now.", chat.Content)

	err := <-done
	require.NoError(t, err)
}
