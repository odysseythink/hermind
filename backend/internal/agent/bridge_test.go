package agent_test

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestBridge_SetsCurrentMessageUUID(t *testing.T) {
	serverConn, clientConn := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{"Hello from agent!", "TERMINATE"},
	}
	ws := &models.Workspace{ID: 1}
	sess := agent.NewSessionForTesting(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, nil)

	done := make(chan error, 1)
	go func() { done <- sess.Run("@agent hi") }()

	var frame agent.ServerFrame
	require.NoError(t, clientConn.ReadJSON(&frame))
	require.NotEmpty(t, frame.UUID, "bridge should assign UUID to assistant frames")
	require.Equal(t, frame.UUID, sess.CurrentMessageUUID(), "session UUID should match frame UUID")
	require.Equal(t, "@agent", frame.From)

	<-done
}
