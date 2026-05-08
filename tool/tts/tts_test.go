package tts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/odysseythink/hermind/tool"
)

func TestSpeakWritesFile(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("missing auth")
		}
		_, _ = w.Write([]byte("fake-mp3-bytes"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := NewClient(srv.URL, "k", "tts-1", "alloy", dir)
	reg := tool.NewRegistry()
	Register(reg, c)

	args, _ := json.Marshal(map[string]string{"text": "hello world"})
	out, err := reg.Dispatch(context.Background(), "speak", args)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out, dir) {
		t.Errorf("expected path in output, got %s", out)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", hits)
	}
}
