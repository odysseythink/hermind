package memprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/tool"
)

func TestOpenVikingAppendAndFind(t *testing.T) {
	var appendHits, findHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/api/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/messages") && r.Method == "POST":
			atomic.AddInt32(&appendHits, 1)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case strings.HasSuffix(r.URL.Path, "/api/v1/search/find") && r.Method == "POST":
			atomic.AddInt32(&findHits, 1)
			_, _ = w.Write([]byte(`{"results":[{"content":"hermes rocks"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewOpenViking(config.OpenVikingConfig{Endpoint: srv.URL, APIKey: "k"})
	ctx := context.Background()
	_ = p.Initialize(ctx, "sess-xyz")
	if err := p.SyncTurn(ctx, "hi", "hello"); err != nil {
		t.Fatalf("SyncTurn: %v", err)
	}
	if atomic.LoadInt32(&appendHits) != 1 {
		t.Errorf("appendHits = %d", appendHits)
	}

	reg := tool.NewRegistry()
	p.RegisterTools(reg)
	args, _ := json.Marshal(map[string]string{"query": "rocks"})
	res, err := reg.Dispatch(ctx, "openviking_find", args)
	if err != nil {
		t.Fatalf("dispatch find: %v", err)
	}
	if !strings.Contains(res, "hermes rocks") {
		t.Errorf("missing result: %s", res)
	}
	if atomic.LoadInt32(&findHits) != 1 {
		t.Errorf("findHits = %d", findHits)
	}
}
