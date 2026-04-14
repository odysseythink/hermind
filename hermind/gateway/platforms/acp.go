package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/odysseythink/hermind/gateway"
)

// ACP is a minimal Agent Communication Protocol adapter. It exposes
// a POST /acp/messages endpoint that accepts an ACP message envelope
// and returns the assistant's reply synchronously.
//
// Plan 8 subset of the ACP message shape:
//
//	{"session_id":"...", "parts":[{"type":"text","text":"..."}]}
type ACP struct {
	addr  string
	token string
	srv   *http.Server
}

func NewACP(addr, token string) *ACP {
	if addr == "" {
		addr = ":8081"
	}
	return &ACP{addr: addr, token: token}
}

func (a *ACP) Name() string { return "acp" }

type acpPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type acpRequest struct {
	SessionID string    `json:"session_id"`
	Parts     []acpPart `json:"parts"`
}

type acpResponse struct {
	SessionID string    `json:"session_id"`
	Parts     []acpPart `json:"parts"`
}

func (a *ACP) Run(ctx context.Context, handler gateway.MessageHandler) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/acp/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if a.token != "" && r.Header.Get("Authorization") != "Bearer "+a.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var req acpRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		text := ""
		for _, p := range req.Parts {
			if p.Type == "text" {
				text += p.Text
			}
		}
		if text == "" {
			http.Error(w, "no text parts", http.StatusBadRequest)
			return
		}
		in := gateway.IncomingMessage{
			Platform: a.Name(),
			UserID:   req.SessionID,
			ChatID:   req.SessionID,
			Text:     text,
		}
		out, err := handler(r.Context(), in)
		if err != nil {
			http.Error(w, "handler error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		reply := acpResponse{SessionID: req.SessionID, Parts: []acpPart{}}
		if out != nil {
			reply.Parts = append(reply.Parts, acpPart{Type: "text", Text: out.Text})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(reply)
	})

	srv := &http.Server{Addr: a.addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	a.srv = srv

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
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
		return fmt.Errorf("acp: %w", err)
	}
}

func (a *ACP) SendReply(context.Context, gateway.OutgoingMessage) error { return nil }
