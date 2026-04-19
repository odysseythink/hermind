package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestWS_ForwardsHubEvents(t *testing.T) {
	s := newTestServer(t)

	httpSrv := httptest.NewServer(s.Router())
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/sessions/sess-ws/stream/ws?t=t"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Give the server's subscribe a moment to register before we
	// publish, otherwise a fast test race drops the event.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		// Heuristic: the memory hub's map is populated under
		// Subscribe. Poll by publishing + reading with a short
		// timeout until a frame arrives.
		s.Streams().Publish(StreamEvent{
			Type:      EventTypeStatus,
			SessionID: "sess-ws",
			Data:      "started",
		})
		readCtx, cancelRead := context.WithTimeout(ctx, 100*time.Millisecond)
		_, data, err := conn.Read(readCtx)
		cancelRead()
		if err == nil {
			var ev StreamEvent
			if err := json.Unmarshal(data, &ev); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if ev.Type != EventTypeStatus || ev.SessionID != "sess-ws" {
				t.Errorf("unexpected event: %+v", ev)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("did not receive any event within the deadline")
}

func TestWS_RejectsMissingToken(t *testing.T) {
	s := newTestServer(t)
	httpSrv := httptest.NewServer(s.Router())
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/sessions/sess-x/stream/ws"

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if resp == nil || resp.StatusCode != 401 {
		t.Errorf("expected 401, got %v", resp)
	}
}
