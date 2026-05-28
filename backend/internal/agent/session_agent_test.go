package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/require"
)

func TestSession_UsesAgentNotRawModel(t *testing.T) {
	serverConn, _ := newPipedWSInternal(t)
	wc := NewWSConnForTesting(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{"Hello!", "TERMINATE"},
	}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", tool.NewRegistry(), wc, 2*time.Minute, nil)

	require.NotNil(t, sess.pAgent, "Participant.Agent should be set")
	// The conversation participant was registered with Agent set and Model left zero.
	// We verify behaviourally via TestSession_EmptyRegistry_StillReplies.
}

func TestSession_EmptyRegistry_StillReplies(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := NewWSConnForTesting(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{"Hello back!", "TERMINATE"},
	}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", tool.NewRegistry(), wc, 2*time.Minute, nil)

	done := make(chan error, 1)
	go func() {
		done <- sess.Run("@agent hi")
	}()

	var chat ServerFrame
	require.NoError(t, clientConn.ReadJSON(&chat))
	require.Equal(t, "@agent", chat.From)
	require.Equal(t, "USER", chat.To)
	require.Equal(t, "Hello back!", chat.Content)
	require.Equal(t, "success", chat.State)

	err := <-done
	require.NoError(t, err)
}

func TestSession_AgentMaxStepsIs10(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := NewWSConnForTesting(serverConn)
	defer wc.Close()

	// A mock that always returns a ToolCallPart so the agent loop never
	// breaks naturally. After 10 steps it should error with "max steps".
	parts := make([][]core.ContentParter, 11)
	for i := range parts {
		parts[i] = []core.ContentParter{
			core.ToolCallPart{ID: fmt.Sprintf("%d", i), Name: "rag-memory", Arguments: `{"action":"search","content":"x"}`},
		}
	}
	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		parts: parts,
	}
	ws := &models.Workspace{ID: 1}
	reg := tool.NewRegistry()
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", reg, wc, 2*time.Minute, nil)

	done := make(chan error, 1)
	go func() {
		done <- sess.Run("@agent do something")
	}()

	// Wait for the failure frame
	var failure ServerFrame
	require.NoError(t, clientConn.ReadJSON(&failure))
	require.Equal(t, FrameWSSFailure, failure.Type)
	require.Contains(t, failure.Content, "max steps")

	err := <-done
	require.Error(t, err)
	require.Contains(t, err.Error(), "max steps")
}
