package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/odysseythink/hermind/gateway"
)

func TestACPRoundTripWithAuth(t *testing.T) {
	addr := freePort(t)
	srv := NewACP(addr, "tok")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			return &gateway.OutgoingMessage{Text: "echo: " + in.Text}, nil
		})
	}()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("tcp", addr); err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	body := `{"session_id":"s1","parts":[{"type":"text","text":"hello"}]}`
	req, _ := http.NewRequest("POST", "http://"+addr+"/acp/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var out acpResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Parts) != 1 || out.Parts[0].Text != "echo: hello" {
		t.Errorf("unexpected parts: %+v", out.Parts)
	}

	// Missing auth should 401
	reqNoAuth, _ := http.NewRequest("POST", "http://"+addr+"/acp/messages", bytes.NewBufferString(body))
	reqNoAuth.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(reqNoAuth)
	if err != nil {
		t.Fatalf("post2: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp2.StatusCode)
	}

	cancel()
	<-errCh
}
