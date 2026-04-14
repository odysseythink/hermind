package memprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool"
)

func TestRetainDBSaveAndSearch(t *testing.T) {
	var saveHits, searchHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/memory") && r.Method == "POST":
			atomic.AddInt32(&saveHits, 1)
			_, _ = w.Write([]byte(`{"id":"r_1"}`))
		case strings.HasSuffix(r.URL.Path, "/v1/memory/search") && r.Method == "POST":
			atomic.AddInt32(&searchHits, 1)
			_, _ = w.Write([]byte(`{"results":[{"content":"user plays piano"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewRetainDB(config.RetainDBConfig{
		BaseURL: srv.URL,
		APIKey:  "k",
		Project: "hermes",
		UserID:  "alice",
	})
	if p.Name() != "retaindb" {
		t.Fatalf("name = %q", p.Name())
	}
	ctx := context.Background()
	_ = p.Initialize(ctx, "sess-1")
	if err := p.SyncTurn(ctx, "I play piano", "ok"); err != nil {
		t.Fatalf("SyncTurn: %v", err)
	}
	if atomic.LoadInt32(&saveHits) != 1 {
		t.Errorf("saveHits = %d", saveHits)
	}

	reg := tool.NewRegistry()
	p.RegisterTools(reg)
	args, _ := json.Marshal(map[string]string{"query": "hobby"})
	res, err := reg.Dispatch(ctx, "retaindb_search", args)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(res, "piano") {
		t.Errorf("missing piano in result: %s", res)
	}
	if atomic.LoadInt32(&searchHits) != 1 {
		t.Errorf("searchHits = %d", searchHits)
	}
}
