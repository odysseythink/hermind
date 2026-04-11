package platforms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nousresearch/hermes-agent/gateway"
)

func TestMatrixSendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Method != "PUT" {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing auth header")
		}
		if !strings.Contains(r.URL.Path, "/_matrix/client/v3/rooms/") {
			t.Errorf("path = %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.Path, "/send/m.room.message/") {
			t.Errorf("missing send/m.room.message: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if payload["msgtype"] != "m.text" || payload["body"] != "hi" {
			t.Errorf("payload = %+v", payload)
		}
		_, _ = w.Write([]byte(`{"event_id":"$evt1"}`))
	}))
	defer srv.Close()

	m := NewMatrix(srv.URL, "tok", "!room:example.com")
	err := m.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", hits)
	}
}
