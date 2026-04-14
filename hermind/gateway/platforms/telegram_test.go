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
	"time"

	"github.com/odysseythink/hermind/gateway"
)

func TestTelegramPollingAndReply(t *testing.T) {
	var getUpdatesHits, sendMessageHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "getUpdates"):
			hits := atomic.AddInt32(&getUpdatesHits, 1)
			if hits == 1 {
				_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":1,"message":{"message_id":10,"from":{"id":42,"username":"alice"},"chat":{"id":99},"text":"hello bot"}}]}`))
			} else {
				_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
			}
		case strings.Contains(r.URL.Path, "sendMessage"):
			atomic.AddInt32(&sendMessageHits, 1)
			body, _ := io.ReadAll(r.Body)
			var m map[string]any
			_ = json.Unmarshal(body, &m)
			if m["text"] != "echo: hello bot" {
				t.Errorf("unexpected reply text: %v", m["text"])
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tg := NewTelegram("bot-token").WithBaseURL(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = tg.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			return &gateway.OutgoingMessage{
				UserID: in.UserID,
				ChatID: in.ChatID,
				Text:   "echo: " + in.Text,
			}, nil
		})
		close(done)
	}()

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&sendMessageHits) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done

	if atomic.LoadInt32(&getUpdatesHits) == 0 {
		t.Error("getUpdates never called")
	}
	if atomic.LoadInt32(&sendMessageHits) != 1 {
		t.Errorf("sendMessage hits = %d", sendMessageHits)
	}
}
