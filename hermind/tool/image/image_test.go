package image

import (
	"context"
	"encoding/base64"
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

func TestImageGenerateAndSave(t *testing.T) {
	var hits int32
	// A 1-byte fake PNG payload, b64-encoded.
	b64 := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("missing auth")
		}
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"` + b64 + `"}]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := NewClient(srv.URL, "k", "dall-e-3", dir)
	reg := tool.NewRegistry()
	Register(reg, c)

	args, _ := json.Marshal(map[string]string{"prompt": "a red cube", "size": "512x512"})
	out, err := reg.Dispatch(context.Background(), "image_generate", args)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out, dir) {
		t.Errorf("expected path in result, got %s", out)
	}
	// Verify a file landed on disk.
	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Error("expected file in save dir")
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", hits)
	}
	_ = filepath.Dir
}

func TestImageGenerateNoAPIKey(t *testing.T) {
	reg := tool.NewRegistry()
	Register(reg, NewClient("", "", "", t.TempDir()))
	if len(reg.Definitions(nil)) != 0 {
		t.Error("expected tool to be skipped when no key")
	}
}
