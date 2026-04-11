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

func TestHomeAssistantSendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bearer llt" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		if !strings.Contains(r.URL.Path, "/api/services/notify/mobile_app_phone") {
			t.Errorf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if payload["message"] != "hi" {
			t.Errorf("message = %v", payload["message"])
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	ha := NewHomeAssistant(srv.URL, "llt", "mobile_app_phone")
	err := ha.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", hits)
	}
}
