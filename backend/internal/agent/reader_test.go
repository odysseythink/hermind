package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func newPipedWSInternal(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	upgrader := websocket.Upgrader{}
	serverConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		serverConnCh <- conn
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	clientConn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientConn.Close() })

	serverConn := <-serverConnCh
	t.Cleanup(func() { _ = serverConn.Close() })
	return serverConn, clientConn
}

func TestReader_BailCommands_AbortSession(t *testing.T) {
	for _, cmd := range []string{"exit", "/exit", "stop", "/stop", "halt", "/halt", "/reset"} {
		t.Run(cmd, func(t *testing.T) {
			serverConn, clientConn := newPipedWSInternal(t)
			wc := newWSConn(serverConn)
			defer wc.Close()

			mock := &mockLanguageModel{
				provider: "mock", model: "mock-model",
				replies: []string{slowReply},
			}
			ws := &models.Workspace{ID: 1}
			sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)

			go sess.readerLoopWithInput(&wsInput{conn: wc})

			// wait for reader to start
			time.Sleep(10 * time.Millisecond)
			require.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(cmd)))

			// ctx should be cancelled within 1s
			select {
			case <-sess.ctx.Done():
			case <-time.After(time.Second):
				t.Fatal("context was not cancelled")
			}
		})
	}
}

func TestReader_AwaitingFeedback_ForwardsToSession(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{"INTERRUPT", "Continuing now.", "TERMINATE"},
	}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)

	go sess.readerLoopWithInput(&wsInput{conn: wc})

	done := make(chan error, 1)
	go func() {
		done <- sess.Run("@agent hi")
	}()

	// Wait for interrupt frame
	var waiting ServerFrame
	require.NoError(t, clientConn.ReadJSON(&waiting))
	require.Equal(t, FrameWaitingOnInput, waiting.Type)

	// Send feedback via clientConn
	require.NoError(t, clientConn.WriteJSON(ClientFrame{Type: FrameAwaitingFeedback, Feedback: "go ahead"}))

	// Should receive continuation chat
	var chat ServerFrame
	require.NoError(t, clientConn.ReadJSON(&chat))
	require.Equal(t, "Continuing now.", chat.Content)

	err := <-done
	require.NoError(t, err)
}

func TestReader_InvalidJSONIsIgnored(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{slowReply},
	}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)

	go sess.readerLoopWithInput(&wsInput{conn: wc})
	go func() { _ = sess.Run("@agent hi") }()

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte("not-json-at-all")))

	// Session should still be alive after sending garbage
	select {
	case <-sess.ctx.Done():
		t.Fatal("session should not be cancelled by invalid JSON")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestReader_BinaryFramesIgnored(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{
		provider: "mock", model: "mock-model",
		replies: []string{slowReply},
	}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)

	go sess.readerLoopWithInput(&wsInput{conn: wc})
	go func() { _ = sess.Run("@agent hi") }()

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, clientConn.WriteMessage(websocket.BinaryMessage, []byte("binary")))

	select {
	case <-sess.ctx.Done():
		t.Fatal("session should not be cancelled by binary frame")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestReader_ToolApprovalResp_WakesPendingApproval(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{slowReply}}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)

	go sess.readerLoopWithInput(&wsInput{conn: wc})

	// Fire an approval request in the background
	result := make(chan bool, 1)
	go func() {
		approved, _ := sess.RequestApproval(context.Background(), "skill", nil, "desc")
		result <- approved
	}()

	// Read the approval request frame
	var req ServerFrame
	require.NoError(t, clientConn.ReadJSON(&req))
	require.Equal(t, FrameToolApprovalReq, req.Type)

	// Send approval response
	require.NoError(t, clientConn.WriteJSON(ClientFrame{
		Type: FrameToolApprovalResp, RequestID: req.RequestID, Approved: true,
	}))

	require.True(t, <-result)
}

func TestReader_SetAutoApprove_TogglesSession(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{slowReply}}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)

	go sess.readerLoopWithInput(&wsInput{conn: wc})

	require.False(t, sess.AutoApprove())

	require.NoError(t, clientConn.WriteJSON(ClientFrame{Type: FrameSetAutoApprove, Enabled: true}))
	time.Sleep(50 * time.Millisecond)
	require.True(t, sess.AutoApprove())

	require.NoError(t, clientConn.WriteJSON(ClientFrame{Type: FrameSetAutoApprove, Enabled: false}))
	time.Sleep(50 * time.Millisecond)
	require.False(t, sess.AutoApprove())
}

func TestReader_ApprovalResp_UnknownRequestID_Ignored(t *testing.T) {
	serverConn, clientConn := newPipedWSInternal(t)
	wc := newWSConn(serverConn)
	defer wc.Close()

	mock := &mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{slowReply}}
	ws := &models.Workspace{ID: 1}
	sess := newSession(context.Background(), "test-uuid", ws, nil, mock, "You are helpful.", nil, wc, 2*time.Minute, nil)

	go sess.readerLoopWithInput(&wsInput{conn: wc})

	// Send approval response for unknown requestID — should not panic or deadlock
	require.NoError(t, clientConn.WriteJSON(ClientFrame{
		Type: FrameToolApprovalResp, RequestID: "unknown-id", Approved: true,
	}))

	// Session should still be alive
	select {
	case <-sess.ctx.Done():
		t.Fatal("session should not be cancelled by unknown approval response")
	case <-time.After(100 * time.Millisecond):
	}
}
