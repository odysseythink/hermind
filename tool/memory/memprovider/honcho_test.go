package memprovider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool"
)

func TestHonchoSyncTurnAndRecall(t *testing.T) {
	var addCount, searchCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/messages") && r.Method == "POST":
			atomic.AddInt32(&addCount, 1)
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "dark mode") {
				t.Errorf("expected body to contain dark mode, got %s", body)
			}
			_, _ = w.Write([]byte(`{"id":"msg_1"}`))
		case strings.HasSuffix(r.URL.Path, "/search") && r.Method == "POST":
			atomic.AddInt32(&searchCount, 1)
			_, _ = w.Write([]byte(`{"results":[{"content":"user likes dark mode"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewHoncho(config.HonchoConfig{
		BaseURL:   srv.URL,
		APIKey:    "test-key",
		Workspace: "hermind",
		Peer:      "me",
	})
	if p.Name() != "honcho" {
		t.Fatalf("name = %q", p.Name())
	}

	ctx := context.Background()
	if err := p.Initialize(ctx, "sess-1"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := p.SyncTurn(ctx, "I like dark mode", "noted"); err != nil {
		t.Fatalf("SyncTurn: %v", err)
	}
	if atomic.LoadInt32(&addCount) != 1 {
		t.Errorf("expected 1 add call, got %d", addCount)
	}

	reg := tool.NewRegistry()
	p.RegisterTools(reg)

	args, _ := json.Marshal(map[string]string{"query": "preferences"})
	res, err := reg.Dispatch(ctx, "honcho_recall", args)
	if err != nil {
		t.Fatalf("Dispatch honcho_recall: %v", err)
	}
	if !strings.Contains(res, "dark mode") {
		t.Errorf("expected recall result to contain dark mode, got %s", res)
	}
	if atomic.LoadInt32(&searchCount) != 1 {
		t.Errorf("expected 1 search call, got %d", searchCount)
	}

	if err := p.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}
