package platforms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/odysseythink/hermind/gateway"
)

func TestSignalSendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		var req signalRPCRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		if req.Method != "send" {
			t.Errorf("method = %q, want send", req.Method)
		}
		if req.Params["account"] != "+1234567890" {
			t.Errorf("account = %v", req.Params["account"])
		}
		if req.Params["message"] != "hi" {
			t.Errorf("message = %v", req.Params["message"])
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"timestamp":0}}`))
	}))
	defer srv.Close()

	s := NewSignal(srv.URL, "+1234567890")
	err := s.SendReply(context.Background(), gateway.OutgoingMessage{ChatID: "+19876543210", Text: "hi"})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", hits)
	}
}

func TestSignalRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`))
	}))
	defer srv.Close()
	s := NewSignal(srv.URL, "+1234567890")
	err := s.SendReply(context.Background(), gateway.OutgoingMessage{ChatID: "+1", Text: "hi"})
	if err == nil {
		t.Error("expected rpc error")
	}
}
