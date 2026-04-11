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

func TestWebhookSendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing auth")
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "hi there") {
			t.Errorf("unexpected body: %s", body)
		}
		var p webhookPayload
		_ = json.Unmarshal(body, &p)
		if p.UserID != "u1" {
			t.Errorf("user id = %q", p.UserID)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	wb := NewWebhook(srv.URL, "tok")
	err := wb.SendReply(context.Background(), gateway.OutgoingMessage{
		UserID: "u1", Text: "hi there",
	})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", hits)
	}
}

func TestWebhookSendReplyNoURL(t *testing.T) {
	wb := NewWebhook("", "")
	err := wb.SendReply(context.Background(), gateway.OutgoingMessage{})
	if err == nil {
		t.Fatal("expected error")
	}
}
