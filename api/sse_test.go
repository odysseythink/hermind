package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSSE_ForwardsHubEvents(t *testing.T) {
	s := newTestServer(t)
	httpSrv := httptest.NewServer(s.Router())
	defer httpSrv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", httpSrv.URL+"/api/sessions/sess-sse/stream/sse?t=t", nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Errorf("content-type = %q", got)
	}

	// Publish in a background goroutine until the client actually
	// subscribes. Reading `data:` off the stream is our signal.
	go func() {
		for i := 0; i < 50; i++ {
			s.Streams().Publish(StreamEvent{
				Type:      EventTypeStatus,
				SessionID: "sess-sse",
				Data:      "started",
			})
			time.Sleep(25 * time.Millisecond)
		}
	}()

	reader := bufio.NewReader(resp.Body)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if strings.HasPrefix(line, "data: ") {
			if !strings.Contains(line, `"type":"status"`) {
				t.Errorf("unexpected data line: %q", line)
			}
			return
		}
	}
	t.Fatal("no data frame received")
}

func TestSSE_RejectsMissingToken(t *testing.T) {
	s := newTestServer(t)
	httpSrv := httptest.NewServer(s.Router())
	defer httpSrv.Close()

	req, _ := http.NewRequest("GET", httpSrv.URL+"/api/sessions/sess-x/stream/sse", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}
