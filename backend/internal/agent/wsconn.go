package agent

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsOutboundBuffer = 8
	wsWriteTimeout   = 30 * time.Second
	wsPingInterval   = 30 * time.Second
	wsReadDeadline   = 5 * time.Minute
)

var (
	ErrSlowReader = errors.New("ws outbound buffer full (slow reader)")
	ErrConnClosed = errors.New("ws connection closed")
)

type wsConn struct {
	conn       *websocket.Conn
	outbound   chan ServerFrame
	done       chan struct{}
	closeOnce  sync.Once
	err        error
	writerDone chan struct{}
}

func newWSConn(conn *websocket.Conn) *wsConn {
	wc := &wsConn{
		conn:       conn,
		outbound:   make(chan ServerFrame, wsOutboundBuffer),
		done:       make(chan struct{}),
		writerDone: make(chan struct{}),
	}
	go wc.writerLoop()
	_ = wc.conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
	wc.conn.SetPongHandler(func(string) error {
		return wc.conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
	})
	return wc
}

// NewWSConnForTesting wraps a raw conn; test-only.
func NewWSConnForTesting(conn *websocket.Conn) *wsConn { return newWSConn(conn) }

func (w *wsConn) Send(f ServerFrame) error {
	select {
	case <-w.done:
		return ErrConnClosed
	default:
	}
	select {
	case <-w.done:
		return ErrConnClosed
	case w.outbound <- f:
		return nil
	default:
		return ErrSlowReader
	}
}

func (w *wsConn) Close() error {
	w.closeOnce.Do(func() {
		close(w.done)
		// wait briefly for writer to drain, then force-close conn
		select {
		case <-w.writerDone:
		case <-time.After(2 * time.Second):
		}
		_ = w.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second))
		_ = w.conn.Close()
	})
	return nil
}

func (w *wsConn) ReadMessage() (int, []byte, error) {
	return w.conn.ReadMessage()
}

func (w *wsConn) writerLoop() {
	defer close(w.writerDone)
	pingTicker := time.NewTicker(wsPingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-w.done:
			// Drain remaining frames before exiting
			for {
				select {
				case f := <-w.outbound:
					_ = w.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
					_ = w.conn.WriteJSON(f)
				default:
					return
				}
			}
		case <-pingTicker.C:
			_ = w.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := w.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				w.err = fmt.Errorf("ping: %w", err)
				return
			}
		case f := <-w.outbound:
			_ = w.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := w.conn.WriteJSON(f); err != nil {
				w.err = fmt.Errorf("write json: %w", err)
				return
			}
		}
	}
}
