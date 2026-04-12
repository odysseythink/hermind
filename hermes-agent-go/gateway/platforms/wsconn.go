package platforms

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// WSConnConfig configures a managed WebSocket connection.
type WSConnConfig struct {
	URL             string
	Headers         map[string]string
	OnMessage       func(data []byte)
	OnConnect       func(conn *WSConn) error
	ReconnectBase   time.Duration // default 2s
	ReconnectMax    time.Duration // default 60s
	ReconnectJitter float64       // 0.0–1.0, default 0.2
}

// WSConn is a managed WebSocket connection with automatic reconnection.
type WSConn struct {
	cfg  WSConnConfig
	mu   sync.Mutex
	conn *websocket.Conn
}

func NewWSConn(cfg WSConnConfig) *WSConn {
	if cfg.ReconnectBase == 0 {
		cfg.ReconnectBase = 2 * time.Second
	}
	if cfg.ReconnectMax == 0 {
		cfg.ReconnectMax = 60 * time.Second
	}
	if cfg.ReconnectJitter == 0 {
		cfg.ReconnectJitter = 0.2
	}
	return &WSConn{cfg: cfg}
}

// WriteJSON marshals v as JSON and sends it as a text frame. Thread-safe.
func (w *WSConn) WriteJSON(ctx context.Context, v any) error {
	w.mu.Lock()
	c := w.conn
	w.mu.Unlock()
	if c == nil {
		return context.Canceled
	}
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, buf)
}

// Run connects and reads messages until ctx is cancelled.
// On disconnect it reconnects with exponential backoff.
func (w *WSConn) Run(ctx context.Context) error {
	backoff := w.cfg.ReconnectBase
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := w.connectAndRead(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("wsconn: disconnected, reconnecting",
			"url", w.cfg.URL, "err", err, "backoff", backoff)
		jittered := w.jitter(backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jittered):
		}
		backoff = min(backoff*2, w.cfg.ReconnectMax)
	}
}

func (w *WSConn) connectAndRead(ctx context.Context) error {
	opts := &websocket.DialOptions{}
	if len(w.cfg.Headers) > 0 {
		h := make(http.Header)
		for k, v := range w.cfg.Headers {
			h.Set(k, v)
		}
		opts.HTTPHeader = h
	}
	c, _, err := websocket.Dial(ctx, w.cfg.URL, opts)
	if err != nil {
		return err
	}
	c.SetReadLimit(1 << 20) // 1MB

	w.mu.Lock()
	w.conn = c
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.conn = nil
		w.mu.Unlock()
		c.CloseNow()
	}()

	if w.cfg.OnConnect != nil {
		if err := w.cfg.OnConnect(w); err != nil {
			return err
		}
	}

	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return err
		}
		if w.cfg.OnMessage != nil {
			w.cfg.OnMessage(data)
		}
	}
}

func (w *WSConn) jitter(d time.Duration) time.Duration {
	if w.cfg.ReconnectJitter <= 0 {
		return d
	}
	j := float64(d) * w.cfg.ReconnectJitter
	return d + time.Duration((rand.Float64()*2-1)*j)
}
