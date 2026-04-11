package platforms

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nousresearch/hermes-agent/gateway"
)

func TestSMSSendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if !strings.Contains(r.URL.Path, "/2010-04-01/Accounts/AC123/Messages.json") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Basic auth: user=AC123, pass=tok
		authHdr := r.Header.Get("Authorization")
		wantHdr := "Basic " + base64.StdEncoding.EncodeToString([]byte("AC123:tok"))
		if authHdr != wantHdr {
			t.Errorf("auth header = %q", authHdr)
		}
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		if !strings.Contains(bodyStr, "From=") || !strings.Contains(bodyStr, "To=") ||
			!strings.Contains(bodyStr, "Body=hello") {
			t.Errorf("missing form fields: %s", bodyStr)
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"sid":"SM1"}`))
	}))
	defer srv.Close()

	s := NewSMS("AC123", "tok", "+15550001111", "+15550002222").WithBaseURL(srv.URL)
	if err := s.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hello"}); err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", hits)
	}
}

func TestSMSMissingConfig(t *testing.T) {
	s := NewSMS("", "", "", "")
	err := s.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}
