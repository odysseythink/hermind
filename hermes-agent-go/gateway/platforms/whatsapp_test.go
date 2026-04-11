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

func TestWhatsAppSendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		if !strings.Contains(r.URL.Path, "/v19.0/PID123/messages") {
			t.Errorf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if payload["messaging_product"] != "whatsapp" || payload["to"] != "+19876543210" {
			t.Errorf("payload = %+v", payload)
		}
		txt, _ := payload["text"].(map[string]any)
		if txt == nil || txt["body"] != "hi" {
			t.Errorf("text = %v", payload["text"])
		}
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.1"}]}`))
	}))
	defer srv.Close()

	wa := NewWhatsApp("PID123", "tok").WithBaseURL(srv.URL)
	err := wa.SendReply(context.Background(), gateway.OutgoingMessage{ChatID: "+19876543210", Text: "hi"})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", hits)
	}
}
