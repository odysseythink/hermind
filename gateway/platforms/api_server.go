// Package platforms contains platform adapters for the gateway.
package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/odysseythink/hermind/gateway"
)

// APIServer is a simple HTTP JSON adapter. POST /message with
// {"user_id":"...","text":"..."} returns the reply synchronously.
type APIServer struct {
	addr string
	mu   sync.Mutex
	srv  *http.Server
}

// NewAPIServer builds an APIServer listening on addr (e.g. ":8080").
func NewAPIServer(addr string) *APIServer {
	if addr == "" {
		addr = ":8080"
	}
	return &APIServer{addr: addr}
}

func (a *APIServer) Name() string { return "api_server" }

type apiRequest struct {
	UserID string `json:"user_id"`
	ChatID string `json:"chat_id,omitempty"`
	Text   string `json:"text"`
}

type apiReply struct {
	Reply string `json:"reply"`
}

func (a *APIServer) Run(ctx context.Context, handler gateway.MessageHandler) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var req apiRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.UserID == "" || req.Text == "" {
			http.Error(w, "user_id and text are required", http.StatusBadRequest)
			return
		}
		in := gateway.IncomingMessage{
			Platform: a.Name(),
			UserID:   req.UserID,
			ChatID:   req.ChatID,
			Text:     req.Text,
		}
		out, err := handler(r.Context(), in)
		if err != nil {
			http.Error(w, "handler error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		text := ""
		if out != nil {
			text = out.Text
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(apiReply{Reply: text})
	})

	srv := &http.Server{Addr: a.addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	a.mu.Lock()
	a.srv = srv
	a.mu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("api_server: %w", err)
	}
}

// SendReply is unused — the api_server replies inline via the HTTP
// response writer. Implemented to satisfy the interface.
func (a *APIServer) SendReply(context.Context, gateway.OutgoingMessage) error { return nil }
