package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/odysseythink/hermind/gateway"
)

// Signal is an outbound-only adapter for signal-cli daemon running
// in HTTP / JSON-RPC 2.0 mode (`signal-cli daemon --http 127.0.0.1:8080`).
// Inbound is not implemented in Plan 7b.1 — the operator can stream
// the daemon's SSE endpoint externally and POST to api_server.
type Signal struct {
	BaseURL string // e.g. http://127.0.0.1:8080
	Account string // +1234567890
	client  *http.Client
}

func NewSignal(baseURL, account string) *Signal {
	return &Signal{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Account: account,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *Signal) Name() string { return "signal" }

func (s *Signal) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return nil
}

type signalRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
	ID      int            `json:"id"`
}

func (s *Signal) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if s.BaseURL == "" || s.Account == "" || out.ChatID == "" {
		return fmt.Errorf("signal: base_url/account/chat_id required")
	}
	req := signalRPCRequest{
		JSONRPC: "2.0",
		Method:  "send",
		Params: map[string]any{
			"account":   s.Account,
			"recipient": []string{out.ChatID},
			"message":   out.Text,
		},
		ID: 1,
	}
	buf, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", s.BaseURL, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("signal: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("signal: status %d: %s", resp.StatusCode, string(body))
	}
	// JSON-RPC errors come back embedded in a 200 response.
	if bytes.Contains(body, []byte(`"error"`)) {
		return fmt.Errorf("signal: rpc error: %s", string(body))
	}
	return nil
}
