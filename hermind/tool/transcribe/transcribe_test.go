package transcribe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/odysseythink/hermind/tool"
)

func TestTranscribeRoundTrip(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("bad content-type: %s", r.Header.Get("Content-Type"))
		}
		_, _ = w.Write([]byte(`{"text":"hello world"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.wav")
	if err := os.WriteFile(path, []byte("fake audio"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewClient(srv.URL, "k", "whisper-1")
	reg := tool.NewRegistry()
	Register(reg, c)

	args, _ := json.Marshal(map[string]string{"path": path})
	out, err := reg.Dispatch(context.Background(), "transcribe_audio", args)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out, "hello world") {
		t.Errorf("unexpected output: %s", out)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", hits)
	}
}
