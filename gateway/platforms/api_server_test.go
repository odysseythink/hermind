package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/odysseythink/hermind/gateway"
)

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestAPIServerRoundTrip(t *testing.T) {
	addr := freePort(t)
	srv := NewAPIServer(addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			return &gateway.OutgoingMessage{UserID: in.UserID, Text: "echo: " + in.Text}, nil
		})
	}()

	// Wait up to 500ms for the server to accept.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("tcp", addr); err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	body := `{"user_id":"u1","text":"ping"}`
	req, _ := http.NewRequest("POST", "http://"+addr+"/message", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var out apiReply
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(out.Reply, "echo: ping") {
		t.Errorf("unexpected reply: %q", out.Reply)
	}

	cancel()
	<-errCh
}
