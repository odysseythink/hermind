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

func TestMem0SyncTurnAndRecall(t *testing.T) {
	var addCount, searchCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		switch {
		case strings.HasSuffix(r.URL.Path, "/memories/") && r.Method == "POST":
			atomic.AddInt32(&addCount, 1)
			if !strings.Contains(string(body), "tea") {
				t.Errorf("unexpected body: %s", body)
			}
			_, _ = w.Write([]byte(`{"id":"m_1","status":"ok"}`))
		case strings.HasSuffix(r.URL.Path, "/memories/search/") && r.Method == "POST":
			atomic.AddInt32(&searchCount, 1)
			_, _ = w.Write([]byte(`[{"memory":"user likes tea"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewMem0(config.Mem0Config{
		BaseURL: srv.URL,
		APIKey:  "m0_key",
		UserID:  "user1",
	})
	if p.Name() != "mem0" {
		t.Fatalf("name = %q", p.Name())
	}

	ctx := context.Background()
	if err := p.Initialize(ctx, "sess-1"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := p.SyncTurn(ctx, "I like tea", "got it"); err != nil {
		t.Fatalf("SyncTurn: %v", err)
	}
	if atomic.LoadInt32(&addCount) != 1 {
		t.Errorf("expected 1 add call, got %d", addCount)
	}

	reg := tool.NewRegistry()
	p.RegisterTools(reg)
	args, _ := json.Marshal(map[string]string{"query": "drink"})
	res, err := reg.Dispatch(ctx, "mem0_recall", args)
	if err != nil {
		t.Fatalf("Dispatch mem0_recall: %v", err)
	}
	if !strings.Contains(res, "likes tea") {
		t.Errorf("expected recall to include tea, got %s", res)
	}
	if atomic.LoadInt32(&searchCount) != 1 {
		t.Errorf("expected 1 search call, got %d", searchCount)
	}
}
