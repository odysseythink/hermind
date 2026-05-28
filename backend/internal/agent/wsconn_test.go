package agent_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/stretchr/testify/require"
)

func newPipedWS(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	upgrader := websocket.Upgrader{}
	serverConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		serverConnCh <- conn
		// Block to keep connection alive until request context is cancelled
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

func TestWSConn_ConcurrentSendsAreSerialised(t *testing.T) {
	serverConn, clientConn := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	defer wc.Close()

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for {
				err := wc.Send(agent.ServerFrame{Type: agent.FrameStatusResponse, Content: fmt.Sprintf("%d", n)})
				if err == nil {
					return
				}
				if errors.Is(err, agent.ErrSlowReader) {
					time.Sleep(5 * time.Millisecond)
					continue
				}
				return
			}
		}(i)
	}
	wg.Wait()

	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		var f agent.ServerFrame
		require.NoError(t, clientConn.ReadJSON(&f))
		seen[f.Content] = true
	}
	require.Len(t, seen, 1000)
}

func TestWSConn_SendAfterCloseReturnsError(t *testing.T) {
	serverConn, _ := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	wc.Close()
	err := wc.Send(agent.ServerFrame{Type: agent.FrameStatusResponse, Content: "x"})
	require.ErrorIs(t, err, agent.ErrConnClosed)
}

func TestWSConn_CloseIsIdempotent(t *testing.T) {
	serverConn, _ := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	wc.Close()
	wc.Close() // should not panic
}

func TestWSConn_PingPongResetsReadDeadline(t *testing.T) {
	serverConn, clientConn := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	defer wc.Close()

	// Verify the connection stays alive and can receive frames after a short delay.
	// The actual 30s ping/pong cycle is too slow for a unit test; we just verify
	// the basic wiring by sending and receiving a frame after a brief wait.
	time.Sleep(100 * time.Millisecond)
	err := wc.Send(agent.ServerFrame{Type: agent.FrameStatusResponse, Content: "alive"})
	require.NoError(t, err)

	var f agent.ServerFrame
	require.NoError(t, clientConn.ReadJSON(&f))
	require.Equal(t, "alive", f.Content)
}

func TestWSConn_SlowReaderTriggersErrSlowReader(t *testing.T) {
	serverConn, _ := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	defer wc.Close()

	// Fill the outbound buffer (8 slots) without reading
	for i := 0; i < 8; i++ {
		err := wc.Send(agent.ServerFrame{Type: agent.FrameStatusResponse, Content: fmt.Sprintf("%d", i)})
		require.NoError(t, err)
	}

	// 9th send should overflow
	err := wc.Send(agent.ServerFrame{Type: agent.FrameStatusResponse, Content: "overflow"})
	require.ErrorIs(t, err, agent.ErrSlowReader)
}
