package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/require"
)

type mockEventLogger struct {
	mu     sync.Mutex
	events []mockEvent
}

type mockEvent struct {
	event    string
	metadata map[string]any
	userID   *int
}

func (m *mockEventLogger) LogEvent(ctx context.Context, event string, metadata map[string]any, userID *int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, mockEvent{event: event, metadata: metadata, userID: userID})
	return nil
}

func (m *mockEventLogger) Events() []mockEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]mockEvent, len(m.events))
	copy(out, m.events)
	return out
}

func TestTelemetry_ChatSent_FiresOnEachNonUserMessage(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{"Hello!", "TERMINATE"}}
	ws := &models.Workspace{ID: 1}
	mel := &mockEventLogger{}
	sess := newSession(context.Background(), "sess-1", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, mel)

	done := make(chan error, 1)
	go func() { done <- sess.Run("@agent hi") }()

	// Read the chat frame so OnMessage has fired
	var chat ServerFrame
	require.NoError(t, clientConn.ReadJSON(&chat))
	require.Equal(t, "@agent", chat.From)

	<-done

	require.Eventually(t, func() bool {
		return len(mel.Events()) >= 1
	}, time.Second, 10*time.Millisecond)

	events := mel.Events()
	require.Equal(t, "agent_chat_sent", events[0].event)
	require.Equal(t, "@agent", events[0].metadata["from"])
	require.Equal(t, "USER", events[0].metadata["to"])
}

func TestTelemetry_ChatSent_NotFiredForUserMessages(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	// The first reply from the mock model is the user seed (muted).
	// We use INTERRUPT so the USER participant sends a message.
	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{"INTERRUPT", "Continuing now.", "TERMINATE"}}
	ws := &models.Workspace{ID: 1}
	mel := &mockEventLogger{}
	sess := newSession(context.Background(), "sess-2", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, mel)

	done := make(chan error, 1)
	go func() { done <- sess.Run("@agent hi") }()

	// Wait for interrupt frame
	var waiting ServerFrame
	require.NoError(t, clientConn.ReadJSON(&waiting))
	require.Equal(t, FrameWaitingOnInput, waiting.Type)

	// Send feedback to continue
	sess.Continue("go ahead", nil)

	// Read continuation chat
	var chat ServerFrame
	require.NoError(t, clientConn.ReadJSON(&chat))
	require.Equal(t, "Continuing now.", chat.Content)

	<-done

	// Only non-USER messages should produce events; USER seed is muted and
	// the feedback continue is also USER → no events.
	require.Eventually(t, func() bool {
		return len(mel.Events()) >= 1
	}, time.Second, 10*time.Millisecond)

	for _, e := range mel.Events() {
		require.Equal(t, "agent_chat_sent", e.event)
		require.NotEqual(t, participantUser, e.metadata["from"])
	}
}

func TestTelemetry_ChatStarted_FiresDirectly(t *testing.T) {
	mel := &mockEventLogger{}
	logChatStarted(mel, nil, "sess-3", 7, "openai", "gpt-4o-mini")

	require.Eventually(t, func() bool {
		return len(mel.Events()) == 1
	}, time.Second, 10*time.Millisecond)

	events := mel.Events()
	require.Equal(t, "agent_chat_started", events[0].event)
	require.Equal(t, "sess-3", events[0].metadata["session_uuid"])
	require.Equal(t, 7, events[0].metadata["workspace_id"])
	require.Equal(t, "openai", events[0].metadata["provider"])
	require.Equal(t, "gpt-4o-mini", events[0].metadata["model"])
}

func TestTelemetry_ChatTerminated_FiresDirectly(t *testing.T) {
	mel := &mockEventLogger{}
	logChatTerminated(mel, nil, "sess-4", "normal", 5*time.Second)

	require.Eventually(t, func() bool {
		return len(mel.Events()) == 1
	}, time.Second, 10*time.Millisecond)

	events := mel.Events()
	require.Equal(t, "agent_chat_terminated", events[0].event)
	require.Equal(t, "normal", events[0].metadata["reason"])
	require.Equal(t, int64(5000), events[0].metadata["duration_ms"])
}

func TestTelemetry_ChatTerminated_FiresOnAbortReason(t *testing.T) {
	mel := &mockEventLogger{}
	logChatTerminated(mel, nil, "sess-5", "cancelled", 1*time.Second)

	require.Eventually(t, func() bool {
		return len(mel.Events()) == 1
	}, time.Second, 10*time.Millisecond)

	events := mel.Events()
	require.Equal(t, "cancelled", events[0].metadata["reason"])
}

func TestTelemetry_FireAndForget_DoesNotBlockOnSlowEventLog(t *testing.T) {
	mel := &slowEventLogger{delay: 3 * time.Second}
	start := time.Now()
	logChatStarted(mel, nil, "sess-6", 1, "mock", "mock")
	elapsed := time.Since(start)

	// The function should return immediately (<50ms) even though the
	// background goroutine sleeps for 3s.
	require.Less(t, elapsed, 100*time.Millisecond, "telemetry should not block on slow event log")
}

func TestTelemetry_NilEventLog_DoesNotPanic(t *testing.T) {
	// Direct calls with nil should not panic
	logChatStarted(nil, nil, "x", 1, "a", "b")
	logChatSent(nil, nil, "x", "a", "b")
	logChatTerminated(nil, nil, "x", "normal", 1*time.Second)
}

type slowEventLogger struct {
	delay time.Duration
}

func (s *slowEventLogger) LogEvent(ctx context.Context, event string, metadata map[string]any, userID *int) error {
	time.Sleep(s.delay)
	return nil
}
