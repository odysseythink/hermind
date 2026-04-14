package platforms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestWSConnReadLoop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer c.CloseNow()
		msg := map[string]string{"type": "hello"}
		buf, _ := json.Marshal(msg)
		_ = c.Write(r.Context(), websocket.MessageText, buf)
		c.Close(websocket.StatusNormalClosure, "done")
	}))
	defer srv.Close()

	var received int32
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wsURL := "ws" + srv.URL[len("http"):]
	conn := NewWSConn(WSConnConfig{
		URL:             wsURL,
		OnMessage:       func(data []byte) { atomic.AddInt32(&received, 1) },
		ReconnectBase:   50 * time.Millisecond,
		ReconnectMax:    200 * time.Millisecond,
		ReconnectJitter: 0,
	})

	err := conn.Run(ctx)
	_ = err

	if atomic.LoadInt32(&received) < 1 {
		t.Errorf("received = %d, want >= 1", atomic.LoadInt32(&received))
	}
}

func TestWSConnWriteJSON(t *testing.T) {
	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		_, data, err := c.Read(r.Context())
		if err != nil {
			return
		}
		var msg map[string]string
		_ = json.Unmarshal(data, &msg)
		if msg["type"] == "ping" {
			atomic.AddInt32(&received, 1)
		}
		c.Close(websocket.StatusNormalClosure, "done")
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wsURL := "ws" + srv.URL[len("http"):]
	connected := make(chan struct{})
	var once sync.Once
	conn := NewWSConn(WSConnConfig{
		URL: wsURL,
		OnConnect: func(c *WSConn) error {
			once.Do(func() { close(connected) })
			return nil
		},
		OnMessage:       func(data []byte) {},
		ReconnectBase:   50 * time.Millisecond,
		ReconnectMax:    200 * time.Millisecond,
		ReconnectJitter: 0,
	})

	go conn.Run(ctx)

	select {
	case <-connected:
	case <-ctx.Done():
		t.Fatal("never connected")
	}

	err := conn.WriteJSON(ctx, map[string]string{"type": "ping"})
	if err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	cancel()

	if atomic.LoadInt32(&received) != 1 {
		t.Errorf("server received = %d, want 1", atomic.LoadInt32(&received))
	}
}
